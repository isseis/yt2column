//go:build test

package artifact

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

// testVideoID is a valid video ID.
const testVideoID = "dQw4w9WgXcQ"

// newTestStore opens a fresh artifacts directory and closes it at cleanup.
func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, dir
}

// shortenOf builds a shorten record that consumes input.
func shortenOf(input string) writer.StageRecord {
	rec := validGeneration()
	rec.Stage = writer.StageShorten
	rec.Title = "Short title"
	rec.Body = "Short body."
	rec.Input = input
	rec.ID = recordID(rec)
	return rec
}

// reviewOf builds a review record that consumes input.
func reviewOf(input string) writer.StageRecord {
	rec := validReview()
	rec.Input = input
	rec.ID = recordID(rec)
	return rec
}

// mustSave saves a record and returns its identifier.
func mustSave(t *testing.T, store *Store, rec writer.StageRecord) string {
	t.Helper()
	id, err := store.Save(context.Background(), testVideoID, rec)
	if err != nil {
		t.Fatalf("Save error = %v", err)
	}
	return id
}

func TestNewStoreRejectsBadDir(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "absent")
		if _, err := NewStore(dir); !errors.Is(err, ErrMissing) {
			t.Fatalf("NewStore error = %v, want ErrMissing", err)
		}
		if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("NewStore created %s (stat error = %v)", dir, err)
		}
	})
	t.Run("a file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if _, err := NewStore(path); !errors.Is(err, ErrMissing) {
			t.Fatalf("NewStore error = %v, want ErrMissing", err)
		}
	})
	t.Run("a symbolic link", func(t *testing.T) {
		base := t.TempDir()
		realDir := filepath.Join(base, "real")
		if err := os.Mkdir(realDir, 0o700); err != nil {
			t.Fatalf("mkdir real: %v", err)
		}
		link := filepath.Join(base, "link")
		if err := os.Symlink(realDir, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		for _, variant := range []string{link, link + "/", link + "/."} {
			t.Run(variant, func(t *testing.T) {
				if _, err := NewStore(variant); !errors.Is(err, ErrMissing) {
					t.Fatalf("NewStore(%q) error = %v, want ErrMissing", variant, err)
				}
			})
		}
	})
	t.Run("swapped after the check", func(t *testing.T) {
		base := t.TempDir()
		dir := filepath.Join(base, "dir")
		other := filepath.Join(base, "other")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("mkdir dir: %v", err)
		}
		if err := os.Mkdir(other, 0o700); err != nil {
			t.Fatalf("mkdir other: %v", err)
		}
		swap := func(_ string) {
			if err := os.Rename(dir, dir+"-old"); err != nil {
				t.Fatalf("rename dir: %v", err)
			}
			if err := os.Rename(other, dir); err != nil {
				t.Fatalf("rename other: %v", err)
			}
		}
		if _, err := newStore(dir, swap); !errors.Is(err, ErrMissing) {
			t.Fatalf("newStore error = %v, want ErrMissing", err)
		}
	})
}

func TestSaveLoadRoundTrip(t *testing.T) {
	store, dir := newTestStore(t)
	materialID := validGeneration().Input
	generation := validGeneration()
	genID := mustSave(t, store, generation)
	shorten := shortenOf(genID)
	shortenID := mustSave(t, store, shorten)
	review := reviewOf(shortenID)
	mustSave(t, store, review)

	t.Run("files use the fixed names", func(t *testing.T) {
		for _, stage := range []writer.Stage{writer.StageGeneration, writer.StageShorten, writer.StageReview} {
			path := filepath.Join(dir, testVideoID+"."+stage.String()+".json")
			if _, err := os.Stat(path); err != nil {
				t.Errorf("stat %s: %v", path, err)
			}
		}
	})

	t.Run("generation", func(t *testing.T) {
		chain, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, materialID)
		if err != nil {
			t.Fatalf("Load error = %v", err)
		}
		if !sameRecord(chain.Generation, generation) {
			t.Errorf("generation = %+v, want %+v", chain.Generation, generation)
		}
		if chain.Shorten != nil || chain.Review != nil {
			t.Errorf("chain = %+v, want only the generation", chain)
		}
	})

	t.Run("shorten", func(t *testing.T) {
		chain, err := store.Load(context.Background(), testVideoID, writer.StageShorten, materialID)
		if err != nil {
			t.Fatalf("Load error = %v", err)
		}
		if chain.Shorten == nil || !sameRecord(*chain.Shorten, shorten) {
			t.Errorf("shorten = %+v, want %+v", chain.Shorten, shorten)
		}
		if chain.Review != nil {
			t.Errorf("review = %+v, want nil", chain.Review)
		}
	})

	t.Run("review", func(t *testing.T) {
		chain, err := store.Load(context.Background(), testVideoID, writer.StageReview, materialID)
		if err != nil {
			t.Fatalf("Load error = %v", err)
		}
		if chain.Review == nil || !sameRecord(*chain.Review, review) {
			t.Errorf("review = %+v, want %+v", chain.Review, review)
		}
		if chain.Shorten == nil {
			t.Errorf("shorten = nil, want the shorten stage")
		}
	})

	t.Run("a save replaces the previous artifact", func(t *testing.T) {
		replacement := validGeneration()
		replacement.Title = "Another title"
		replacement.ID = recordID(replacement)
		mustSave(t, store, replacement)
		chain, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, materialID)
		if err != nil {
			t.Fatalf("Load error = %v", err)
		}
		if chain.Generation.Title != "Another title" {
			t.Errorf("generation title = %q, want the replacement", chain.Generation.Title)
		}
	})
}

