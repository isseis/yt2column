package transcript

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Fixed names of the per-video cache layout. A video has two slots, one
// pointer, and one pointer temporary file; a slot holds exactly the subtitle
// file and the info.json of one yt-dlp run.
const (
	slotNameA       = "a"
	slotNameB       = "b"
	currentSuffix   = "current"
	tmpSuffix       = "current.tmp"
	subtitlesSuffix = ".ja.json3"
	infoSuffix      = ".info.json"
)

// Static errors of the cache layer. An entry of an unexpected type is never
// touched, and is not an error by itself.
var (
	errFileMissing       = errors.New("cache file is missing")
	errNotRegularFile    = errors.New("cache entry is not a regular file")
	errSlotNotDirectory  = errors.New("cache slot is not a directory")
	errPointerNotRegular = errors.New("cache pointer is not a regular file")
)

// pointerState describes what the pointer of one video contains.
type pointerState int

const (
	// pointerMissing means the pointer does not exist: there is no valid
	// generation.
	pointerMissing pointerState = iota
	// pointerSlotA and pointerSlotB mean the slot of that name is valid.
	pointerSlotA
	pointerSlotB
	// pointerInvalid means a regular pointer holds something other than a or b.
	pointerInvalid
	// pointerNotRegular means the pointer exists but is not a regular file.
	pointerNotRegular
)

// validSlot returns the slot the pointer refers to, if any.
func (p pointerState) validSlot() (string, bool) {
	switch p {
	case pointerSlotA:
		return slotNameA, true
	case pointerSlotB:
		return slotNameB, true
	default:
		return "", false
	}
}

func slotDirPath(dir, id, slot string) string {
	return filepath.Join(dir, id+"."+slot)
}

func pointerPath(dir, id string) string {
	return filepath.Join(dir, id+"."+currentSuffix)
}

func pointerTmpPath(dir, id string) string {
	return filepath.Join(dir, id+"."+tmpSuffix)
}

func subtitlesPath(slotDir, id string) string {
	return filepath.Join(slotDir, id+subtitlesSuffix)
}

func infoPath(slotDir, id string) string {
	return filepath.Join(slotDir, id+infoSuffix)
}

// readPointer classifies the pointer of one video. It never reads more than
// one byte of the pointer's content, so an oversized pointer is invalid
// instead of being loaded.
func readPointer(dir, id string) (pointerState, error) {
	path := pointerPath(dir, id)
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return pointerMissing, nil
		}
		return pointerMissing, err
	}
	if !info.Mode().IsRegular() {
		return pointerNotRegular, nil
	}
	if info.Size() != 1 {
		return pointerInvalid, nil
	}
	file, err := os.Open(path) //nolint:gosec // the path is a fixed name under the caller-provided cache directory
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return pointerMissing, nil
		}
		// An unreadable pointer is an error, not an invalid one: treating it
		// as invalid would let the cleanup delete an intact cache.
		return pointerMissing, err
	}
	defer func() { _ = file.Close() }()
	var content [1]byte
	switch _, err := io.ReadFull(file, content[:]); {
	case err == nil:
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		// The file was truncated under us: there is no valid generation.
		return pointerMissing, nil
	default:
		return pointerMissing, err
	}
	switch content[0] {
	case 'a':
		return pointerSlotA, nil
	case 'b':
		return pointerSlotB, nil
	default:
		return pointerInvalid, nil
	}
}

// writeSlot returns the slot a run must write to: the slot the pointer does
// not refer to, or slot a when there is no valid pointer.
func writeSlot(state pointerState) string {
	current, ok := state.validSlot()
	if !ok || current == slotNameB {
		return slotNameA
	}
	return slotNameB
}

// cacheFileExists reports whether the fixed-name entry exists, whatever its
// type: a non-regular entry still counts as present and fails when read.
func cacheFileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// readFileBounded reads a cache output file. A non-regular entry is refused
// without being followed, and a file larger than maxBytes is refused without
// being loaded.
func readFileBounded(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errNotRegularFile
	}
	if info.Size() > maxBytes {
		return nil, errInputTooLarge
	}
	file, err := os.Open(path) //nolint:gosec // the path is a fixed name under the caller-provided cache directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errInputTooLarge
	}
	return data, nil
}

// readSubtitlesFile reads and parses the subtitle file at path. A missing file
// returns errFileMissing; every other failure is a *ParseError.
func readSubtitlesFile(path string) ([]Segment, error) {
	data, err := readFileBounded(path, maxSubtitlesBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, errFileMissing
		}
		return nil, &ParseError{Path: path, Err: fmt.Errorf("%w: %w", ErrParseSubtitles, err)}
	}
	return parseSubtitles(path, data)
}

// readInfoFile reads and parses the info.json at path. A missing file returns
// errFileMissing; every other failure is a *ParseError.
func readInfoFile(path, wantID string) (videoInfo, error) {
	data, err := readFileBounded(path, maxInfoBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return videoInfo{}, errFileMissing
		}
		return videoInfo{}, &ParseError{Path: path, Err: fmt.Errorf("%w: %w", ErrParseInfo, err)}
	}
	return parseInfo(path, wantID, data)
}

