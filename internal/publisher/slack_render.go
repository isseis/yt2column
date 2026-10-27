package publisher

import (
	"strings"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/writer"
)

// footnoteSeparator joins the parts of the footnote.
const footnoteSeparator = " \u00b7 "

// zeroWidthSpace is inserted after an '@' to keep it from starting a mention.
const zeroWidthSpace = "\xe2\x80\x8b"

// slackPost is the full text of a post.
type slackPost struct {
	text string
	// keepTogetherFrom is the rune index in text from which the post must not
	// be cut: the last line of the body and the footer stay in one message.
	// It lies in [0, rune count of text]; the rune count means no such region.
	keepTogetherFrom int
}

// stageModel is one model name and the stages that used it.
type stageModel struct {
	Model  string
	Stages []writer.Stage // StageGeneration, StageShorten, StageReview, in this order
}

// stageModels returns the models of the stages that ran, grouped by model name
// in order of first appearance: generation, then shorten when Article.Shorten
// is set, then review when Article.Review is set. A model's version is not
// part of the grouping.
func stageModels(a writer.Article) []stageModel {
	var models []stageModel
	add := func(stage writer.Stage, model string) {
		for i := range models {
			if models[i].Model == model {
				models[i].Stages = append(models[i].Stages, stage)
				return
			}
		}
		models = append(models, stageModel{Model: model, Stages: []writer.Stage{stage}})
	}
	add(writer.StageGeneration, a.Model)
	if a.Shorten != nil {
		add(writer.StageShorten, a.Shorten.Model)
	}
	if a.Review != nil {
		add(writer.StageReview, a.Review.Model.Model)
	}
	return models
}

// renderSlackPost returns the post for a: a heading line with the title and
// the channel name, a blank line, the body, a blank line, and a one-line
// footnote of the models and the estimated cost.
//
// Contract: every '@' that mentionAts reports in the title, the channel name,
// or the footnote's model names is neutralized; in the body only those outside
// code are, and none when writer.ProseRanges cannot judge the body (the caller
// then refuses the post if a target remains). a is not changed. The body's
// trailing newlines are dropped so that exactly one blank line precedes the
// footnote. a's strings must be valid UTF-8.
func renderSlackPost(a writer.Article) slackPost {
	var b strings.Builder
	b.WriteString("# " + neutralizeMentions(a.Title))
	if a.ChannelName != "" {
		b.WriteString("\uff08" + neutralizeMentions(a.ChannelName) + "\uff09")
	}
	b.WriteString("\n\n")

	body := a.Body
	if prose, ok := writer.ProseRanges(body); ok {
		body = neutralizeProse(body, prose)
	}
	body = strings.TrimRight(body, "\n")
	b.WriteString(body)
	// The last line of the body is the text after its last newline.
	lastLine := strings.LastIndexByte(body, '\n') + 1
	keep := utf8.RuneCountInString(b.String()) - utf8.RuneCountInString(body[lastLine:])

	b.WriteString("\n\n" + slackFootnote(a))
	return slackPost{text: b.String(), keepTogetherFrom: keep}
}

// slackFootnote returns the footnote line: the models with their stages, then
// the estimated cost. It shows no model version.
func slackFootnote(a writer.Article) string {
	parts := []string{"Models: "}
	for i, m := range stageModels(a) {
		names := make([]string, len(m.Stages))
		for j, stage := range m.Stages {
			names[j] = stage.String()
		}
		if i > 0 {
			parts = append(parts, footnoteSeparator)
		}
		parts = append(parts, neutralizeMentions(m.Model)+" ("+strings.Join(names, ", ")+")")
	}
	cost := "unknown"
	if usd, known := FormatArticleCost(a.RunUsage); known {
		cost = usd + " USD"
	}
	parts = append(parts, footnoteSeparator+"Estimated cost: "+cost)
	return strings.Join(parts, "")
}

// neutralizeMentions returns value with U+200B inserted after each '@' that
// mentionAts reports for the V3-replaced value, except an '@' that directly
// follows '/': it may be part of a URL, which U+200B would break, and
// rejectMention refuses it instead. A look-alike such as U+FF20 is treated as
// '@' and keeps its own rune, followed by U+200B. A value with no such '@' is
// returned unchanged. value must be valid UTF-8.
func neutralizeMentions(value string) string {
	return insertAfterAts(value, func(int) bool { return true })
}

// neutralizeProse does the same for a body, but only for the '@' characters
// whose byte offset in body lies inside one of prose, the ranges outside
// code. mentionAts reports rune indices of the V3-replaced body; each is
// converted to the byte offset of the same rune in body before it is compared
// with prose, because a look-alike such as U+FF20 takes three bytes in body
// and one after replacement. body must be valid UTF-8. With ProseRanges
// reporting false there is no range to pass: the caller leaves the body as it
// is, and rejectMention refuses its targets.
func neutralizeProse(body string, prose []writer.ByteRange) string {
	return insertAfterAts(body, func(offset int) bool { return inRanges(prose, offset) })
}

// insertAfterAts inserts U+200B after each target '@' of value that does not
// follow '/' and whose byte offset in value satisfies inScope.
func insertAfterAts(value string, inScope func(offset int) bool) string {
	runes := []rune(value)
	ats := mentionAtsOf(runes)
	if len(ats) == 0 {
		return value
	}
	var b strings.Builder
	b.Grow(len(value) + len(ats)*len(zeroWidthSpace))
	next, offset := 0, 0
	for i, r := range runes {
		b.WriteRune(r)
		if next < len(ats) && ats[next] == i {
			next++
			if !followsSlash(runes, i) && inScope(offset) {
				b.WriteString(zeroWidthSpace)
			}
		}
		offset += utf8.RuneLen(r)
	}
	return b.String()
}

// followsSlash reports whether the rune before index i is '/'.
func followsSlash(runes []rune, i int) bool {
	return i > 0 && runes[i-1] == '/'
}

// inRanges reports whether offset lies in one of ranges.
func inRanges(ranges []writer.ByteRange, offset int) bool {
	for _, r := range ranges {
		if r.Start <= offset && offset < r.End {
			return true
		}
	}
	return false
}
