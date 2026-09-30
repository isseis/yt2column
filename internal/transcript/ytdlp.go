package transcript

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

// Options configures a YtDlpSource. CacheDir and Timeout are required.
type Options struct {
	CacheDir     string
	YtDlpPath    string // empty means "yt-dlp" on PATH
	Timeout      time.Duration
	ForceRefresh bool
}

// errInvalidOptions reports a missing or non-positive required option.
var errInvalidOptions = errors.New("invalid YtDlpSource options")

// YtDlpSource implements TranscriptSource by invoking yt-dlp.
type YtDlpSource struct {
	options Options
	exec    commandExecutor
}

var _ TranscriptSource = (*YtDlpSource)(nil)

// NewYtDlpSource validates opts and returns a source that runs the real
// yt-dlp. An empty YtDlpPath means "yt-dlp" on PATH.
func NewYtDlpSource(opts Options) (*YtDlpSource, error) {
	if opts.CacheDir == "" {
		return nil, fmt.Errorf("%w: CacheDir must not be empty", errInvalidOptions)
	}
	if opts.Timeout <= 0 {
		return nil, fmt.Errorf("%w: Timeout must be positive", errInvalidOptions)
	}
	if opts.YtDlpPath == "" {
		opts.YtDlpPath = "yt-dlp"
	}
	return &YtDlpSource{options: opts, exec: osExecutor{}}, nil
}

// Fetch implements TranscriptSource. It serves a complete cache without
// running yt-dlp and runs yt-dlp only for a cache miss or a forced refresh. A
// run changes the cache only when it produces a usable transcript; after any
// outcome Fetch removes the video's dangling entries.
func (s *YtDlpSource) Fetch(ctx context.Context, videoURL string) (Transcript, error) {
	if err := ctx.Err(); err != nil {
		return Transcript{}, err
	}
	id, normalizedURL, err := validateVideoURL(videoURL)
	if err != nil {
		return Transcript{}, err
	}
	defer func() {
		// The cleanup is local and must not be skipped just because the
		// caller's context ended.
		s.cleanDangling(context.WithoutCancel(ctx), id)
	}()

	if !s.options.ForceRefresh {
		transcript, hit, err := s.readCached(id, normalizedURL)
		if err != nil {
			return Transcript{}, err
		}
		if hit {
			return transcript, nil
		}
	}
	return s.runYtDlp(ctx, id, normalizedURL)
}

// RemoveCache removes every cache entry of the video, valid or dangling.
// The caller must not run Fetch for the same video concurrently.
func (s *YtDlpSource) RemoveCache(ctx context.Context, videoURL string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, _, err := validateVideoURL(videoURL)
	if err != nil {
		return err
	}

	dir := s.options.CacheDir
	state, err := readPointer(dir, id)
	if err != nil {
		return err
	}
	if state == pointerSlotA || state == pointerSlotB || state == pointerInvalid {
		// Delete the pointer first: from here on the video is a cache miss,
		// and an interruption cannot leave a mixed generation behind. A
		// failure here leaves the valid cache untouched.
		if err := removeCacheEntry(cacheEntry{path: pointerPath(dir, id)}); err != nil {
			return err
		}
	}
	return removeDangling(ctx, dir, id, pointerMissing)
}

