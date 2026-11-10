//go:build test

package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// regenerated returns a generation record distinct from validGeneration (same
// material, different content, so a different identifier).
func regenerated() writer.StageRecord {
	rec := validGeneration()
	rec.Title = "Second title"
	rec.ID = recordID(rec)
	return rec
}

func TestLoadRejectsMismatch(t *testing.T) {
	store, _ := newTestStore(t)
	materialID := validGeneration().Input

	t.Run("material changed", func(t *testing.T) {
		mustSave(t, store, validGeneration())
		_, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, writer.Digest("other material"))
		if !errors.Is(err, ErrMismatch) {
			t.Fatalf("Load error = %v, want ErrMismatch", err)
		}
		for _, want := range []string{writer.Digest("other material"), validGeneration().Input, "--keep-cache"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Load error = %v, want it to contain %q", err, want)
			}
		}
	})

	t.Run("shorten input is not the generation id", func(t *testing.T) {
		mustSave(t, store, validGeneration())
		mustSave(t, store, shortenOf(writer.Digest("wrong")))
		if _, err := store.Load(context.Background(), testVideoID, writer.StageShorten, materialID); !errors.Is(err, ErrMismatch) {
			t.Fatalf("Load error = %v, want ErrMismatch", err)
		}
	})
}

func TestLoadRejectsStaleChain(t *testing.T) {
	t.Run("shorten left from another generation", func(t *testing.T) {
		store, _ := newTestStore(t)
		materialID := validGeneration().Input
		genID := mustSave(t, store, validGeneration())
		mustSave(t, store, shortenOf(genID))
		mustSave(t, store, regenerated())

		if _, err := store.Load(context.Background(), testVideoID, writer.StageShorten, materialID); !errors.Is(err, ErrMismatch) {
			t.Fatalf("Load error = %v, want ErrMismatch", err)
		}
	})

	t.Run("review left from another generation with a shorten", func(t *testing.T) {
		store, _ := newTestStore(t)
		materialID := validGeneration().Input
		genID := mustSave(t, store, validGeneration())
		shortenID := mustSave(t, store, shortenOf(genID))
		mustSave(t, store, reviewOf(shortenID))
		mustSave(t, store, regenerated())

		if _, err := store.Load(context.Background(), testVideoID, writer.StageReview, materialID); !errors.Is(err, ErrMismatch) {
			t.Fatalf("Load error = %v, want ErrMismatch", err)
		}
	})

	t.Run("review left from another generation without a shorten", func(t *testing.T) {
		store, _ := newTestStore(t)
		materialID := validGeneration().Input
		genID := mustSave(t, store, validGeneration())
		mustSave(t, store, reviewOf(genID))
		mustSave(t, store, regenerated())

		if _, err := store.Load(context.Background(), testVideoID, writer.StageReview, materialID); !errors.Is(err, ErrMismatch) {
			t.Fatalf("Load error = %v, want ErrMismatch", err)
		}
	})
}

func TestLoadReviewWithoutShorten(t *testing.T) {
	store, _ := newTestStore(t)
	materialID := validGeneration().Input
	genID := mustSave(t, store, validGeneration())
	mustSave(t, store, reviewOf(genID))

	chain, err := store.Load(context.Background(), testVideoID, writer.StageReview, materialID)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if chain.Review == nil {
		t.Fatal("review = nil, want the review stage")
	}
	if chain.Shorten != nil {
		t.Errorf("shorten = %+v, want nil (the shorten step did not run)", chain.Shorten)
	}
	if chain.Generation.ID != genID {
		t.Errorf("generation id = %q, want %q", chain.Generation.ID, genID)
	}
}

func TestLoadRejectsBrokenPreviousStage(t *testing.T) {
	// The shorten file exists but is broken, so the review's previous stage
	// cannot be decided: the load is malformed, not missing.
	store, dir := newTestStore(t)
	materialID := validGeneration().Input
	genID := mustSave(t, store, validGeneration())
	shortenID := mustSave(t, store, shortenOf(genID))
	mustSave(t, store, reviewOf(shortenID))

	if err := os.WriteFile(filepath.Join(dir, testVideoID+".shorten.json"), []byte("not json"), 0o600); err != nil {
		t.Fatalf("corrupt shorten: %v", err)
	}
	if _, err := store.Load(context.Background(), testVideoID, writer.StageReview, materialID); !errors.Is(err, ErrMalformed) {
		t.Fatalf("Load error = %v, want ErrMalformed", err)
	}
}