// prepareWriteSlot creates an empty slot directory for the next run, replacing
// a stale directory so leftovers cannot be mistaken for this run's output. A
// non-directory entry at the fixed name is left untouched and fails.
func prepareWriteSlot(dir, id, slot string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := slotDirPath(dir, id, slot)
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if !info.IsDir() {
			return "", fmt.Errorf("%w: %s", errSlotNotDirectory, path)
		}
		if err := os.RemoveAll(path); err != nil {
			return "", err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return "", err
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

// persistSlot makes the run output durable before the commit: it sets the
// cache file permissions and syncs the files and the slot directory.
func persistSlot(slotDir, id string) error {
	for _, path := range []string{subtitlesPath(slotDir, id), infoPath(slotDir, id)} {
		if err := os.Chmod(path, 0o600); err != nil {
			return err
		}
		if err := syncFile(path); err != nil {
			return err
		}
	}
	return syncDir(slotDir)
}

func syncFile(path string) error {
	file, err := os.Open(path) //nolint:gosec // the path is a fixed name under the caller-provided cache directory
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return file.Sync()
}

func syncDir(path string) error {
	dir, err := os.Open(path) //nolint:gosec // the path is a fixed name under the caller-provided cache directory
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}

// commitCache makes the slot the valid generation by atomically replacing the
// pointer. This is the only point that changes which generation is valid, so a
// failure before it leaves the existing cache untouched.
func commitCache(dir, id, slot string) error {
	currentTmp := pointerTmpPath(dir, id)
	if err := removeStalePointerTmp(currentTmp); err != nil {
		return err
	}
	current := pointerPath(dir, id)
	if err := ensurePointerReplaceable(current); err != nil {
		return err
	}
	file, err := os.OpenFile(currentTmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // the path is a fixed name under the caller-provided cache directory
	if err != nil {
		return err
	}
	if _, err := file.WriteString(slot); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(currentTmp, current); err != nil {
		return err
	}
	// The rename itself is atomic; syncing the directory afterwards is a
	// best-effort durability improvement.
	_ = syncDir(dir)
	return nil
}

// removeStalePointerTmp removes a leftover temporary file so the exclusive
// create in commitCache can succeed. A non-regular entry is left untouched.
func removeStalePointerTmp(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", errPointerNotRegular, path)
	}
	return os.Remove(path)
}

// ensurePointerReplaceable refuses to rename over an entry that is not a
// regular file, so a symlink at the pointer name is never replaced.
func ensurePointerReplaceable(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", errPointerNotRegular, path)
	}
	return nil
}

// cacheEntry is one fixed-name cache entry and the type it must have to be
// touched.
type cacheEntry struct {
	path    string
	wantDir bool
}

// danglingEntries returns the entries the pointer state leaves unreferenced, in
// the order they are removed. A pointer of another type is never listed.
func danglingEntries(dir, id string, state pointerState) []cacheEntry {
	slotA := cacheEntry{path: slotDirPath(dir, id, slotNameA), wantDir: true}
	slotB := cacheEntry{path: slotDirPath(dir, id, slotNameB), wantDir: true}
	tmp := cacheEntry{path: pointerTmpPath(dir, id)}
	switch state {
	case pointerSlotA:
		return []cacheEntry{slotB, tmp}
	case pointerSlotB:
		return []cacheEntry{slotA, tmp}
	case pointerInvalid:
		return []cacheEntry{{path: pointerPath(dir, id)}, slotA, slotB, tmp}
	default:
		return []cacheEntry{slotA, slotB, tmp}
	}
}

// removeCacheEntry removes one entry if it exists with the expected type. A
// missing entry or an entry of another type is not an error: the fixed-name
// rule says not to touch the latter.
func removeCacheEntry(entry cacheEntry) error {
	info, err := os.Lstat(entry.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if entry.wantDir {
		if !info.IsDir() {
			return nil
		}
		return os.RemoveAll(entry.path)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	return os.Remove(entry.path)
}

// removeDangling removes the entries the pointer state leaves unreferenced and
// joins the deletion failures. It stops before the next entry once ctx ends.
func removeDangling(ctx context.Context, dir, id string, state pointerState) error {
	var failures []error
	for _, entry := range danglingEntries(dir, id, state) {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		if err := removeCacheEntry(entry); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// pruneCandidateID returns the video ID when name is exactly
// <video ID>.<a|b|current|current.tmp> and the ID part is a valid video ID.
func pruneCandidateID(name string) (string, bool) {
	id, suffix, ok := strings.Cut(name, ".")
	if !ok || !videoIDPattern.MatchString(id) {
		return "", false
	}
	switch suffix {
	case slotNameA, slotNameB, currentSuffix, tmpSuffix:
		return id, true
	default:
		return "", false
	}
}
