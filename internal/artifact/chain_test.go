//go:build test

package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