func TestLoadNamesBrokenStage(t *testing.T) {
	// A version 1 shorten artifact under a valid version 4 generation and
	// review chain: the load must name the shorten stage as the broken one.
	store, dir := newTestStore(t)
	materialID := validGeneration().Input
	genID := mustSave(t, store, validGeneration())
	shortenID := mustSave(t, store, shortenOf(genID))
	mustSave(t, store, reviewOf(shortenID))

	v1 := fieldsFrom(shortenOf(genID))
	v1["version"] = float64(1)
	delete(v1, "usage")
	if err := os.WriteFile(filepath.Join(dir, testVideoID+".shorten.json"), marshalFields(t, v1), 0o600); err != nil {
		t.Fatalf("write version 1 shorten: %v", err)
	}
	_, err := store.Load(context.Background(), testVideoID, writer.StageReview, materialID)
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("Load error = %v, want ErrMalformed", err)
	}
	if !strings.Contains(err.Error(), writer.StageShorten.String()) {
		t.Errorf("Load error = %v, want it to name the %s stage", err, writer.StageShorten)
	}
}

// TestMixedVersionResume checks a resume over version 3 files an earlier CLI
// saved: the chain loads, a new stage is saved as version 4 with its input
// naming the version 3 id, and the reused files are not rewritten.
func TestMixedVersionResume(t *testing.T) {
	materialID := validGeneration().Input
	generationPath := func(dir string) string { return filepath.Join(dir, testVideoID+".generation.json") }
	shortenPath := func(dir string) string { return filepath.Join(dir, testVideoID+".shorten.json") }
	reviewPath := func(dir string) string { return filepath.Join(dir, testVideoID+".review.json") }
	writeFile := func(t *testing.T, path, data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	requireBytes := func(t *testing.T, path, want string) {
		t.Helper()
		got, err := os.ReadFile(path) //nolint:gosec // a file under the test's temporary directory
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("%s was rewritten:\n got %s\nwant %s", filepath.Base(path), got, want)
		}
	}
	requireVersion4 := func(t *testing.T, path string) {
		t.Helper()
		data, err := os.ReadFile(path) //nolint:gosec // a file under the test's temporary directory
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if version := fieldsOf(t, data)["version"]; version != float64(artifactVersion) {
			t.Errorf("%s has version %v, want %d", filepath.Base(path), version, artifactVersion)
		}
	}

	t.Run("version 3 generation, version 4 shorten and review", func(t *testing.T) {
		store, dir := newTestStore(t)
		writeFile(t, generationPath(dir), version3Generation)

		chain, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, materialID)
		if err != nil {
			t.Fatalf("Load generation error = %v", err)
		}
		if chain.Generation.ID != version3GenerationID {
			t.Fatalf("generation id = %s, want the version 3 id %s", chain.Generation.ID, version3GenerationID)
		}

		shortenID := mustSave(t, store, shortenOf(chain.Generation.ID))
		requireVersion4(t, shortenPath(dir))
		mustSave(t, store, reviewOf(shortenID))

		chain, err = store.Load(context.Background(), testVideoID, writer.StageReview, materialID)
		if err != nil {
			t.Fatalf("Load review error = %v", err)
		}
		if chain.Shorten == nil || chain.Shorten.Input != version3GenerationID {
			t.Errorf("shorten = %+v, want its input to be the version 3 id %s", chain.Shorten, version3GenerationID)
		}
		if chain.Review == nil || chain.Review.Input != shortenID {
			t.Errorf("review = %+v, want its input to be %s", chain.Review, shortenID)
		}
		requireBytes(t, generationPath(dir), version3Generation)
	})

	t.Run("version 3 generation and shorten, version 4 review", func(t *testing.T) {
		store, dir := newTestStore(t)
		writeFile(t, generationPath(dir), version3Generation)
		writeFile(t, shortenPath(dir), version3Shorten)

		chain, err := store.Load(context.Background(), testVideoID, writer.StageShorten, materialID)
		if err != nil {
			t.Fatalf("Load shorten error = %v", err)
		}
		if chain.Shorten == nil || chain.Shorten.ID != version3ShortenID {
			t.Fatalf("shorten = %+v, want the version 3 id %s", chain.Shorten, version3ShortenID)
		}

		mustSave(t, store, reviewOf(chain.Shorten.ID))
		chain, err = store.Load(context.Background(), testVideoID, writer.StageReview, materialID)
		if err != nil {
			t.Fatalf("Load review error = %v", err)
		}
		if chain.Review == nil || chain.Review.Input != version3ShortenID {
			t.Errorf("review = %+v, want its input to be the version 3 id %s", chain.Review, version3ShortenID)
		}
		requireBytes(t, generationPath(dir), version3Generation)
		requireBytes(t, shortenPath(dir), version3Shorten)
	})

	t.Run("every stage version 3, resumed after the review", func(t *testing.T) {
		store, dir := newTestStore(t)
		writeFile(t, generationPath(dir), version3Generation)
		writeFile(t, shortenPath(dir), version3Shorten)
		writeFile(t, reviewPath(dir), version3Review)

		chain, err := store.Load(context.Background(), testVideoID, writer.StageReview, materialID)
		if err != nil {
			t.Fatalf("Load review error = %v", err)
		}
		if chain.Generation.ID != version3GenerationID || chain.Shorten == nil || chain.Shorten.ID != version3ShortenID ||
			chain.Review == nil || chain.Review.ID != version3ReviewID {
			t.Errorf("chain = %+v, want the version 3 ids", chain)
		}
		requireBytes(t, generationPath(dir), version3Generation)
		requireBytes(t, shortenPath(dir), version3Shorten)
		requireBytes(t, reviewPath(dir), version3Review)
	})
}