func TestSaveFileMode(t *testing.T) {
	store, dir := newTestStore(t)
	mustSave(t, store, validGeneration())
	info, err := os.Stat(filepath.Join(dir, testVideoID+".generation.json"))
	if err != nil {
		t.Fatalf("stat artifact: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("artifact mode = %04o, want 0600", got)
	}
}

func TestSaveOnlyValidatedVideoID(t *testing.T) {
	store, _ := newTestStore(t)
	for _, videoID := range []string{"../escape", "short", "", "dQw4w9WgXcQ/"} {
		t.Run(videoID, func(t *testing.T) {
			if _, err := store.Save(context.Background(), videoID, validGeneration()); !errors.Is(err, errInvalidVideoID) {
				t.Errorf("Save error = %v, want errInvalidVideoID", err)
			}
			if _, err := store.Load(context.Background(), videoID, writer.StageGeneration, "m"); !errors.Is(err, errInvalidVideoID) {
				t.Errorf("Load error = %v, want errInvalidVideoID", err)
			}
		})
	}
}

func TestLoadMissing(t *testing.T) {
	store, _ := newTestStore(t)
	if _, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, "m"); !errors.Is(err, ErrMissing) {
		t.Fatalf("Load error = %v, want ErrMissing", err)
	}
}

func TestLoadRejectsSymlinkAndFIFO(t *testing.T) {
	// The bytes of a valid artifact, written under a name no reader looks at,
	// prove that the rejection below is about the file type, not the content.
	valid := mustMarshal(t, validGeneration())

	t.Run("relative symbolic link inside the root", func(t *testing.T) {
		store, dir := newTestStore(t)
		if _, err := Parse(valid); err != nil {
			t.Fatalf("Parse(valid) error = %v, want nil", err)
		}
		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, valid, 0o600); err != nil {
			t.Fatalf("write target: %v", err)
		}
		link := filepath.Join(dir, testVideoID+".generation.json")
		if err := os.Symlink("target", link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		_, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, "m")
		if !errors.Is(err, ErrMalformed) {
			t.Fatalf("Load error = %v, want ErrMalformed", err)
		}
	})

	t.Run("symbolic link outside the root", func(t *testing.T) {
		store, dir := newTestStore(t)
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, valid, 0o600); err != nil {
			t.Fatalf("write outside: %v", err)
		}
		link := filepath.Join(dir, testVideoID+".generation.json")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		_, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, "m")
		if !errors.Is(err, ErrMalformed) {
			t.Fatalf("Load error = %v, want ErrMalformed", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		store, dir := newTestStore(t)
		if err := os.Mkdir(filepath.Join(dir, testVideoID+".generation.json"), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		_, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, "m")
		if !errors.Is(err, ErrMalformed) {
			t.Fatalf("Load error = %v, want ErrMalformed", err)
		}
	})

	t.Run("FIFO", func(t *testing.T) {
		store, dir := newTestStore(t)
		if err := syscall.Mkfifo(filepath.Join(dir, testVideoID+".generation.json"), 0o600); err != nil {
			t.Fatalf("mkfifo: %v", err)
		}
		_, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, "m")
		if !errors.Is(err, ErrMalformed) {
			t.Fatalf("Load error = %v, want ErrMalformed", err)
		}
	})
}

// swapWithSymlink returns a newStore hook that, when called for the artifact
// name, replaces it with a symbolic link to linkTarget.
func swapWithSymlink(t *testing.T, dir, name, linkTarget string) func(string) {
	t.Helper()
	return func(hit string) {
		if filepath.Base(hit) != name {
			return
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatalf("remove artifact: %v", err)
		}
		if err := os.Symlink(linkTarget, filepath.Join(dir, name)); err != nil {
			t.Fatalf("symlink: %v", err)
		}
	}
}

