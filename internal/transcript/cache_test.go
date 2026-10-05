//go:build test

package transcript

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCachePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID

	slotDir, err := prepareWriteSlot(dir, id, slotNameA)
	if err != nil {
		t.Fatalf("prepareWriteSlot error = %v", err)
	}
	// The files start with a looser mode so the assertions below fail when
	// persistSlot does not tighten them.
	for _, path := range []string{subtitlesPath(slotDir, id), infoPath(slotDir, id)} {
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := persistSlot(slotDir, id); err != nil {
		t.Fatalf("persistSlot error = %v", err)
	}
	if err := commitCache(dir, id, slotNameA); err != nil {
		t.Fatalf("commitCache error = %v", err)
	}

	assertMode(t, dir, 0o700)
	assertMode(t, slotDir, 0o700)
	assertMode(t, subtitlesPath(slotDir, id), 0o600)
	assertMode(t, infoPath(slotDir, id), 0o600)
	assertMode(t, pointerPath(dir, id), 0o600)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %o, want %o", path, got, want)
	}
}

func TestPruneCache(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	fake := &fakeCommandExecutor{}
	source := newTestSource(t, dir, fake, nil)
	ctx := t.Context()

	stale := generation{
		subtitles: `{"events":[{"tStartMs":0,"segs":[{"utf8":"stale"}]}]}`,
		info:      `{"id":"x","title":"stale","channel":"stale"}`,
	}

	// Two valid caches: the real fixtures and a second video whose slots must
	// survive the prune.
	realSubtitles, realInfo := placeRealCache(t, dir, testdataRealVideoID)
	otherID := testVideoID(t, "abcdefghijk")
	otherGen := generation{
		subtitles: `{"events":[{"tStartMs":0,"segs":[{"utf8":"other"}]}]}`,
		info:      `{"id":"` + otherID + `","title":"other","channel":"other"}`,
	}
	placeSlot(t, dir, otherID, slotNameB, otherGen)
	placePointer(t, dir, otherID, slotNameB)

	// Dangling entries: a missing pointer and an invalid pointer.
	danglingID := testVideoID(t, "dQw4w9WgXcQ")
	placeSlot(t, dir, danglingID, slotNameA, stale)
	placeSlot(t, dir, danglingID, slotNameB, stale)
	placePointerTmp(t, dir, danglingID, slotNameA)
	invalidPointerID := testVideoID(t, "zzzzzzzzzzz")
	placeSlot(t, dir, invalidPointerID, slotNameA, stale)
	placePointer(t, dir, invalidPointerID, "x")

	// Entries the prune must never touch: names outside the rule, an invalid
	// video ID part, and entries of a type the rule does not expect.
	unrelated := map[string]string{
		"random.txt":                  "keep",
		danglingID + ".notes":         "keep",
		"has.dot.a":                   "keep",
		"short." + slotNameA:          "keep",
		danglingID + ".ja-orig.json3": "keep",
	}
	for name, content := range unrelated {
		writeTestFile(t, filepath.Join(dir, name), content)
	}
	symlinkID := testVideoID(t, "aaaaaaaaaaa")
	symlinkTarget := filepath.Join(dir, "symlink-target.txt")
	writeTestFile(t, symlinkTarget, "target")
	symlinkPath := slotDirPath(dir, symlinkID, slotNameA)
	if err := os.Symlink(symlinkTarget, symlinkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	pointerSymlinkID := testVideoID(t, "bbbbbbbbbbb")
	pointerTarget := filepath.Join(dir, "pointer-target.txt")
	writeTestFile(t, pointerTarget, "target")
	pointerSymlink := pointerPath(dir, pointerSymlinkID)
	if err := os.Symlink(pointerTarget, pointerSymlink); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	tmpDirID := testVideoID(t, "ccccccccccc")
	tmpDir := pointerTmpPath(dir, tmpDirID)
	if err := os.Mkdir(tmpDir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", tmpDir, err)
	}

	if err := source.PruneCache(ctx); err != nil {
		t.Fatalf("PruneCache error = %v", err)
	}

	// Dangling entries are gone.
	for _, path := range []string{
		slotDirPath(dir, danglingID, slotNameA),
		slotDirPath(dir, danglingID, slotNameB),
		pointerTmpPath(dir, danglingID),
		slotDirPath(dir, invalidPointerID, slotNameA),
		pointerPath(dir, invalidPointerID),
	} {
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s still exists after prune (error = %v)", path, err)
		}
	}

	// Valid caches and unrelated entries are unchanged.
	realSlotDir := slotDirPath(dir, testdataRealVideoID, slotNameA)
	assertFileContent(t, subtitlesPath(realSlotDir, testdataRealVideoID), string(realSubtitles))
	assertFileContent(t, infoPath(realSlotDir, testdataRealVideoID), string(realInfo))
	assertFileContent(t, subtitlesPath(slotDirPath(dir, otherID, slotNameB), otherID), otherGen.subtitles)
	assertFileContent(t, pointerPath(dir, otherID), slotNameB)
	for name, content := range unrelated {
		assertFileContent(t, filepath.Join(dir, name), content)
	}
	assertFileContent(t, symlinkTarget, "target")
	assertFileContent(t, pointerTarget, "target")
	if info, err := os.Lstat(symlinkPath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("slot symlink was touched: mode = %v, error = %v", info, err)
	}
	if info, err := os.Lstat(pointerSymlink); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("pointer symlink was touched: mode = %v, error = %v", info, err)
	}
	if info, err := os.Lstat(tmpDir); err != nil || !info.IsDir() {
		t.Errorf("temporary-file directory was touched: mode = %v, error = %v", info, err)
	}

	// A Fetch after the prune serves each valid cache without running yt-dlp.
	transcript, err := source.Fetch(ctx, "https://www.youtube.com/watch?v="+testdataRealVideoID)
	if err != nil {
		t.Fatalf("Fetch after prune error = %v", err)
	}
	wantSegments := subtitlesOracle(t, realSubtitles)
	if len(transcript.Segments) != len(wantSegments) {
		t.Errorf("segments = %d, want %d", len(transcript.Segments), len(wantSegments))
	}
	other, err := source.Fetch(ctx, "https://youtu.be/"+otherID)
	if err != nil {
		t.Fatalf("Fetch after prune error = %v", err)
	}
	if len(other.Segments) != 1 || other.Segments[0].Text != "other" {
		t.Errorf("other segments = %+v, want the cached text", other.Segments)
	}
	if fake.callCount() != 0 {
		t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
	}
}

