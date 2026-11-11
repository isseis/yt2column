package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/isseis/yt2column/internal/artifact"
	"github.com/isseis/yt2column/internal/writer"
)

// stageFiles maps a stage to the suffix of its artifact file,
// "<video ID>.<stage>.json" (docs/dev/guides/cli.md).
var stageFiles = map[string]string{
	stageGeneration: ".generation.json",
	stageShorten:    ".shorten.json",
	stageReview:     ".review.json",
}

// readArtifact reads one stage's artifact of article from dir with the CLI's
// own parser. It returns false when the file does not exist.
func readArtifact(dir, article, stage string) (writer.StageRecord, bool, error) {
	path := filepath.Join(dir, videoOf(article)+stageFiles[stage])
	data, err := os.ReadFile(path) //nolint:gosec // paths come from the evaluation directories the flags name
	if errors.Is(err, os.ErrNotExist) {
		return writer.StageRecord{}, false, nil
	}
	if err != nil {
		return writer.StageRecord{}, false, err
	}
	rec, err := artifact.Parse(data)
	if err != nil {
		return writer.StageRecord{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return rec, true, nil
}

// artifactIDs returns the identifier of each stage's artifact in dir, or "-"
// for a stage without one.
func artifactIDs(dir, article string) (map[string]string, error) {
	out := map[string]string{}
	for _, s := range stages {
		rec, ok, err := readArtifact(dir, article, s)
		if err != nil {
			return nil, err
		}
		out[s] = none
		if ok {
			out[s] = rec.ID
		}
	}
	return out, nil
}
