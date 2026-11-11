package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/transcript"
)

// errMaterial reports scoring material that cannot be built without revealing
// the config, or whose inputs are missing.
var errMaterial = errors.New("cannot build the scoring material")

// reviewLogModelPrefix starts the review log's model lines (internal/publisher
// writes "- Review model: ..." and "- Review model version: ..."), which name
// the review model and are left out of the material.
const reviewLogModelPrefix = "- Review model"

// The names of the material files of one article.
const (
	materialArticle = "article.md"
	materialShorten = "shorten.md"
	materialReview  = "review-log.md"
	materialTx      = "transcript.txt"
	mappingName     = "mapping.tsv"
)

// materialRequest is what material builds.
type materialRequest struct {
	Root     string // the 0087 directory
	OutDir   string // score/<stage>
	Configs  []config
	Videos   []string
	Shuffler func(n int) ([]int, error)
}

// cryptoShuffle returns a random permutation of 0..n-1.
func cryptoShuffle(n int) ([]int, error) {
	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return nil, err
		}
		perm[i], perm[j.Int64()] = perm[j.Int64()], perm[i]
	}
	return perm, nil
}

// buildMaterial writes, for each video, the transcript and every article of
// the stage's configs under names that do not reveal the config, in random
// order, to OutDir/material/<video>/; and the mapping from names to configs to
// OutDir/mapping.tsv, outside the material directory (design section 3.6 step 2).
func buildMaterial(q materialRequest) error {
	materialDir := filepath.Join(q.OutDir, "material")
	if _, err := os.Stat(materialDir); err == nil {
		return fmt.Errorf("%w: %s already exists", errMaterial, materialDir)
	}
	forbidden := forbiddenWords(q.Configs)
	var mapping strings.Builder
	mapping.WriteString("name\tconfig\tarticle\n")
	for _, v := range q.Videos {
		dir := filepath.Join(materialDir, v)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := writeTranscript(q, v, dir, forbidden); err != nil {
			return err
		}
		var items []articleKey
		for _, c := range q.Configs {
			for _, n := range []string{"1", "2"} {
				if _, err := os.Stat(filepath.Join(q.Root, "out", c.ID, v+"-"+n+".md")); err == nil {
					items = append(items, articleKey{c.ID, v + "-" + n})
				}
			}
		}
		perm, err := q.Shuffler(len(items))
		if err != nil {
			return err
		}
		for i, p := range perm {
			name := fmt.Sprintf("%s-%c", v, 'a'+i)
			if err := writeArticleMaterial(q, items[p], filepath.Join(dir, name), forbidden); err != nil {
				return err
			}
			fmt.Fprintf(&mapping, "%s\t%s\t%s\n", name, items[p].Config, items[p].Article)
		}
	}
	return os.WriteFile(filepath.Join(q.OutDir, mappingName), []byte(mapping.String()), 0o600)
}

// forbiddenWords are the strings no material file may contain: every config
// ID and every model ID of the stage.
func forbiddenWords(configs []config) []string {
	var out []string
	for _, c := range configs {
		out = append(out, c.ID, c.LLM.Model, c.ReviewLLM.Model)
	}
	return out
}

// writeChecked writes text to path after checking it reveals no config.
func writeChecked(path, text string, forbidden []string) error {
	for _, w := range forbidden {
		if strings.Contains(text, w) {
			return fmt.Errorf("%w: %s would contain a config or model name", errMaterial, path)
		}
	}
	return os.WriteFile(path, []byte(text), 0o600)
}

