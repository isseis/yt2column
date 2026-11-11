package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The criteria of a judgment, as keys of the judgments file. The required
// criteria are pass or fail; the quality criteria are 1, 2, or 3; the product
// constraints 1 to 3 are yes (the article has that kind of error) or no.
const (
	critFormatChoice = "format_choice"
	critFormatRules  = "format_rules"
	critEvidence     = "evidence"
	critLength       = "length"
	critNotation     = "notation"
	critCoverage     = "coverage"
	critImportance   = "importance"
	critStructure    = "structure"
	critReadability  = "readability"
	critTitleLead    = "title_lead"
	critRedundancy   = "redundancy"
	critConstraint1  = "constraint1"
	critConstraint2  = "constraint2"
	critConstraint3  = "constraint3"
)

// The criteria in the order the record lists them.
var (
	requiredCriteria   = []string{critFormatChoice, critFormatRules, critEvidence, critLength, critNotation}
	qualityCriteria    = []string{critCoverage, critImportance, critStructure, critReadability, critTitleLead, critRedundancy}
	constraintCriteria = []string{critConstraint1, critConstraint2, critConstraint3}
	allCriteria        = slices.Concat(requiredCriteria, qualityCriteria, constraintCriteria)
)

// The judges of a judgment: the check script, the scoring LLM, or a human who
// changed the LLM's judgment.
const (
	judgeScript = "script"
	judgeLLM    = "llm"
	judgeHuman  = "human"
)

// The values of a judgment cell.
const (
	valPass = "pass"
	valFail = "fail"
	valYes  = "yes"
	valNo   = "no"
)

// judgmentsHeader is the first line of a judgments file.
const judgmentsHeader = "article\tconfig\tcriterion\tinterp\tliteral\tjudge\thuman_check"

// errJudgments reports a judgments file that breaks one of its rules.
var errJudgments = errors.New("judgments file is rejected")

// judgment is one row: the judgment of one criterion of one article, by the
// interpretive reading and by the literal one (0018 evaluation record section 3.8).
// A constraint row carries its answer in Interp and "-" in Literal.
type judgment struct {
	Article, Config, Criterion string
	Interp, Literal            string
	Judge                      string
	HumanCheck                 bool
}

// judgmentKey identifies a judgment.
type judgmentKey struct{ Config, Article, Criterion string }

// articleKey identifies an article of a config.
type articleKey struct{ Config, Article string }

// judgments holds a validated judgments file.
type judgments map[judgmentKey]judgment

// parseJudgments reads a judgments file and checks each row on its own.
func parseJudgments(r io.Reader) (judgments, error) {
	out := judgments{}
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() || scanner.Text() != judgmentsHeader {
		return nil, fmt.Errorf("%w: the first line is not the header", errJudgments)
	}
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 7 { //nolint:mnd // the seven columns of the header
			return nil, fmt.Errorf("%w: line %q", errJudgments, scanner.Text())
		}
		j := judgment{
			Article: fields[0], Config: fields[1], Criterion: fields[2],
			Interp: fields[3], Literal: fields[4], Judge: fields[5],
		}
		switch fields[6] {
		case valYes:
			j.HumanCheck = true
		case valNo:
		default:
			return nil, fmt.Errorf("%w: %s %s %s: human_check %q", errJudgments, j.Config, j.Article, j.Criterion, fields[6])
		}
		if err := j.validate(); err != nil {
			return nil, err
		}
		k := judgmentKey{j.Config, j.Article, j.Criterion}
		if _, dup := out[k]; dup {
			return nil, fmt.Errorf("%w: %s %s %s appears twice", errJudgments, j.Config, j.Article, j.Criterion)
		}
		out[k] = j
	}
	return out, scanner.Err()
}

// validate checks one row: the values its criterion allows, its judge, and
// the human checks design section 3.6 requires: every required-criterion fail, every
// quality 1 and 2, every evidence pass, and every product-constraint answer.
func (j judgment) validate() error {
	fail := func(what string) error {
		return fmt.Errorf("%w: %s %s %s: %s", errJudgments, j.Config, j.Article, j.Criterion, what)
	}
	if !articleRE.MatchString(j.Article) || j.Config == "" {
		return fail("article or config")
	}
	needsCheck, problem := j.checkValues()
	if problem != "" {
		return fail(problem)
	}
	switch j.Judge {
	case judgeScript:
		if j.Criterion != critLength {
			return fail("only length is judged by the script")
		}
		needsCheck = false
	case judgeLLM:
		if j.Criterion == critLength {
			return fail("length is judged by the script")
		}
	case judgeHuman:
		needsCheck = true
	default:
		return fail("unknown judge")
	}
	if needsCheck && !j.HumanCheck {
		return fail("needs a human check")
	}
	return nil
}