// PruneCache removes dangling cache entries of every video in CacheDir.
// The caller must not run Fetch or another PruneCache on the same CacheDir
// concurrently.
func (s *YtDlpSource) PruneCache(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := s.options.CacheDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	ids := make(map[string]struct{})
	for _, entry := range entries {
		if id, ok := pruneCandidateID(entry.Name()); ok {
			ids[id] = struct{}{}
		}
	}

	var failures []error
	for id := range ids {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		state, err := readPointer(dir, id)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if err := removeDangling(ctx, dir, id, state); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// readCached returns the transcript of a complete cache hit. hit is false when
// the pair is incomplete or a file disappeared while reading, which turns the
// call into a cache miss.
func (s *YtDlpSource) readCached(id, normalizedURL string) (Transcript, bool, error) {
	dir := s.options.CacheDir
	state, err := readPointer(dir, id)
	if err != nil {
		return Transcript{}, false, err
	}
	slot, ok := state.validSlot()
	if !ok {
		return Transcript{}, false, nil
	}

	slotDir := slotDirPath(dir, id, slot)
	if info, err := os.Lstat(slotDir); err != nil || !info.IsDir() {
		// A missing or non-directory slot, including a symlink, is not a
		// valid generation and is never followed.
		return Transcript{}, false, nil
	}
	subtitleFile := subtitlesPath(slotDir, id)
	infoFile := infoPath(slotDir, id)
	if !cacheFileExists(subtitleFile) || !cacheFileExists(infoFile) {
		return Transcript{}, false, nil
	}

	segments, err := readSubtitlesFile(subtitleFile)
	if errors.Is(err, errFileMissing) {
		return Transcript{}, false, nil
	}
	if err != nil {
		return Transcript{}, false, err
	}
	if len(segments) == 0 {
		return Transcript{}, false, fmt.Errorf("%w: %s: cached subtitles contain no text segments", ErrNoSubtitles, id)
	}

	info, err := readInfoFile(infoFile, id)
	if errors.Is(err, errFileMissing) {
		return Transcript{}, false, nil
	}
	if err != nil {
		return Transcript{}, false, err
	}
	return newTranscript(id, normalizedURL, info, segments), true, nil
}

// runYtDlp runs one yt-dlp into the slot that is not currently valid, verifies
// the output, and commits it. A failure leaves the existing cache untouched.
func (s *YtDlpSource) runYtDlp(ctx context.Context, id, normalizedURL string) (Transcript, error) {
	dir := s.options.CacheDir
	state, err := readPointer(dir, id)
	if err != nil {
		return Transcript{}, err
	}
	slot := writeSlot(state)
	slotDir, err := prepareWriteSlot(dir, id, slot)
	if err != nil {
		return Transcript{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, s.options.Timeout)
	defer cancel()
	stderr := &cappedWriter{}
	env := allowlistEnv(os.Environ())
	if err := s.exec.Run(runCtx, s.options.YtDlpPath, ytDlpArgs(slotDir, normalizedURL), env, stderr); err != nil {
		// Run returns raw results; a timeout or a cancellation is recognized
		// through the context that was used for the run.
		if ctxErr := runCtx.Err(); ctxErr != nil {
			return Transcript{}, ctxErr
		}
		message := fmt.Sprintf("%s: %s failed: %v", id, s.options.YtDlpPath, err)
		if captured := redactStderr(stderr.String(), env); captured != "" {
			message += ": " + captured
		}
		return Transcript{}, fmt.Errorf("%w: %s", ErrYtDlpExec, message)
	}

	segments, info, err := verifyRunOutput(slotDir, id)
	if err != nil {
		return Transcript{}, err
	}
	if err := persistSlot(slotDir, id); err != nil {
		return Transcript{}, err
	}
	if err := commitCache(dir, id, slot); err != nil {
		return Transcript{}, err
	}
	return newTranscript(id, normalizedURL, info, segments), nil
}

// verifyRunOutput validates the files yt-dlp wrote into the write slot and
// returns the segments and metadata. The subtitle file is checked first, so a
// composite failure reports the subtitle sentinel.
func verifyRunOutput(slotDir, id string) ([]Segment, videoInfo, error) {
	segments, err := readSubtitlesFile(subtitlesPath(slotDir, id))
	if errors.Is(err, errFileMissing) {
		return nil, videoInfo{}, fmt.Errorf("%w: %s: no subtitle file was produced", ErrNoSubtitles, id)
	}
	if err != nil {
		return nil, videoInfo{}, err
	}
	if len(segments) == 0 {
		return nil, videoInfo{}, fmt.Errorf("%w: %s: subtitle file contains no text segments", ErrNoSubtitles, id)
	}

	info, err := readInfoFile(infoPath(slotDir, id), id)
	if errors.Is(err, errFileMissing) {
		return nil, videoInfo{}, fmt.Errorf("%w: %s: no info.json was produced", ErrParseInfo, id)
	}
	if err != nil {
		return nil, videoInfo{}, err
	}
	return segments, info, nil
}

// cleanDangling removes the dangling entries of one video. It is best effort:
// a failure leaves the entries to the next Fetch or prune.
func (s *YtDlpSource) cleanDangling(ctx context.Context, id string) {
	state, err := readPointer(s.options.CacheDir, id)
	if err != nil {
		return
	}
	_ = removeDangling(ctx, s.options.CacheDir, id, state)
}

func newTranscript(id, normalizedURL string, info videoInfo, segments []Segment) Transcript {
	return Transcript{
		VideoID:     id,
		VideoURL:    normalizedURL,
		Title:       info.Title,
		ChannelName: info.ChannelName,
		Description: info.Description,
		Segments:    segments,
	}
}

// ytDlpArgs is the fixed argument array of one yt-dlp run. The normalized URL
// is the last argument, after "--", so it is never interpreted as an option.
func ytDlpArgs(slotDir, normalizedURL string) []string {
	return []string{
		"--ignore-config",
		"--no-plugin-dirs",
		"--skip-download",
		"--write-subs",
		"--write-auto-subs",
		"--sub-langs", "ja",
		"--sub-format", "json3",
		"--write-info-json",
		"-P", slotDir,
		"-o", "%(id)s",
		"--", normalizedURL,
	}
}