func TestPruneCachePartialFailure(t *testing.T) {
	requireNonRoot(t)
	dir := filepath.Join(t.TempDir(), "cache")
	source := newTestSource(t, dir, &fakeCommandExecutor{}, nil)
	ctx := t.Context()

	stale := generation{subtitles: "{}", info: "{}"}
	failingIDs := []string{
		testVideoID(t, "aaaaaaaaaaa"),
		testVideoID(t, "bbbbbbbbbbb"),
	}
	for _, id := range failingIDs {
		slotDir := slotDirPath(dir, id, slotNameA)
		placeSlot(t, dir, id, slotNameA, stale)
		writeTestFile(t, filepath.Join(slotDir, "busy.txt"), "x")
		chmodForTest(t, slotDir, 0o500)
	}
	intactID := testVideoID(t, "dQw4w9WgXcQ")
	placeSlot(t, dir, intactID, slotNameA, stale)

	err := source.PruneCache(ctx)
	if err == nil {
		t.Fatal("PruneCache error = nil, want the failed entries reported")
	}
	for _, id := range failingIDs {
		if !strings.Contains(err.Error(), slotDirPath(dir, id, slotNameA)) {
			t.Errorf("PruneCache error %q does not name %s", err, slotDirPath(dir, id, slotNameA))
		}
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("PruneCache error = %v, want a permission failure", err)
	}

	// The deletable dangling entry was removed despite the failures.
	if _, err := os.Lstat(slotDirPath(dir, intactID, slotNameA)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("deletable dangling slot still exists (error = %v)", err)
	}
	for _, id := range failingIDs {
		if _, err := os.Lstat(slotDirPath(dir, id, slotNameA)); err != nil {
			t.Errorf("failing slot %s disappeared: %v", slotDirPath(dir, id, slotNameA), err)
		}
	}
}