// checkValues checks the values the row's criterion allows. It returns
// whether the values need a human check, or the problem with them.
func (j judgment) checkValues() (needsCheck bool, problem string) {
	switch {
	case slices.Contains(requiredCriteria, j.Criterion):
		for _, v := range []string{j.Interp, j.Literal} {
			if v != valPass && v != valFail {
				return false, "value must be pass or fail"
			}
			needsCheck = needsCheck || v == valFail || (j.Criterion == critEvidence && v == valPass)
		}
		return needsCheck, ""
	case slices.Contains(qualityCriteria, j.Criterion):
		for _, v := range []string{j.Interp, j.Literal} {
			if v != "1" && v != "2" && v != "3" {
				return false, "value must be 1, 2, or 3"
			}
			needsCheck = needsCheck || v != "3"
		}
		return needsCheck, ""
	case slices.Contains(constraintCriteria, j.Criterion):
		if (j.Interp != valYes && j.Interp != valNo) || j.Literal != none {
			return false, "value must be yes or no, with - as the literal value"
		}
		return true, ""
	default:
		return false, "unknown criterion"
	}
}

// checkComplete rejects a judgments file that lacks a criterion of an article
// it must judge, or judges an article it must not (one not obtained).
func (js judgments) checkComplete(expected []articleKey) error {
	want := map[articleKey]bool{}
	for _, a := range expected {
		want[a] = true
		for _, c := range allCriteria {
			if _, ok := js[judgmentKey{a.Config, a.Article, c}]; !ok {
				return fmt.Errorf("%w: %s %s lacks %s", errJudgments, a.Config, a.Article, c)
			}
		}
	}
	for k := range js {
		if !want[articleKey{k.Config, k.Article}] {
			return fmt.Errorf("%w: %s %s is not an obtained article of the stage", errJudgments, k.Config, k.Article)
		}
	}
	return nil
}

// goalMet reports whether an article meets the quality goal by one reading
// (interp true for the interpretive one): every required criterion passes and
// no quality criterion is 1.
func (js judgments) goalMet(cfg, article string, interp bool) bool {
	pick := func(j judgment) string {
		if interp {
			return j.Interp
		}
		return j.Literal
	}
	for _, c := range requiredCriteria {
		if pick(js[judgmentKey{cfg, article, c}]) != valPass {
			return false
		}
	}
	for _, c := range qualityCriteria {
		if pick(js[judgmentKey{cfg, article, c}]) == "1" {
			return false
		}
	}
	return true
}

// requiredFailInterp reports whether an article fails a required criterion
// by the interpretive reading, which condition (ii) compares per video.
func (js judgments) requiredFailInterp(cfg, article string) bool {
	for _, c := range requiredCriteria {
		if js[judgmentKey{cfg, article, c}].Interp == valFail {
			return true
		}
	}
	return false
}

// articleCheck is one row of check_article.py's table: the title's character
// count, the number of level-2 sections, and the body length.
type articleCheck struct {
	TitleChars, Sections, Length int
}

// errCheckArticle reports a check_article.txt that does not match its form.
var errCheckArticle = errors.New("check_article output does not match its form")

// parseCheckArticle reads check_article.txt: for each config, a line
// "config <ID>" followed by the tables check_article.py printed for that
// config's articles.
func parseCheckArticle(r io.Reader) (map[articleKey]articleCheck, error) {
	out := map[articleKey]articleCheck{}
	cfg := ""
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "config "):
			cfg = strings.TrimPrefix(line, "config ")
		case strings.HasPrefix(line, "| file |"), strings.HasPrefix(line, "|---"):
		case strings.HasPrefix(line, "| "):
			if cfg == "" {
				return nil, fmt.Errorf("%w: a row before the first config line", errCheckArticle)
			}
			cells := splitMarkdownRow(line)
			if len(cells) != 7 { //nolint:mnd // check_article.py's seven columns
				return nil, fmt.Errorf("%w: row %q", errCheckArticle, line)
			}
			article, ok := strings.CutSuffix(cells[0], ".md")
			if !ok || !articleRE.MatchString(article) {
				return nil, fmt.Errorf("%w: file %q", errCheckArticle, cells[0])
			}
			sections, err1 := strconv.Atoi(cells[4])
			length, err2 := strconv.Atoi(cells[5])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("%w: row %q", errCheckArticle, line)
			}
			k := articleKey{cfg, article}
			if _, dup := out[k]; dup {
				return nil, fmt.Errorf("%w: %s %s appears twice", errCheckArticle, cfg, article)
			}
			out[k] = articleCheck{TitleChars: utf8.RuneCountInString(cells[3]), Sections: sections, Length: length}
		case strings.TrimSpace(line) == "":
		default:
			return nil, fmt.Errorf("%w: line %q", errCheckArticle, line)
		}
	}
	return out, scanner.Err()
}

// splitMarkdownRow splits "| a | b\|c |" into its trimmed cells, undoing the
// "\|" escape.
func splitMarkdownRow(line string) []string {
	inner := strings.TrimSpace(line)
	inner = strings.TrimPrefix(inner, "|")
	inner = strings.TrimSuffix(inner, "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(inner); i++ {
		switch {
		case inner[i] == '\\' && i+1 < len(inner) && inner[i+1] == '|':
			cur.WriteByte('|')
			i++
		case inner[i] == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(inner[i])
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}