// writeArticleMaterial writes one article's material: its title and body,
// the shorten output's title and body when the shorten artifact exists, and
// the review log without its model lines.
func writeArticleMaterial(q materialRequest, a articleKey, prefix string, forbidden []string) error {
	work := filepath.Join(q.Root, "work", a.Config, a.Article)
	review, ok, err := readArtifact(work, a.Article, stageReview)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s %s has no review artifact", errMaterial, a.Config, a.Article)
	}
	// The article file is the published article; its title and body are the
	// review artifact's, which carry no front matter. Check they are the same.
	out, err := os.ReadFile(filepath.Join(q.Root, "out", a.Config, a.Article+".md")) //nolint:gosec // under the operator's root
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "# "+review.Title+"\n") || !strings.Contains(string(out), strings.TrimSpace(review.Body)) {
		return fmt.Errorf("%w: %s %s: the article file and the review artifact differ", errMaterial, a.Config, a.Article)
	}
	if err := writeChecked(prefix+"."+materialArticle, "# "+review.Title+"\n"+review.Body, forbidden); err != nil {
		return err
	}
	shorten, ok, err := readArtifact(work, a.Article, stageShorten)
	if err != nil {
		return err
	}
	if ok {
		if err := writeChecked(prefix+"."+materialShorten, "# "+shorten.Title+"\n"+shorten.Body, forbidden); err != nil {
			return err
		}
	}
	logText, err := os.ReadFile(filepath.Join(q.Root, "review", a.Config, a.Article+".md")) //nolint:gosec // under the operator's root
	if err != nil {
		return err
	}
	var kept strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(string(logText)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20) //nolint:mnd // a log line is short; 1 MiB bounds a broken one
	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), reviewLogModelPrefix) {
			kept.WriteString(scanner.Text() + "\n")
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return writeChecked(prefix+"."+materialReview, kept.String(), forbidden)
}

// writeTranscript writes the video's title, channel, description, and
// time-stamped transcript, read with the CLI's own cache reader from the
// cache copy of the first config's first article (a yt-dlp path that always
// fails keeps a cache miss from fetching).
func writeTranscript(q materialRequest, video, dir string, forbidden []string) error {
	cache := filepath.Join(q.Root, "runcache", q.Configs[0].ID, video+"-1")
	source, err := transcript.NewYtDlpSource(transcript.Options{CacheDir: cache, YtDlpPath: "/usr/bin/false", Timeout: time.Minute})
	if err != nil {
		return err
	}
	url, _ := transcript.NormalizedVideoURL(video)
	tr, err := source.Fetch(context.Background(), url)
	if err != nil {
		return fmt.Errorf("%w: transcript of %s: %w", errMaterial, video, err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s\nChannel: %s\n\nDescription:\n%s\n\nTranscript:\n", tr.Title, tr.ChannelName, tr.Description)
	for _, s := range tr.Segments {
		sec := s.StartMs / 1000                                   //nolint:mnd // milliseconds per second
		fmt.Fprintf(&b, "%02d:%02d %s\n", sec/60, sec%60, s.Text) //nolint:mnd // seconds per minute
	}
	return writeChecked(filepath.Join(dir, materialTx), b.String(), forbidden)
}

// runMaterial implements "material".
func runMaterial(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("material", flag.ContinueOnError)
	root := fs.String("root", "", "the 0087 directory")
	out := fs.String("out", "", "score/<stage>")
	runsh := fs.String("runsh", "", "the run script, which prints the config table")
	stage := fs.Int("stage", 0, "the evaluation stage, 1 or 2")
	if err := fs.Parse(args); err != nil {
		return err
	}
	table, err := loadConfigTable(*runsh)
	if err != nil {
		return err
	}
	var cfgs []config
	for _, id := range table.ids(*stage) {
		c, _ := table.byID(id)
		if _, err := os.Stat(filepath.Join(*root, "out", id)); err == nil {
			cfgs = append(cfgs, c)
		}
	}
	if len(cfgs) == 0 {
		return fmt.Errorf("%w: stage %d has no articles", errMaterial, *stage)
	}
	if err := buildMaterial(materialRequest{Root: *root, OutDir: *out, Configs: cfgs, Videos: videos, Shuffler: cryptoShuffle}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "material: wrote %s and %s\n", filepath.Join(*out, "material"), filepath.Join(*out, mappingName))
	return err
}