func TestReadMissingCacheFile(t *testing.T) {
	dir := t.TempDir()
	missingSubtitles := filepath.Join(dir, "missing.json3")
	missingInfo := filepath.Join(dir, "missing.info.json")

	// A file that disappeared between the existence check and the read is a
	// miss, not a parse error, so the next Fetch re-fetches it.
	if _, err := readSubtitlesFile(missingSubtitles); !errors.Is(err, errFileMissing) {
		t.Errorf("readSubtitlesFile error = %v, want errFileMissing", err)
	}
	if _, err := readInfoFile(missingInfo, testdataRealVideoID); !errors.Is(err, errFileMissing) {
		t.Errorf("readInfoFile error = %v, want errFileMissing", err)
	}
}

func TestRemoveCache(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	fake := &fakeCommandExecutor{}
	source := newTestSource(t, dir, fake, nil)
	ctx := t.Context()
	id := testdataRealVideoID

	subtitles, info := placeRealCache(t, dir, id)
	// Dangling entries of the same video are removed too.
	placeSlot(t, dir, id, slotNameB, generation{subtitles: "{}", info: "{}"})
	placePointerTmp(t, dir, id, slotNameB)
	otherID := testVideoID(t, "abcdefghijk")
	placeSlot(t, dir, otherID, slotNameA, generation{subtitles: "{}", info: "{}"})
	placePointer(t, dir, otherID, slotNameA)
	unrelatedPath := filepath.Join(dir, id+".notes")
	writeTestFile(t, unrelatedPath, "notes")

	if err := source.RemoveCache(ctx, "https://www.youtube.com/watch?v="+id); err != nil {
		t.Fatalf("RemoveCache error = %v", err)
	}

	for _, path := range []string{
		pointerPath(dir, id),
		slotDirPath(dir, id, slotNameA),
		slotDirPath(dir, id, slotNameB),
		pointerTmpPath(dir, id),
	} {
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s still exists after RemoveCache (error = %v)", path, err)
		}
	}
	assertFileContent(t, pointerPath(dir, otherID), slotNameA)
	assertFileContent(t, unrelatedPath, "notes")

	// The next Fetch is a cache miss and repopulates the cache from yt-dlp.
	fake.behavior = func(call fakeCommandCall) error {
		writeSlotFiles(t, slotDirFromArgs(t, call.args), id, string(subtitles), string(info))
		return nil
	}
	transcript, err := source.Fetch(ctx, "https://youtu.be/"+id)
	if err != nil {
		t.Fatalf("Fetch after RemoveCache error = %v", err)
	}
	if fake.callCount() != 1 {
		t.Fatalf("yt-dlp ran %d times, want 1", fake.callCount())
	}
	if transcript.Title == "" || transcript.VideoID != id {
		t.Errorf("transcript = %+v, want the fetched video", transcript)
	}
}

func TestRemoveCacheUnreadablePointer(t *testing.T) {
	requireNonRoot(t)
	dir := filepath.Join(t.TempDir(), "cache")
	source := newTestSource(t, dir, &fakeCommandExecutor{}, nil)
	id := testdataRealVideoID
	placeRealCache(t, dir, id)
	chmodForTest(t, pointerPath(dir, id), 0o000)

	if err := source.RemoveCache(t.Context(), "https://www.youtube.com/watch?v="+id); err != nil {
		t.Fatalf("RemoveCache error = %v, want the unreadable pointer removed", err)
	}
	for _, path := range []string{pointerPath(dir, id), slotDirPath(dir, id, slotNameA)} {
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s still exists after RemoveCache (error = %v)", path, err)
		}
	}
}