func TestMaterialID(t *testing.T) {
	// MaterialID is the value the whole chain is anchored to: the generation
	// artifact's input must equal the current material. Pin its exact
	// composition, including the video ID and the transcript text's join.
	base := transcript.Transcript{
		VideoID:     testVideoID,
		VideoURL:    "https://www.youtube.com/watch?v=" + testVideoID,
		Title:       "Title",
		ChannelName: "Channel",
		Description: "Description",
		Segments: []transcript.Segment{
			{StartMs: 0, Text: "one"},
			{StartMs: 1500, Text: "two"},
		},
	}
	id := writer.MaterialID(base)
	want := writer.Digest(base.VideoID, base.Title, base.ChannelName, base.Description, "one\ntwo")
	if id != want {
		t.Fatalf("MaterialID = %q, want %q", id, want)
	}

	changes := []struct {
		name   string
		change func(*transcript.Transcript)
	}{
		{"video ID", func(tr *transcript.Transcript) {
			tr.VideoID = "aaaaaaaaaaa"
			tr.VideoURL = "https://www.youtube.com/watch?v=aaaaaaaaaaa"
		}},
		{"title", func(tr *transcript.Transcript) { tr.Title = "Other" }},
		{"channel name", func(tr *transcript.Transcript) { tr.ChannelName = "Other" }},
		{"description", func(tr *transcript.Transcript) { tr.Description = "Other" }},
		{"segment text", func(tr *transcript.Transcript) { tr.Segments[0].Text = "changed" }},
		{"segment count", func(tr *transcript.Transcript) { tr.Segments = tr.Segments[:1] }},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			other := base
			other.Segments = append([]transcript.Segment(nil), base.Segments...)
			tc.change(&other)
			if writer.MaterialID(other) == id {
				t.Errorf("MaterialID did not change with the %s", tc.name)
			}
		})
	}

	t.Run("segment start times do not matter", func(t *testing.T) {
		other := base
		other.Segments = append([]transcript.Segment(nil), base.Segments...)
		other.Segments[0].StartMs = 9999
		if got := writer.MaterialID(other); got != id {
			t.Errorf("MaterialID = %q, want %q (StartMs must not affect it)", got, id)
		}
	})
}