func TestLoadRejectsSwapAfterLstat(t *testing.T) {
	valid := mustMarshal(t, validGeneration())
	name := testVideoID + ".generation.json"

	t.Run("replaced by a relative symbolic link", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "target"), valid, 0o600); err != nil {
			t.Fatalf("write target: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), valid, 0o600); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
		store, err := newStore(dir, swapWithSymlink(t, dir, name, "target"))
		if err != nil {
			t.Fatalf("newStore error = %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })
		if _, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, "m"); !errors.Is(err, ErrMalformed) {
			t.Fatalf("Load error = %v, want ErrMalformed", err)
		}
	})

	t.Run("replaced by an escaping symbolic link", func(t *testing.T) {
		dir := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, valid, 0o600); err != nil {
			t.Fatalf("write outside: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), valid, 0o600); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
		store, err := newStore(dir, swapWithSymlink(t, dir, name, outside))
		if err != nil {
			t.Fatalf("newStore error = %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })
		if _, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, "m"); !errors.Is(err, ErrMalformed) {
			t.Fatalf("Load error = %v, want ErrMalformed", err)
		}
	})
}

func TestSaveDoesNotFollowSymlink(t *testing.T) {
	store, dir := newTestStore(t)
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("untouched"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	name := testVideoID + ".generation.json"
	if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	mustSave(t, store, validGeneration())

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "untouched" {
		t.Errorf("target = %q, want the untouched content", got)
	}
	info, err := os.Lstat(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("lstat artifact: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("artifact is %v, want a regular file (the rename replaces the link)", info.Mode())
	}
	if _, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, validGeneration().Input); err != nil {
		t.Errorf("Load error = %v, want the saved artifact to be readable", err)
	}
}

func TestCollectTempFiles(t *testing.T) {
	store, dir := newTestStore(t)
	stale := tempPrefix + testVideoID + ".generation.deadbeef" + tempSuffix
	if err := os.WriteFile(filepath.Join(dir, stale), []byte("partial"), 0o600); err != nil {
		t.Fatalf("write stale temp: %v", err)
	}
	otherVideo := tempPrefix + "aaaaaaaaaaa" + ".generation.deadbeef" + tempSuffix
	if err := os.WriteFile(filepath.Join(dir, otherVideo), []byte("partial"), 0o600); err != nil {
		t.Fatalf("write other temp: %v", err)
	}
	mustSave(t, store, validGeneration())

	if _, err := os.Stat(filepath.Join(dir, stale)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stale temp file still exists (stat error = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, otherVideo)); err != nil {
		t.Errorf("other video's temp file is gone: %v", err)
	}
}

func TestLoadRejectsOversizedFile(t *testing.T) {
	store, dir := newTestStore(t)
	name := filepath.Join(dir, testVideoID+".generation.json")
	if err := os.WriteFile(name, make([]byte, maxArtifactBytes+1), 0o600); err != nil {
		t.Fatalf("write oversized: %v", err)
	}
	if _, err := store.Load(context.Background(), testVideoID, writer.StageGeneration, "m"); !errors.Is(err, ErrMalformed) {
		t.Fatalf("Load error = %v, want ErrMalformed", err)
	}
}

func TestLoadRejectsWrongStageInFile(t *testing.T) {
	// A generation artifact stored under the shorten name is a mismatch, not a
	// malformed file: the stage inside the file does not match the request.
	store, dir := newTestStore(t)
	data := mustMarshal(t, validGeneration())
	if err := os.WriteFile(filepath.Join(dir, testVideoID+".shorten.json"), data, 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if _, err := store.Load(context.Background(), testVideoID, writer.StageShorten, validGeneration().Input); !errors.Is(err, ErrMismatch) {
		t.Fatalf("Load error = %v, want ErrMismatch", err)
	}
}

func TestSaveLoadRejectUnknownStage(t *testing.T) {
	store, _ := newTestStore(t)
	for _, stage := range []writer.Stage{writer.StageUnset, writer.Stage(99)} {
		t.Run(stage.String(), func(t *testing.T) {
			rec := validGeneration()
			rec.Stage = stage
			if _, err := store.Save(context.Background(), testVideoID, rec); !errors.Is(err, errInvalidStage) {
				t.Errorf("Save error = %v, want errInvalidStage", err)
			}
			if _, err := store.Load(context.Background(), testVideoID, stage, "m"); !errors.Is(err, errInvalidStage) {
				t.Errorf("Load error = %v, want errInvalidStage", err)
			}
		})
	}
}