func TestRemoveCacheNoEntries(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	source := newTestSource(t, dir, &fakeCommandExecutor{}, nil)
	ctx := t.Context()
	id := testdataRealVideoID

	otherID := testVideoID(t, "abcdefghijk")
	placeSlot(t, dir, otherID, slotNameA, generation{subtitles: "{}", info: "{}"})
	placePointer(t, dir, otherID, slotNameA)
	unrelatedPath := filepath.Join(dir, id+".notes")
	writeTestFile(t, unrelatedPath, "notes")
	before := readDirNames(t, dir)

	if err := source.RemoveCache(ctx, "https://www.youtube.com/watch?v="+id); err != nil {
		t.Fatalf("RemoveCache error = %v", err)
	}
	after := readDirNames(t, dir)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("cache directory changed: before %q, after %q", before, after)
	}

	// An invalid URL fails without touching the directory.
	err := source.RemoveCache(ctx, "https://example.com/watch?v="+id)
	if !errors.Is(err, ErrInvalidVideoURL) {
		t.Fatalf("RemoveCache error = %v, want ErrInvalidVideoURL", err)
	}
	if afterInvalid := readDirNames(t, dir); strings.Join(after, "\n") != strings.Join(afterInvalid, "\n") {
		t.Errorf("cache directory changed after an invalid URL: %q -> %q", after, afterInvalid)
	}
	assertFileContent(t, unrelatedPath, "notes")
}

func TestRemoveCacheFailure(t *testing.T) {
	requireNonRoot(t)
	id := testdataRealVideoID

	t.Run("pointer removal fails and keeps the valid cache", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		_, info := placeRealCache(t, dir, id)
		chmodForTest(t, dir, 0o500)

		fake := &fakeCommandExecutor{}
		source := newTestSource(t, dir, fake, nil)
		err := source.RemoveCache(t.Context(), "https://www.youtube.com/watch?v="+id)
		if err == nil {
			t.Fatal("RemoveCache error = nil, want a failure")
		}
		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("RemoveCache error = %v, want a permission failure", err)
		}

		// The cache is still valid, so a non-forced Fetch serves it without
		// running yt-dlp.
		transcript, err := source.Fetch(t.Context(), "https://youtu.be/"+id)
		if err != nil {
			t.Fatalf("Fetch error = %v", err)
		}
		if fake.callCount() != 0 {
			t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
		}
		if want := infoOracle(t, info); transcript.Title != want.Title || transcript.Description != want.Description {
			t.Errorf("transcript metadata = %+v, want %+v from the kept cache", transcript, want)
		}
	})

	t.Run("dangling slot removal fails and the next Fetch recovers", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		subtitles, info := placeRealCache(t, dir, id)
		staleDir := slotDirPath(dir, id, slotNameB)
		writeTestFile(t, filepath.Join(staleDir, "busy.txt"), "x")
		chmodForTest(t, staleDir, 0o500)

		fake := &fakeCommandExecutor{}
		source := newTestSource(t, dir, fake, nil)
		err := source.RemoveCache(t.Context(), "https://www.youtube.com/watch?v="+id)
		if err == nil {
			t.Fatal("RemoveCache error = nil, want a failure")
		}
		if !strings.Contains(err.Error(), staleDir) {
			t.Errorf("RemoveCache error %q does not name %s", err, staleDir)
		}

		// A transient failure: the next Fetch is a cache miss, runs yt-dlp,
		// and leaves no dangling entries behind.
		if err := os.Chmod(staleDir, 0o700); err != nil {
			t.Fatalf("chmod %s: %v", staleDir, err)
		}
		fake.behavior = func(call fakeCommandCall) error {
			writeSlotFiles(t, slotDirFromArgs(t, call.args), id, string(subtitles), string(info))
			return nil
		}
		transcript, err := source.Fetch(t.Context(), "https://www.youtube.com/watch?v="+id)
		if err != nil {
			t.Fatalf("Fetch error = %v", err)
		}
		if fake.callCount() != 1 {
			t.Fatalf("yt-dlp ran %d times, want 1", fake.callCount())
		}
		if transcript.Title == "" {
			t.Error("Fetch returned an empty title")
		}
		for _, path := range []string{staleDir, pointerTmpPath(dir, id)} {
			if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("%s still exists after the recovery Fetch (error = %v)", path, err)
			}
		}
	})
}

func TestRemoveCacheCanceled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	placeRealCache(t, dir, id)
	source := newTestSource(t, dir, &fakeCommandExecutor{}, nil)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := source.RemoveCache(ctx, "https://www.youtube.com/watch?v="+id)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RemoveCache error = %v, want context.Canceled", err)
	}
	assertFileContent(t, pointerPath(dir, id), slotNameA)
}

func TestPruneCacheCanceled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	placeSlot(t, dir, id, slotNameA, generationFor(id, "stale"))
	source := newTestSource(t, dir, &fakeCommandExecutor{}, nil)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := source.PruneCache(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PruneCache error = %v, want context.Canceled", err)
	}
	if _, err := os.Lstat(slotDirPath(dir, id, slotNameA)); err != nil {
		t.Errorf("cancellation deleted a dangling entry: %v", err)
	}
}

func TestRemoveDanglingCanceled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	placeSlot(t, dir, id, slotNameA, generationFor(id, "stale"))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := removeDangling(ctx, dir, id, pointerMissing); !errors.Is(err, context.Canceled) {
		t.Fatalf("removeDangling error = %v, want context.Canceled", err)
	}
	if _, err := os.Lstat(slotDirPath(dir, id, slotNameA)); err != nil {
		t.Errorf("cancellation deleted a dangling entry: %v", err)
	}
}

// readDirNames returns the sorted names in dir.
func readDirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Errorf("%s = %q, want %q", path, data, want)
	}
}

// TestReadFileBounded checks the reader's own limit with a maxBytes far below
// the parser limits, so only readFileBounded can reject the file.
func TestReadFileBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	const content = "0123456789"
	writeTestFile(t, path, content)

	if _, err := readFileBounded(path, int64(len(content))-1); !errors.Is(err, errInputTooLarge) {
		t.Errorf("readFileBounded over the limit error = %v, want errInputTooLarge", err)
	}
	data, err := readFileBounded(path, int64(len(content)))
	if err != nil || string(data) != content {
		t.Errorf("readFileBounded at the limit = %q, %v, want %q, nil", data, err, content)
	}
}

// TestSeedCacheForTestLayout pins the layout the seeder produces: the bytes it
// is given, unvalidated, in slot a, and a pointer naming slot a.
func TestSeedCacheForTestLayout(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	// Not valid json3: the seeder must not check the content.
	subtitles, info := []byte("truncated"), []byte("{}")
	if err := SeedCacheForTest(dir, id, subtitles, info); err != nil {
		t.Fatalf("SeedCacheForTest error = %v", err)
	}
	slotDir := slotDirPath(dir, id, slotNameA)
	for path, want := range map[string][]byte{
		subtitlesPath(slotDir, id): subtitles,
		infoPath(slotDir, id):      info,
		pointerPath(dir, id):       []byte(slotNameA),
	} {
		got, err := os.ReadFile(path) //nolint:gosec // the path is inside a test temp directory
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
	if _, err := os.Lstat(slotDirPath(dir, id, slotNameB)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("slot b exists or failed: %v", err)
	}
}
