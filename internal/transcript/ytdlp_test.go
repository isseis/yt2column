//go:build test

package transcript

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func watchURL(id string) string { return "https://www.youtube.com/watch?v=" + id }
func shortURL(id string) string { return "https://youtu.be/" + id }

// subtitleDocument builds a one-segment json3 document with the given text.
func subtitleDocument(text string) string {
	return `{"events":[{"tStartMs":0,"segs":[{"utf8":"` + text + `"}]}]}`
}

// infoDocument builds a minimal valid info.json for id with the given label.
func infoDocument(id, label string) string {
	return `{"id":"` + id + `","title":"title-` + label + `","channel":"channel-` + label + `","description":"description-` + label + `"}`
}

// generationFor builds a distinguishable generation labeled label.
func generationFor(id, label string) generation {
	return generation{subtitles: subtitleDocument("text-" + label), info: infoDocument(id, label)}
}

// writeGeneration writes gen into the slot the recorded -P argument names.
func writeGeneration(t *testing.T, id string, gen generation) func(fakeCommandCall) error {
	return func(call fakeCommandCall) error {
		writeSlotFiles(t, slotDirFromArgs(t, call.args), id, gen.subtitles, gen.info)
		return nil
	}
}

// failRun reports a fixed failure from the fake run.
func failRun(err error) func(fakeCommandCall) error {
	return func(fakeCommandCall) error { return err }
}

// runGeneration builds the generation a fake run writes.
func runGeneration(id string) generation { return generationFor(id, "run") }

// assertSingleGeneration checks that the metadata and the first segment come
// from the same generation, so a transcript mixing two generations fails.
func assertSingleGeneration(t *testing.T, transcript Transcript) {
	t.Helper()
	titleLabel := strings.TrimPrefix(transcript.Title, "title-")
	if titleLabel == transcript.Title {
		t.Fatalf("title %q has no generation label", transcript.Title)
	}
	if len(transcript.Segments) == 0 {
		t.Fatal("transcript has no segments")
	}
	textLabel := strings.TrimPrefix(transcript.Segments[0].Text, "text-")
	if textLabel != titleLabel {
		t.Errorf("transcript mixes generations: title %q, text %q", transcript.Title, transcript.Segments[0].Text)
	}
}

// assertNoDangling checks that only the pointer and the slot it names remain
// among the video's fixed names. Entries outside the rule, such as
// <id>.notes, are ignored.
func assertNoDangling(t *testing.T, dir, id string) {
	t.Helper()
	allowed := map[string]bool{}
	if pointer, err := os.ReadFile(pointerPath(dir, id)); err == nil {
		if slot := string(pointer); slot == slotNameA || slot == slotNameB {
			allowed[pointerPath(dir, id)] = true
			allowed[slotDirPath(dir, id, slot)] = true
		}
	}
	for _, path := range []string{
		pointerPath(dir, id),
		slotDirPath(dir, id, slotNameA),
		slotDirPath(dir, id, slotNameB),
		pointerTmpPath(dir, id),
	} {
		if allowed[path] {
			continue
		}
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s is a dangling entry (error = %v)", path, err)
		}
	}
}

// envMap converts an environment slice into a name-to-value map.
func envMap(env []string) map[string]string {
	result := make(map[string]string, len(env))
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		result[name] = value
	}
	return result
}

func TestNewYtDlpSource(t *testing.T) {
	t.Run("valid options", func(t *testing.T) {
		source, err := NewYtDlpSource(Options{CacheDir: t.TempDir(), Timeout: time.Minute})
		if err != nil {
			t.Fatalf("NewYtDlpSource error = %v", err)
		}
		if source.options.YtDlpPath != "yt-dlp" {
			t.Errorf("YtDlpPath = %q, want the PATH default", source.options.YtDlpPath)
		}
		if _, ok := source.exec.(osExecutor); !ok {
			t.Errorf("executor = %T, want osExecutor", source.exec)
		}
	})

	t.Run("empty cache dir", func(t *testing.T) {
		source, err := NewYtDlpSource(Options{Timeout: time.Minute})
		if err == nil || source != nil {
			t.Fatalf("NewYtDlpSource = (%v, %v), want an error", source, err)
		}
		if !errors.Is(err, errInvalidOptions) {
			t.Errorf("error = %v, want errInvalidOptions", err)
		}
	})

	t.Run("zero timeout", func(t *testing.T) {
		if _, err := NewYtDlpSource(Options{CacheDir: t.TempDir()}); !errors.Is(err, errInvalidOptions) {
			t.Fatalf("NewYtDlpSource error = %v, want errInvalidOptions", err)
		}
	})

	t.Run("negative timeout", func(t *testing.T) {
		if _, err := NewYtDlpSource(Options{CacheDir: t.TempDir(), Timeout: -time.Second}); !errors.Is(err, errInvalidOptions) {
			t.Fatalf("NewYtDlpSource error = %v, want errInvalidOptions", err)
		}
	})
}

func TestFetchCacheHit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	subtitles, info := placeRealCache(t, dir, id)

	fake := &fakeCommandExecutor{behavior: failRun(errors.New("yt-dlp must not run on a cache hit"))}
	source := newTestSource(t, dir, fake, nil)

	transcript, err := source.Fetch(t.Context(), watchURL(id))
	if err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if fake.callCount() != 0 {
		t.Fatalf("yt-dlp ran %d times, want 0", fake.callCount())
	}
	if transcript.VideoID != id || transcript.VideoURL != watchURL(id) {
		t.Errorf("identity = %q / %q, want %q / %q", transcript.VideoID, transcript.VideoURL, id, watchURL(id))
	}
	wantInfo := infoOracle(t, info)
	if wantInfo.Title == wantInfo.ChannelName || wantInfo.Title == wantInfo.Description || wantInfo.ChannelName == wantInfo.Description {
		t.Fatalf("fixture metadata values are not distinct: %+v", wantInfo)
	}
	if transcript.Title != wantInfo.Title || transcript.ChannelName != wantInfo.ChannelName || transcript.Description != wantInfo.Description {
		t.Errorf("metadata = %q / %q / %q, want %q / %q / %q",
			transcript.Title, transcript.ChannelName, transcript.Description,
			wantInfo.Title, wantInfo.ChannelName, wantInfo.Description)
	}
	if wantSegments := subtitlesOracle(t, subtitles); !reflect.DeepEqual(transcript.Segments, wantSegments) {
		t.Errorf("segments = %d, want %d from the fixture", len(transcript.Segments), len(wantSegments))
	}

	// The short URL form maps to the same video and the same normalized URL.
	short, err := source.Fetch(t.Context(), shortURL(id))
	if err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if short.VideoID != id || short.VideoURL != watchURL(id) {
		t.Errorf("short URL identity = %q / %q, want %q / %q", short.VideoID, short.VideoURL, id, watchURL(id))
	}
	if fake.callCount() != 0 {
		t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
	}
}

func TestFetchCacheMiss(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	subtitles := readTestdataFile(t, testdataRealSubtitles)
	info := readTestdataFile(t, testdataRealInfo)

	fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, generation{subtitles: string(subtitles), info: string(info)})}
	source := newTestSource(t, dir, fake, nil)

	transcript, err := source.Fetch(t.Context(), watchURL(id))
	if err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if fake.callCount() != 1 {
		t.Fatalf("yt-dlp ran %d times, want 1", fake.callCount())
	}
	if transcript.VideoID != id || transcript.VideoURL != watchURL(id) {
		t.Errorf("identity = %q / %q, want %q / %q", transcript.VideoID, transcript.VideoURL, id, watchURL(id))
	}
	wantInfo := infoOracle(t, info)
	if transcript.Title != wantInfo.Title || transcript.ChannelName != wantInfo.ChannelName || transcript.Description != wantInfo.Description {
		t.Errorf("metadata = %q / %q / %q, want %q / %q / %q",
			transcript.Title, transcript.ChannelName, transcript.Description,
			wantInfo.Title, wantInfo.ChannelName, wantInfo.Description)
	}
	if wantSegments := subtitlesOracle(t, subtitles); !reflect.DeepEqual(transcript.Segments, wantSegments) {
		t.Errorf("segments = %d, want %d from the fixture", len(transcript.Segments), len(wantSegments))
	}

	// The miss committed the cache: the second Fetch is a hit.
	slotDir := slotDirPath(dir, id, slotNameA)
	assertFileContent(t, pointerPath(dir, id), slotNameA)
	assertFileContent(t, subtitlesPath(slotDir, id), string(subtitles))
	assertFileContent(t, infoPath(slotDir, id), string(info))
	again, err := source.Fetch(t.Context(), watchURL(id))
	if err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if fake.callCount() != 1 {
		t.Errorf("yt-dlp ran %d times, want 1", fake.callCount())
	}
	if !reflect.DeepEqual(again, transcript) {
		t.Errorf("second transcript differs from the first")
	}
}

func TestFetchYtDlpArgs(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache-%(title)s-%%")
	id := testdataRealVideoID
	fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
	source := newTestSource(t, cacheDir, fake, nil)

	if _, err := source.Fetch(t.Context(), watchURL(id)); err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	call := fake.lastCall(t)
	if call.name != source.options.YtDlpPath {
		t.Errorf("executable = %q, want %q", call.name, source.options.YtDlpPath)
	}
	slotDir := slotDirPath(cacheDir, id, slotNameA)
	want := []string{
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
		"--", watchURL(id),
	}
	if !reflect.DeepEqual(call.args, want) {
		t.Errorf("args = %q, want %q", call.args, want)
	}
}

func TestFetchPassesInheritedFiles(t *testing.T) {
	id := testdataRealVideoID
	first, err := os.CreateTemp(t.TempDir(), "inherited-*")
	if err != nil {
		t.Fatalf("create inherited file: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := os.CreateTemp(t.TempDir(), "inherited-*")
	if err != nil {
		t.Fatalf("create inherited file: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
	source := newTestSource(t, filepath.Join(t.TempDir(), "cache"), fake, func(o *Options) {
		o.InheritedFiles = []*os.File{first, second}
	})

	if _, err := source.Fetch(t.Context(), watchURL(id)); err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if got := fake.lastCall(t).files; !slices.Equal(got, []*os.File{first, second}) {
		t.Errorf("inherited files = %v, want the Options files in order", got)
	}
}

func TestFetchYtDlpFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	fake := &fakeCommandExecutor{behavior: failRun(errors.New("exit status 3"))}
	source := newTestSource(t, dir, fake, nil)

	_, err := source.Fetch(t.Context(), watchURL(id))
	if !errors.Is(err, ErrYtDlpExec) {
		t.Fatalf("Fetch error = %v, want ErrYtDlpExec", err)
	}
	if !strings.Contains(err.Error(), source.options.YtDlpPath) {
		t.Errorf("error %q does not name the executable %q", err, source.options.YtDlpPath)
	}
	assertNoDangling(t, dir, id)
}

func TestFetchTimeoutAndCancel(t *testing.T) {
	id := testdataRealVideoID

	t.Run("canceled before the run", func(t *testing.T) {
		fake := &fakeCommandExecutor{}
		source := newTestSource(t, filepath.Join(t.TempDir(), "cache"), fake, nil)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := source.Fetch(ctx, watchURL(id))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Fetch error = %v, want context.Canceled", err)
		}
		if fake.callCount() != 0 {
			t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
		}
	})

	t.Run("timeout while running", func(t *testing.T) {
		fake := &fakeCommandExecutor{behavior: func(call fakeCommandCall) error {
			<-call.ctx.Done()
			return errors.New("signal: killed")
		}}
		source := newTestSource(t, filepath.Join(t.TempDir(), "cache"), fake, func(options *Options) {
			options.Timeout = 50 * time.Millisecond
		})
		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Fetch error = %v, want context.DeadlineExceeded", err)
		}
		if errors.Is(err, ErrYtDlpExec) {
			t.Errorf("timeout error also matches ErrYtDlpExec")
		}
	})

	t.Run("canceled while running", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		fake := &fakeCommandExecutor{behavior: func(call fakeCommandCall) error {
			cancel()
			<-call.ctx.Done()
			return errors.New("signal: killed")
		}}
		source := newTestSource(t, filepath.Join(t.TempDir(), "cache"), fake, nil)
		_, err := source.Fetch(ctx, watchURL(id))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Fetch error = %v, want context.Canceled", err)
		}
		if errors.Is(err, ErrYtDlpExec) {
			t.Errorf("cancellation error also matches ErrYtDlpExec")
		}
	})
}

func TestFetchNoSubtitles(t *testing.T) {
	id := testdataRealVideoID

	cases := map[string]func(fakeCommandCall) error{
		"no output at all": nil,
		"info.json only": func(call fakeCommandCall) error {
			writeTestFile(t, infoPath(slotDirFromArgs(t, call.args), id), infoDocument(id, "run"))
			return nil
		},
		"zero segments": func(call fakeCommandCall) error {
			writeSlotFiles(t, slotDirFromArgs(t, call.args), id, `{"events":[]}`, infoDocument(id, "run"))
			return nil
		},
	}
	for name, behavior := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "cache")
			fake := &fakeCommandExecutor{behavior: behavior}
			source := newTestSource(t, dir, fake, nil)

			_, err := source.Fetch(t.Context(), watchURL(id))
			if !errors.Is(err, ErrNoSubtitles) {
				t.Fatalf("Fetch error = %v, want ErrNoSubtitles", err)
			}
			if errors.Is(err, ErrParseInfo) {
				t.Errorf("error also matches ErrParseInfo")
			}
			if !strings.Contains(err.Error(), id) {
				t.Errorf("error %q does not name the video %q", err, id)
			}
			if _, err := os.Lstat(pointerPath(dir, id)); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("a failed run committed a cache entry (error = %v)", err)
			}
		})
	}
}

func TestFetchInfoMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	fake := &fakeCommandExecutor{behavior: func(call fakeCommandCall) error {
		writeTestFile(t, subtitlesPath(slotDirFromArgs(t, call.args), id), subtitleDocument("text"))
		return nil
	}}
	source := newTestSource(t, dir, fake, nil)

	_, err := source.Fetch(t.Context(), watchURL(id))
	if !errors.Is(err, ErrParseInfo) {
		t.Fatalf("Fetch error = %v, want ErrParseInfo", err)
	}
	if errors.Is(err, ErrNoSubtitles) {
		t.Errorf("missing info.json also matches ErrNoSubtitles")
	}
	parseErr, ok := errors.AsType[*ParseError](err)
	if !ok {
		t.Fatalf("Fetch error = %v, want *ParseError", err)
	}
	if want := infoPath(slotDirPath(dir, id, slotNameA), id); parseErr.Path != want {
		t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, want)
	}
	if _, err := os.Lstat(pointerPath(dir, id)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a failed run committed a cache entry (error = %v)", err)
	}
}

func TestFetchValidationOrder(t *testing.T) {
	id := testdataRealVideoID
	invalidInfo := `{"id":"` + id + `","title":`

	t.Run("run output", func(t *testing.T) {
		cases := []struct {
			name      string
			subtitles string
			info      string
			writeInfo bool
			want      error
		}{
			{"zero segments and invalid info", `{"events":[]}`, invalidInfo, true, ErrNoSubtitles},
			{"invalid subtitles and invalid info", `{"events":`, invalidInfo, true, ErrParseSubtitles},
			{"missing subtitles and invalid info", "", invalidInfo, true, ErrNoSubtitles},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "cache")
				fake := &fakeCommandExecutor{behavior: func(call fakeCommandCall) error {
					slotDir := slotDirFromArgs(t, call.args)
					if tc.subtitles != "" {
						writeTestFile(t, subtitlesPath(slotDir, id), tc.subtitles)
					}
					if tc.writeInfo {
						writeTestFile(t, infoPath(slotDir, id), tc.info)
					}
					return nil
				}}
				source := newTestSource(t, dir, fake, nil)
				_, err := source.Fetch(t.Context(), watchURL(id))
				if !errors.Is(err, tc.want) {
					t.Fatalf("Fetch error = %v, want %v", err, tc.want)
				}
				if errors.Is(err, ErrParseInfo) {
					t.Errorf("error %v also matches ErrParseInfo", err)
				}
			})
		}
	})

	t.Run("cache read", func(t *testing.T) {
		cases := []struct {
			name      string
			subtitles string
			info      string
			want      error
		}{
			{"zero segments and invalid info", `{"events":[]}`, invalidInfo, ErrNoSubtitles},
			{"invalid subtitles and invalid info", `{"events":`, invalidInfo, ErrParseSubtitles},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "cache")
				slotDir := slotDirPath(dir, id, slotNameA)
				writeSlotFiles(t, slotDir, id, tc.subtitles, tc.info)
				placePointer(t, dir, id, slotNameA)
				fake := &fakeCommandExecutor{}
				source := newTestSource(t, dir, fake, nil)

				_, err := source.Fetch(t.Context(), watchURL(id))
				if !errors.Is(err, tc.want) {
					t.Fatalf("Fetch error = %v, want %v", err, tc.want)
				}
				if errors.Is(err, ErrParseInfo) {
					t.Errorf("error %v also matches ErrParseInfo", err)
				}
				if fake.callCount() != 0 {
					t.Errorf("yt-dlp ran %d times, want 0 on a complete cache", fake.callCount())
				}
			})
		}
	})
}

func TestFetchParseErrorPath(t *testing.T) {
	id := testdataRealVideoID

	cases := []struct {
		name     string
		behavior func(call fakeCommandCall) error
		file     func(slotDir string) string
		sentinel error
	}{
		{
			name: "invalid subtitles",
			behavior: func(call fakeCommandCall) error {
				writeSlotFiles(t, slotDirFromArgs(t, call.args), id, `{"events":`, infoDocument(id, "run"))
				return nil
			},
			file:     func(slotDir string) string { return subtitlesPath(slotDir, id) },
			sentinel: ErrParseSubtitles,
		},
		{
			name: "invalid info",
			behavior: func(call fakeCommandCall) error {
				writeSlotFiles(t, slotDirFromArgs(t, call.args), id, subtitleDocument("text"), `{"id":"`+id+`","title":`)
				return nil
			},
			file:     func(slotDir string) string { return infoPath(slotDir, id) },
			sentinel: ErrParseInfo,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "cache")
			fake := &fakeCommandExecutor{behavior: tc.behavior}
			source := newTestSource(t, dir, fake, nil)

			_, err := source.Fetch(t.Context(), watchURL(id))
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("Fetch error = %v, want %v", err, tc.sentinel)
			}
			parseErr, ok := errors.AsType[*ParseError](err)
			if !ok {
				t.Fatalf("Fetch error = %v, want *ParseError", err)
			}
			wantPath := tc.file(slotDirPath(dir, id, slotNameA))
			if parseErr.Path != wantPath {
				t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, wantPath)
			}
		})
	}
}

func TestFetchCacheParseError(t *testing.T) {
	id := testdataRealVideoID

	cases := []struct {
		name      string
		subtitles string
		info      string
		file      func(dir string) string
		sentinel  error
	}{
		{
			name:      "invalid cached subtitles",
			subtitles: `{"events":`,
			info:      infoDocument(id, "old"),
			file:      func(dir string) string { return subtitlesPath(slotDirPath(dir, id, slotNameA), id) },
			sentinel:  ErrParseSubtitles,
		},
		{
			name:      "invalid cached info",
			subtitles: subtitleDocument("old"),
			info:      `{"id":"` + id + `","title":`,
			file:      func(dir string) string { return infoPath(slotDirPath(dir, id, slotNameA), id) },
			sentinel:  ErrParseInfo,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "cache")
			placeSlot(t, dir, id, slotNameA, generation{subtitles: tc.subtitles, info: tc.info})
			placePointer(t, dir, id, slotNameA)
			fake := &fakeCommandExecutor{}
			source := newTestSource(t, dir, fake, nil)

			_, err := source.Fetch(t.Context(), watchURL(id))
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("Fetch error = %v, want %v", err, tc.sentinel)
			}
			parseErr, ok := errors.AsType[*ParseError](err)
			if !ok {
				t.Fatalf("Fetch error = %v, want *ParseError", err)
			}
			if wantPath := tc.file(dir); parseErr.Path != wantPath {
				t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, wantPath)
			}
			if fake.callCount() != 0 {
				t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
			}
			// The damaged cache is not replaced by the failed read.
			assertFileContent(t, pointerPath(dir, id), slotNameA)
			slotDir := slotDirPath(dir, id, slotNameA)
			assertFileContent(t, subtitlesPath(slotDir, id), tc.subtitles)
			assertFileContent(t, infoPath(slotDir, id), tc.info)
		})
	}
}

func TestFetchPartialCache(t *testing.T) {
	id := testdataRealVideoID
	cases := map[string]func(dir string){
		"pointer with subtitles only": func(dir string) {
			slotDir := slotDirPath(dir, id, slotNameA)
			writeTestFile(t, subtitlesPath(slotDir, id), subtitleDocument("partial"))
			placePointer(t, dir, id, slotNameA)
		},
		"pointer with info only": func(dir string) {
			slotDir := slotDirPath(dir, id, slotNameA)
			writeTestFile(t, infoPath(slotDir, id), infoDocument(id, "partial"))
			placePointer(t, dir, id, slotNameA)
		},
		"files without a pointer": func(dir string) {
			placeSlot(t, dir, id, slotNameA, generationFor(id, "partial"))
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "cache")
			build(dir)
			fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
			source := newTestSource(t, dir, fake, nil)

			transcript, err := source.Fetch(t.Context(), watchURL(id))
			if err != nil {
				t.Fatalf("Fetch error = %v", err)
			}
			if fake.callCount() != 1 {
				t.Fatalf("yt-dlp ran %d times, want 1", fake.callCount())
			}
			if transcript.Title != "title-run" {
				t.Errorf("title = %q, want the run generation, not the partial cache", transcript.Title)
			}
		})
	}
}

func TestFetchOversizedPointer(t *testing.T) {
	id := testdataRealVideoID
	stale := generationFor(id, "stale")

	t.Run("oversized pointer is a miss", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		placeSlot(t, dir, id, slotNameA, stale)
		placePointer(t, dir, id, "a"+strings.Repeat("b", 100))
		fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
		source := newTestSource(t, dir, fake, nil)

		transcript, err := source.Fetch(t.Context(), watchURL(id))
		if err != nil {
			t.Fatalf("Fetch error = %v", err)
		}
		if fake.callCount() != 1 {
			t.Fatalf("yt-dlp ran %d times, want 1: a pointer longer than one byte is not a hit", fake.callCount())
		}
		if transcript.Title != "title-run" {
			t.Errorf("title = %q, want the run generation", transcript.Title)
		}
		assertFileContent(t, pointerPath(dir, id), slotNameA)
		assertNoDangling(t, dir, id)
	})

	t.Run("oversized pointer is removed after a failure", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		placeSlot(t, dir, id, slotNameA, stale)
		placePointer(t, dir, id, "a"+strings.Repeat("b", 100))
		fake := &fakeCommandExecutor{behavior: failRun(errors.New("exit status 1"))}
		source := newTestSource(t, dir, fake, nil)

		if _, err := source.Fetch(t.Context(), watchURL(id)); !errors.Is(err, ErrYtDlpExec) {
			t.Fatalf("Fetch error = %v, want ErrYtDlpExec", err)
		}
		assertNoDangling(t, dir, id)
	})

	t.Run("prune removes the oversized pointer", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		placeSlot(t, dir, id, slotNameA, stale)
		placePointer(t, dir, id, "a"+strings.Repeat("b", 100))
		source := newTestSource(t, dir, &fakeCommandExecutor{}, nil)

		if err := source.PruneCache(t.Context()); err != nil {
			t.Fatalf("PruneCache error = %v", err)
		}
		assertNoDangling(t, dir, id)
	})
}

func TestFetchStaleFileNotMistaken(t *testing.T) {
	id := testdataRealVideoID
	stale := generationFor(id, "stale")
	cases := map[string]func(fakeCommandCall) error{
		"run writes nothing": nil,
		"run writes info only": func(call fakeCommandCall) error {
			writeTestFile(t, infoPath(slotDirFromArgs(t, call.args), id), infoDocument(id, "run"))
			return nil
		},
	}
	for name, behavior := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "cache")
			placeSlot(t, dir, id, slotNameA, stale)
			// No pointer: the cache is a miss, and the write slot holds the
			// leftovers of the interrupted run.
			fake := &fakeCommandExecutor{behavior: behavior}
			source := newTestSource(t, dir, fake, nil)

			_, err := source.Fetch(t.Context(), watchURL(id))
			if !errors.Is(err, ErrNoSubtitles) {
				t.Fatalf("Fetch error = %v, want ErrNoSubtitles", err)
			}
			assertNoDangling(t, dir, id)
		})
	}
}

func TestFetchSubtitleSelection(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	slotDir := slotDirPath(dir, id, slotNameA)
	writeSlotFiles(t, slotDir, id, subtitleDocument("selected"), infoDocument(id, "selected"))
	writeTestFile(t, filepath.Join(slotDir, id+".ja-orig.json3"), subtitleDocument("ignored"))
	placePointer(t, dir, id, slotNameA)

	fake := &fakeCommandExecutor{}
	source := newTestSource(t, dir, fake, nil)

	transcript, err := source.Fetch(t.Context(), watchURL(id))
	if err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if fake.callCount() != 0 {
		t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
	}
	if len(transcript.Segments) != 1 || transcript.Segments[0].Text != "selected" {
		t.Errorf("segments = %+v, want the ja.json3 content", transcript.Segments)
	}
	// The other subtitle file is not read, changed, or removed.
	assertFileContent(t, filepath.Join(slotDir, id+".ja-orig.json3"), subtitleDocument("ignored"))
}

func TestFetchForceRefresh(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	placeSlot(t, dir, id, slotNameA, generationFor(id, "old"))
	placePointer(t, dir, id, slotNameA)
	newGen := runGeneration(id)
	fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, newGen)}
	source := newTestSource(t, dir, fake, func(options *Options) { options.ForceRefresh = true })

	transcript, err := source.Fetch(t.Context(), watchURL(id))
	if err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if fake.callCount() != 1 {
		t.Fatalf("yt-dlp ran %d times, want 1", fake.callCount())
	}
	if transcript.Title != "title-run" {
		t.Errorf("title = %q, want the refreshed generation", transcript.Title)
	}

	// The new generation is the only one left: the pointer names slot b and
	// the old slot is gone.
	assertFileContent(t, pointerPath(dir, id), slotNameB)
	newSlot := slotDirPath(dir, id, slotNameB)
	assertFileContent(t, subtitlesPath(newSlot, id), newGen.subtitles)
	assertFileContent(t, infoPath(newSlot, id), newGen.info)
	if _, err := os.Lstat(slotDirPath(dir, id, slotNameA)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("old slot remains after the refresh (error = %v)", err)
	}
	assertNoDangling(t, dir, id)
}

// refreshFailureCase is one failure kind that a Fetch must survive without
// changing an existing valid cache.
type refreshFailureCase struct {
	name     string
	timeout  time.Duration
	canceled bool
	behavior func(t *testing.T, id string, cancel context.CancelFunc) func(fakeCommandCall) error
}

// refreshFailureCases returns every failure kind a forced refresh must
// survive: a non-zero exit, a timeout, a cancellation during the run, and a
// successful run that produces no usable pair.
func refreshFailureCases() []refreshFailureCase {
	return []refreshFailureCase{
		{
			name: "nonzero exit",
			behavior: func(*testing.T, string, context.CancelFunc) func(fakeCommandCall) error {
				return failRun(errors.New("exit status 1"))
			},
		},
		{
			name:    "timeout",
			timeout: 50 * time.Millisecond,
			behavior: func(*testing.T, string, context.CancelFunc) func(fakeCommandCall) error {
				return func(call fakeCommandCall) error {
					<-call.ctx.Done()
					return errors.New("signal: killed")
				}
			},
		},
		{
			name:     "canceled while running",
			canceled: true,
			behavior: func(_ *testing.T, _ string, cancel context.CancelFunc) func(fakeCommandCall) error {
				return func(call fakeCommandCall) error {
					cancel()
					<-call.ctx.Done()
					return errors.New("signal: killed")
				}
			},
		},
		{
			name: "no subtitle file",
			behavior: func(t *testing.T, id string, _ context.CancelFunc) func(fakeCommandCall) error {
				return func(call fakeCommandCall) error {
					writeTestFile(t, infoPath(slotDirFromArgs(t, call.args), id), infoDocument(id, "run"))
					return nil
				}
			},
		},
		{
			name: "invalid subtitles",
			behavior: func(t *testing.T, id string, _ context.CancelFunc) func(fakeCommandCall) error {
				return func(call fakeCommandCall) error {
					writeSlotFiles(t, slotDirFromArgs(t, call.args), id, `{"events":`, infoDocument(id, "run"))
					return nil
				}
			},
		},
		{
			name: "invalid info",
			behavior: func(t *testing.T, id string, _ context.CancelFunc) func(fakeCommandCall) error {
				return func(call fakeCommandCall) error {
					writeSlotFiles(t, slotDirFromArgs(t, call.args), id, subtitleDocument("text"), `{"id":"`+id+`","title":`)
					return nil
				}
			},
		},
		{
			name: "id mismatch",
			behavior: func(t *testing.T, id string, _ context.CancelFunc) func(fakeCommandCall) error {
				return func(call fakeCommandCall) error {
					writeSlotFiles(t, slotDirFromArgs(t, call.args), id, subtitleDocument("text"), infoDocument("differenth", "run"))
					return nil
				}
			},
		},
		{
			name: "no info file",
			behavior: func(t *testing.T, id string, _ context.CancelFunc) func(fakeCommandCall) error {
				return func(call fakeCommandCall) error {
					writeTestFile(t, subtitlesPath(slotDirFromArgs(t, call.args), id), subtitleDocument("text"))
					return nil
				}
			},
		},
	}
}

func TestFetchForceRefreshFailureKeepsCache(t *testing.T) {
	id := testdataRealVideoID

	assertCacheKept := func(t *testing.T, source *YtDlpSource, ctx context.Context, dir string, oldSubtitles, oldInfo []byte) {
		t.Helper()
		if _, err := source.Fetch(ctx, watchURL(id)); err == nil {
			t.Fatal("Fetch error = nil, want a failure")
		}
		slotA := slotDirPath(dir, id, slotNameA)
		assertFileContent(t, pointerPath(dir, id), slotNameA)
		assertFileContent(t, subtitlesPath(slotA, id), string(oldSubtitles))
		assertFileContent(t, infoPath(slotA, id), string(oldInfo))
		if _, err := os.Lstat(slotDirPath(dir, id, slotNameB)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("write slot remains after the failed refresh (error = %v)", err)
		}
		assertFileContent(t, filepath.Join(dir, id+".notes"), "notes")
		assertNoDangling(t, dir, id)
	}

	for _, tc := range refreshFailureCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "cache")
			oldSubtitles, oldInfo := placeRealCache(t, dir, id)
			writeTestFile(t, filepath.Join(dir, id+".notes"), "notes")

			ctx := t.Context()
			cancel := context.CancelFunc(func() {})
			if tc.canceled {
				ctx, cancel = context.WithCancel(ctx)
			}
			defer cancel()
			fake := &fakeCommandExecutor{behavior: tc.behavior(t, id, cancel)}
			source := newTestSource(t, dir, fake, func(options *Options) {
				options.ForceRefresh = true
				if tc.timeout != 0 {
					options.Timeout = tc.timeout
				}
			})
			assertCacheKept(t, source, ctx, dir, oldSubtitles, oldInfo)
		})
	}
}

func TestFetchUnrelatedEntriesUntouched(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	placeRealCache(t, dir, id)
	otherID := testVideoID(t, "abcdefghijk")
	placeSlot(t, dir, otherID, slotNameA, generationFor(otherID, "other"))
	placePointer(t, dir, otherID, slotNameA)

	unrelated := map[string]string{
		id + ".notes":         "notes",
		"random.txt":          "random",
		id + ".ja-orig.json3": "orig",
	}
	for name, content := range unrelated {
		writeTestFile(t, filepath.Join(dir, name), content)
	}

	t.Run("successful refresh", func(t *testing.T) {
		fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
		source := newTestSource(t, dir, fake, func(options *Options) { options.ForceRefresh = true })
		if _, err := source.Fetch(t.Context(), watchURL(id)); err != nil {
			t.Fatalf("Fetch error = %v", err)
		}
		assertUnrelatedUntouched(t, dir, id, unrelated, otherID)
	})

	t.Run("failed refresh", func(t *testing.T) {
		fake := &fakeCommandExecutor{behavior: failRun(errors.New("exit status 1"))}
		source := newTestSource(t, dir, fake, func(options *Options) { options.ForceRefresh = true })
		if _, err := source.Fetch(t.Context(), watchURL(id)); !errors.Is(err, ErrYtDlpExec) {
			t.Fatalf("Fetch error = %v, want ErrYtDlpExec", err)
		}
		assertUnrelatedUntouched(t, dir, id, unrelated, otherID)
	})
}

func assertUnrelatedUntouched(t *testing.T, dir, id string, unrelated map[string]string, otherID string) {
	t.Helper()
	if _, err := os.Lstat(pointerTmpPath(dir, id)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("temporary pointer file for the fetched video remains (error = %v)", err)
	}
	assertFileContent(t, pointerPath(dir, otherID), slotNameA)
	for name, content := range unrelated {
		assertFileContent(t, filepath.Join(dir, name), content)
	}
}

func TestFetchEntryTypeMismatch(t *testing.T) {
	id := testdataRealVideoID

	t.Run("slot name is a regular file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		slotPath := slotDirPath(dir, id, slotNameA)
		writeTestFile(t, slotPath, "not a directory")
		fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
		source := newTestSource(t, dir, fake, nil)

		if _, err := source.Fetch(t.Context(), watchURL(id)); err == nil {
			t.Fatal("Fetch error = nil, want a filesystem error")
		}
		assertFileContent(t, slotPath, "not a directory")
	})

	t.Run("pointer temporary file is a directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		tmpPath := pointerTmpPath(dir, id)
		if err := os.Mkdir(tmpPath, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", tmpPath, err)
		}
		fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
		source := newTestSource(t, dir, fake, nil)

		if _, err := source.Fetch(t.Context(), watchURL(id)); err == nil {
			t.Fatal("Fetch error = nil, want a filesystem error")
		}
		if info, err := os.Lstat(tmpPath); err != nil || !info.IsDir() {
			t.Errorf("temporary-file directory was touched: mode = %v, error = %v", info, err)
		}
		if _, err := os.Lstat(pointerPath(dir, id)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("a failed commit wrote a pointer (error = %v)", err)
		}
	})

	t.Run("slot is a symlink to another directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.MkdirAll(outside, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", outside, err)
		}
		writeSlotFiles(t, outside, id, subtitleDocument("outside"), infoDocument(id, "outside"))
		if err := os.Symlink(outside, slotDirPath(dir, id, slotNameA)); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		placePointer(t, dir, id, slotNameA)

		fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
		source := newTestSource(t, dir, fake, nil)
		transcript, err := source.Fetch(t.Context(), watchURL(id))
		if err != nil {
			t.Fatalf("Fetch error = %v", err)
		}
		if fake.callCount() != 1 {
			t.Fatalf("yt-dlp ran %d times, want 1: a symlinked slot is not a valid cache", fake.callCount())
		}
		if transcript.Title != "title-run" {
			t.Errorf("title = %q, want the run generation, not the linked directory", transcript.Title)
		}
		assertFileContent(t, subtitlesPath(outside, id), subtitleDocument("outside"))
	})

	t.Run("pointer is a symlink", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		target := filepath.Join(dir, "target.txt")
		writeTestFile(t, target, "target")
		if err := os.Symlink(target, pointerPath(dir, id)); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
		source := newTestSource(t, dir, fake, nil)

		if _, err := source.Fetch(t.Context(), watchURL(id)); err == nil {
			t.Fatal("Fetch error = nil, want a filesystem error")
		}
		if info, err := os.Lstat(pointerPath(dir, id)); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("pointer symlink was replaced: mode = %v, error = %v", info, err)
		}
		assertFileContent(t, target, "target")
	})
}

func TestFetchCacheInvalidation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
	source := newTestSource(t, dir, fake, nil)

	if _, err := source.Fetch(t.Context(), watchURL(id)); err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	// Deleting the pointer invalidates the cache; deleting a slot file does too.
	if err := os.Remove(pointerPath(dir, id)); err != nil {
		t.Fatalf("remove pointer: %v", err)
	}
	if _, err := source.Fetch(t.Context(), watchURL(id)); err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if fake.callCount() != 2 {
		t.Fatalf("yt-dlp ran %d times, want 2 after the pointer was deleted", fake.callCount())
	}
	if err := os.Remove(subtitlesPath(slotDirPath(dir, id, slotNameA), id)); err != nil {
		t.Fatalf("remove subtitles: %v", err)
	}
	if _, err := source.Fetch(t.Context(), watchURL(id)); err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	if fake.callCount() != 3 {
		t.Fatalf("yt-dlp ran %d times, want 3 after a slot file was deleted", fake.callCount())
	}
}

func TestFetchCachePathUsesOnlyVideoID(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
	source := newTestSource(t, cacheDir, fake, nil)

	videoURL := watchURL(id) + "&list=../../../etc"
	if _, err := source.Fetch(t.Context(), videoURL); err != nil {
		t.Fatalf("Fetch error = %v", err)
	}
	call := fake.lastCall(t)
	slotDir := slotDirPath(cacheDir, id, slotNameA)
	for _, arg := range call.args {
		if strings.Contains(arg, "..") {
			t.Errorf("argument %q contains a path traversal", arg)
		}
		if strings.Contains(arg, cacheDir) && arg != slotDir {
			t.Errorf("argument %q uses the cache directory outside -P", arg)
		}
	}
	for _, name := range readDirNames(t, cacheDir) {
		if !strings.HasPrefix(name, id+".") {
			t.Errorf("cache entry %q does not start with the video ID", name)
		}
	}
}

func TestFetchEnvAllowlist(t *testing.T) {
	id := testdataRealVideoID

	t.Run("passes only allowlisted variables", func(t *testing.T) {
		// Take the temp directory before TMPDIR points at a test value.
		dir := filepath.Join(t.TempDir(), "cache")
		t.Setenv("YT2COLUMN_TEST_UNLISTED_MARKER", "marker")
		t.Setenv("DEEPSEEK_API_KEY", "secret")
		want := sampleAllowlistEnv()
		for name, value := range want {
			t.Setenv(name, value)
		}
		fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
		source := newTestSource(t, dir, fake, nil)

		if _, err := source.Fetch(t.Context(), watchURL(id)); err != nil {
			t.Fatalf("Fetch error = %v", err)
		}
		got := envMap(fake.lastCall(t).env)
		if !maps.Equal(got, want) {
			t.Errorf("child environment = %v, want %v", got, want)
		}
	})

	t.Run("an environment without allowlisted variables is empty", func(t *testing.T) {
		// Take the temp directory before TMPDIR is unset.
		dir := filepath.Join(t.TempDir(), "cache")
		for _, name := range allowedEnvVars {
			unsetenvForTest(t, name)
		}
		t.Setenv("DEEPSEEK_API_KEY", "secret")
		fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
		source := newTestSource(t, dir, fake, nil)

		if _, err := source.Fetch(t.Context(), watchURL(id)); err != nil {
			t.Fatalf("Fetch error = %v", err)
		}
		env := fake.lastCall(t).env
		if env == nil {
			t.Fatal("environment is nil, want a non-nil empty environment")
		}
		if len(env) != 0 {
			t.Errorf("child environment = %q, want it empty", env)
		}
	})
}

func TestFetchStderrCapAndRedaction(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	const proxyValue = "SECRET-PROXY-VALUE-abcdefghijklmnopqrstuvwxyz-0123456789"
	fragment := proxyValue[:20]
	const afterCap = "AFTER-THE-CAP-MARKER"
	const userinfo = "https://user:pass@proxy.example:8080 "

	fake := &fakeCommandExecutor{behavior: func(call fakeCommandCall) error {
		if call.stderr == nil {
			t.Error("no standard error writer was connected")
			return errors.New("exit status 1")
		}
		_, _ = io.WriteString(call.stderr, userinfo+proxyValue+" ")
		padding := maxStderrBytes - len(userinfo) - len(proxyValue) - 1 - len(fragment)
		_, _ = io.WriteString(call.stderr, strings.Repeat("p", padding))
		_, _ = io.WriteString(call.stderr, fragment+afterCap)
		return errors.New("exit status 1")
	}}
	t.Setenv("HTTPS_PROXY", proxyValue)
	source := newTestSource(t, dir, fake, func(options *Options) {
		options.Timeout = 100 * time.Millisecond
	})

	_, err := source.Fetch(t.Context(), watchURL(id))
	if !errors.Is(err, ErrYtDlpExec) {
		t.Fatalf("Fetch error = %v, want ErrYtDlpExec", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error is a timeout, want the failure to return without waiting for it")
	}
	message := err.Error()
	if !strings.Contains(message, userinfo) && !strings.Contains(message, "https://") {
		t.Errorf("error %q does not include the captured stderr", message)
	}
	if strings.Contains(message, "user:pass") {
		t.Errorf("error %q leaks URL userinfo", message)
	}
	if strings.Contains(message, proxyValue) {
		t.Errorf("error %q leaks the proxy value", message)
	}
	if strings.Contains(message, fragment) {
		t.Errorf("error %q leaks the proxy fragment at the cap boundary", message)
	}
	if strings.Contains(message, afterCap) {
		t.Errorf("error %q kept output beyond the 4 KiB cap", message)
	}
	if !strings.Contains(message, redactedMarker) {
		t.Errorf("error %q does not show that redaction happened", message)
	}
}

func TestFetchSentinelDistinction(t *testing.T) {
	id := testdataRealVideoID
	all := []error{ErrInvalidVideoURL, ErrYtDlpExec, ErrParseSubtitles, ErrParseInfo, ErrNoSubtitles, context.DeadlineExceeded, context.Canceled}

	cases := []struct {
		name  string
		own   error
		build func(t *testing.T, dir string) (*YtDlpSource, context.Context)
	}{
		{
			name: "invalid URL",
			own:  ErrInvalidVideoURL,
			build: func(t *testing.T, dir string) (*YtDlpSource, context.Context) {
				return newTestSource(t, dir, &fakeCommandExecutor{}, nil), t.Context()
			},
		},
		{
			name: "exec failure",
			own:  ErrYtDlpExec,
			build: func(t *testing.T, dir string) (*YtDlpSource, context.Context) {
				return newTestSource(t, dir, &fakeCommandExecutor{behavior: failRun(errors.New("exit status 1"))}, nil), t.Context()
			},
		},
		{
			name: "subtitle parse failure",
			own:  ErrParseSubtitles,
			build: func(t *testing.T, dir string) (*YtDlpSource, context.Context) {
				behavior := func(call fakeCommandCall) error {
					writeSlotFiles(t, slotDirFromArgs(t, call.args), id, `{"events":`, infoDocument(id, "run"))
					return nil
				}
				return newTestSource(t, dir, &fakeCommandExecutor{behavior: behavior}, nil), t.Context()
			},
		},
		{
			name: "info parse failure",
			own:  ErrParseInfo,
			build: func(t *testing.T, dir string) (*YtDlpSource, context.Context) {
				behavior := func(call fakeCommandCall) error {
					writeSlotFiles(t, slotDirFromArgs(t, call.args), id, subtitleDocument("text"), `{"id":"`+id+`","title":`)
					return nil
				}
				return newTestSource(t, dir, &fakeCommandExecutor{behavior: behavior}, nil), t.Context()
			},
		},
		{
			name: "no subtitles",
			own:  ErrNoSubtitles,
			build: func(t *testing.T, dir string) (*YtDlpSource, context.Context) {
				return newTestSource(t, dir, &fakeCommandExecutor{}, nil), t.Context()
			},
		},
		{
			name: "timeout",
			own:  context.DeadlineExceeded,
			build: func(t *testing.T, dir string) (*YtDlpSource, context.Context) {
				behavior := func(call fakeCommandCall) error {
					<-call.ctx.Done()
					return errors.New("signal: killed")
				}
				source := newTestSource(t, dir, &fakeCommandExecutor{behavior: behavior}, func(options *Options) {
					options.Timeout = 50 * time.Millisecond
				})
				return source, t.Context()
			},
		},
		{
			name: "cancellation",
			own:  context.Canceled,
			build: func(t *testing.T, dir string) (*YtDlpSource, context.Context) {
				ctx, cancel := context.WithCancel(t.Context())
				behavior := func(call fakeCommandCall) error {
					cancel()
					<-call.ctx.Done()
					return errors.New("signal: killed")
				}
				return newTestSource(t, dir, &fakeCommandExecutor{behavior: behavior}, nil), ctx
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "cache")
			source, ctx := tc.build(t, dir)
			url := watchURL(id)
			if tc.own == ErrInvalidVideoURL {
				url = "https://example.com/watch?v=" + id
			}
			_, err := source.Fetch(ctx, url)
			if !errors.Is(err, tc.own) {
				t.Fatalf("Fetch error = %v, want %v", err, tc.own)
			}
			for _, other := range all {
				if other == tc.own {
					continue
				}
				if errors.Is(err, other) {
					t.Errorf("Fetch error %v also matches %v", err, other)
				}
			}
		})
	}
}

func TestFetchInterruptedStates(t *testing.T) {
	id := testdataRealVideoID
	oldGen := generationFor(id, "old")
	newGen := generationFor(id, "new")
	partial := generationFor(id, "partial")

	states := []struct {
		name         string
		currentTitle string // the generation a non-forced Fetch returns; empty means a miss
		build        func(t *testing.T, dir string)
	}{
		{"S1", "title-old", func(t *testing.T, dir string) { placeStateS1(t, dir, id, oldGen, partial) }},
		{"S2", "title-old", func(t *testing.T, dir string) { placeStateS2(t, dir, id, oldGen, newGen) }},
		{"S3", "title-new", func(t *testing.T, dir string) { placeStateS3(t, dir, id, oldGen, newGen) }},
		{"S4", "", func(t *testing.T, dir string) { placeStateS4(t, dir, id, partial) }},
		{"S5", "", func(t *testing.T, dir string) { placeStateS5(t, dir, id, partial) }},
		{"S6", "", func(t *testing.T, dir string) { placeStateS6(t, dir, id, partial) }},
	}
	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			t.Run("non-forced Fetch", func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "cache")
				state.build(t, dir)
				fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
				source := newTestSource(t, dir, fake, nil)

				transcript, err := source.Fetch(t.Context(), watchURL(id))
				if err != nil {
					t.Fatalf("Fetch error = %v", err)
				}
				assertSingleGeneration(t, transcript)
				if state.currentTitle != "" {
					if fake.callCount() != 0 {
						t.Errorf("yt-dlp ran %d times, want 0 for a valid generation", fake.callCount())
					}
					if transcript.Title != state.currentTitle {
						t.Errorf("title = %q, want %q", transcript.Title, state.currentTitle)
					}
				} else if fake.callCount() != 1 {
					t.Errorf("yt-dlp ran %d times, want 1 for a miss", fake.callCount())
				}
				assertNoDangling(t, dir, id)
			})

			t.Run("forced success", func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "cache")
				state.build(t, dir)
				fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, runGeneration(id))}
				source := newTestSource(t, dir, fake, func(options *Options) { options.ForceRefresh = true })

				transcript, err := source.Fetch(t.Context(), watchURL(id))
				if err != nil {
					t.Fatalf("Fetch error = %v", err)
				}
				assertSingleGeneration(t, transcript)
				if transcript.Title != "title-run" {
					t.Errorf("title = %q, want the refreshed generation", transcript.Title)
				}
				assertNoDangling(t, dir, id)
			})

			t.Run("forced failure", func(t *testing.T) {
				for _, failure := range refreshFailureCases() {
					t.Run(failure.name, func(t *testing.T) {
						dir := filepath.Join(t.TempDir(), "cache")
						state.build(t, dir)

						ctx := t.Context()
						cancel := context.CancelFunc(func() {})
						if failure.canceled {
							ctx, cancel = context.WithCancel(ctx)
						}
						defer cancel()
						fake := &fakeCommandExecutor{behavior: failure.behavior(t, id, cancel)}
						source := newTestSource(t, dir, fake, func(options *Options) {
							options.ForceRefresh = true
							if failure.timeout != 0 {
								options.Timeout = failure.timeout
							}
						})

						if _, err := source.Fetch(ctx, watchURL(id)); err == nil {
							t.Fatal("Fetch error = nil, want a failure")
						}
						assertNoDangling(t, dir, id)
					})
				}
			})
		})
	}
}

func TestFetchDanglingDeleteFailure(t *testing.T) {
	requireNonRoot(t)
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	placeRealCache(t, dir, id)
	staleDir := slotDirPath(dir, id, slotNameB)
	writeTestFile(t, filepath.Join(staleDir, "busy.txt"), "x")
	chmodForTest(t, staleDir, 0o500)

	fake := &fakeCommandExecutor{}
	source := newTestSource(t, dir, fake, nil)
	transcript, err := source.Fetch(t.Context(), watchURL(id))
	if err != nil {
		t.Fatalf("Fetch error = %v, want the hit to succeed despite the cleanup failure", err)
	}
	if fake.callCount() != 0 {
		t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
	}
	if transcript.Title == "" {
		t.Error("Fetch returned an empty title")
	}
	// The undeletable entry is left for a later Fetch or prune, and the
	// result of this Fetch is unchanged.
	if _, err := os.Lstat(staleDir); err != nil {
		t.Errorf("dangling slot disappeared: %v", err)
	}
}

func TestFetchUnreadableSubtitleFile(t *testing.T) {
	id := testdataRealVideoID

	t.Run("directory subtitle in the run output", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		fake := &fakeCommandExecutor{behavior: func(call fakeCommandCall) error {
			slotDir := slotDirFromArgs(t, call.args)
			if err := os.Mkdir(subtitlesPath(slotDir, id), 0o700); err != nil {
				t.Fatalf("mkdir subtitles: %v", err)
			}
			writeTestFile(t, infoPath(slotDir, id), infoDocument(id, "run"))
			return nil
		}}
		source := newTestSource(t, dir, fake, nil)

		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, ErrParseSubtitles) {
			t.Fatalf("Fetch error = %v, want ErrParseSubtitles", err)
		}
		if errors.Is(err, ErrNoSubtitles) {
			t.Errorf("unreadable subtitle also matches ErrNoSubtitles")
		}
		parseErr, ok := errors.AsType[*ParseError](err)
		if !ok {
			t.Fatalf("Fetch error = %v, want *ParseError", err)
		}
		if want := subtitlesPath(slotDirPath(dir, id, slotNameA), id); parseErr.Path != want {
			t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, want)
		}
	})

	t.Run("directory info in the run output", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		fake := &fakeCommandExecutor{behavior: func(call fakeCommandCall) error {
			slotDir := slotDirFromArgs(t, call.args)
			writeTestFile(t, subtitlesPath(slotDir, id), subtitleDocument("text"))
			if err := os.Mkdir(infoPath(slotDir, id), 0o700); err != nil {
				t.Fatalf("mkdir info: %v", err)
			}
			return nil
		}}
		source := newTestSource(t, dir, fake, nil)

		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, ErrParseInfo) {
			t.Fatalf("Fetch error = %v, want ErrParseInfo", err)
		}
		parseErr, ok := errors.AsType[*ParseError](err)
		if !ok {
			t.Fatalf("Fetch error = %v, want *ParseError", err)
		}
		if want := infoPath(slotDirPath(dir, id, slotNameA), id); parseErr.Path != want {
			t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, want)
		}
	})

	t.Run("unreadable cached subtitle", func(t *testing.T) {
		requireNonRoot(t)
		dir := filepath.Join(t.TempDir(), "cache")
		placeRealCache(t, dir, id)
		path := subtitlesPath(slotDirPath(dir, id, slotNameA), id)
		chmodForTest(t, path, 0o000)
		source := newTestSource(t, dir, &fakeCommandExecutor{}, nil)

		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, ErrParseSubtitles) {
			t.Fatalf("Fetch error = %v, want ErrParseSubtitles", err)
		}
		parseErr, ok := errors.AsType[*ParseError](err)
		if !ok {
			t.Fatalf("Fetch error = %v, want *ParseError", err)
		}
		if parseErr.Path != path {
			t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, path)
		}
		assertFileContent(t, pointerPath(dir, id), slotNameA)
	})

	t.Run("unsearchable cached slot", func(t *testing.T) {
		requireNonRoot(t)
		dir := filepath.Join(t.TempDir(), "cache")
		placeRealCache(t, dir, id)
		slotDir := slotDirPath(dir, id, slotNameA)
		// Without search permission Lstat of both outputs fails with EACCES, not ENOENT.
		chmodForTest(t, slotDir, 0o600)
		fake := &fakeCommandExecutor{behavior: writeGeneration(t, id, generationFor(id, "new"))}
		source := newTestSource(t, dir, fake, nil)

		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, ErrParseSubtitles) {
			t.Fatalf("Fetch error = %v, want ErrParseSubtitles", err)
		}
		parseErr, ok := errors.AsType[*ParseError](err)
		if !ok {
			t.Fatalf("Fetch error = %v, want *ParseError", err)
		}
		if want := subtitlesPath(slotDir, id); parseErr.Path != want {
			t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, want)
		}
		if fake.callCount() != 0 {
			t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
		}
		assertFileContent(t, pointerPath(dir, id), slotNameA)
	})
}

func TestFetchUnreadablePointer(t *testing.T) {
	requireNonRoot(t)
	dir := filepath.Join(t.TempDir(), "cache")
	id := testdataRealVideoID
	subtitles, info := placeRealCache(t, dir, id)
	chmodForTest(t, pointerPath(dir, id), 0o000)

	fake := &fakeCommandExecutor{}
	source := newTestSource(t, dir, fake, nil)
	_, err := source.Fetch(t.Context(), watchURL(id))
	if err == nil {
		t.Fatal("Fetch error = nil, want a read failure")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("Fetch error = %v, want a permission failure", err)
	}
	if fake.callCount() != 0 {
		t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
	}
	// Treating an unreadable pointer as invalid would have deleted the cache.
	slotDir := slotDirPath(dir, id, slotNameA)
	assertFileContent(t, subtitlesPath(slotDir, id), string(subtitles))
	assertFileContent(t, infoPath(slotDir, id), string(info))
	if _, err := os.Lstat(pointerPath(dir, id)); err != nil {
		t.Errorf("pointer disappeared: %v", err)
	}
}

func TestFetchOversizedFiles(t *testing.T) {
	id := testdataRealVideoID

	t.Run("cached subtitle over the limit", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		slotDir := slotDirPath(dir, id, slotNameA)
		writeSparseFile(t, subtitlesPath(slotDir, id), maxSubtitlesBytes+1)
		writeTestFile(t, infoPath(slotDir, id), infoDocument(id, "old"))
		placePointer(t, dir, id, slotNameA)
		fake := &fakeCommandExecutor{}
		source := newTestSource(t, dir, fake, nil)

		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, ErrParseSubtitles) || !errors.Is(err, errInputTooLarge) {
			t.Fatalf("Fetch error = %v, want ErrParseSubtitles with the size limit", err)
		}
		parseErr, ok := errors.AsType[*ParseError](err)
		if !ok || parseErr.Path != subtitlesPath(slotDir, id) {
			t.Errorf("Fetch error = %v, want a ParseError with the subtitle path", err)
		}
		if fake.callCount() != 0 {
			t.Errorf("yt-dlp ran %d times, want 0", fake.callCount())
		}
	})

	t.Run("cached info over the limit", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		slotDir := slotDirPath(dir, id, slotNameA)
		writeTestFile(t, subtitlesPath(slotDir, id), subtitleDocument("old"))
		writeSparseFile(t, infoPath(slotDir, id), maxInfoBytes+1)
		placePointer(t, dir, id, slotNameA)
		fake := &fakeCommandExecutor{}
		source := newTestSource(t, dir, fake, nil)

		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, ErrParseInfo) || !errors.Is(err, errInputTooLarge) {
			t.Fatalf("Fetch error = %v, want ErrParseInfo with the size limit", err)
		}
		parseErr, ok := errors.AsType[*ParseError](err)
		if !ok || parseErr.Path != infoPath(slotDir, id) {
			t.Errorf("Fetch error = %v, want a ParseError with the info path", err)
		}
	})

	t.Run("run output over the limit", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		fake := &fakeCommandExecutor{behavior: func(call fakeCommandCall) error {
			slotDir := slotDirFromArgs(t, call.args)
			writeSparseFile(t, subtitlesPath(slotDir, id), maxSubtitlesBytes+1)
			writeTestFile(t, infoPath(slotDir, id), infoDocument(id, "run"))
			return nil
		}}
		source := newTestSource(t, dir, fake, nil)

		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, ErrParseSubtitles) || !errors.Is(err, errInputTooLarge) {
			t.Fatalf("Fetch error = %v, want ErrParseSubtitles with the size limit", err)
		}
		if _, err := os.Lstat(pointerPath(dir, id)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("an oversized run output committed a cache entry (error = %v)", err)
		}
	})
}

func TestFetchSymlinkSlotOutput(t *testing.T) {
	id := testdataRealVideoID
	// The targets are valid documents with a mode the cache never sets, so
	// following the link would make Fetch succeed and the commit chmod them.
	const targetMode fs.FileMode = 0o640

	t.Run("symlink subtitle in the run output", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		target := filepath.Join(dir, "target.json3")
		targetContent := subtitleDocument("target")
		writeTestFile(t, target, targetContent)
		if err := os.Chmod(target, targetMode); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		fake := &fakeCommandExecutor{behavior: func(call fakeCommandCall) error {
			slotDir := slotDirFromArgs(t, call.args)
			if err := os.Symlink(target, subtitlesPath(slotDir, id)); err != nil {
				t.Fatalf("symlink: %v", err)
			}
			writeTestFile(t, infoPath(slotDir, id), infoDocument(id, "run"))
			return nil
		}}
		source := newTestSource(t, dir, fake, nil)

		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, ErrParseSubtitles) || !errors.Is(err, errNotRegularFile) {
			t.Fatalf("Fetch error = %v, want ErrParseSubtitles with errNotRegularFile", err)
		}
		parseErr, ok := errors.AsType[*ParseError](err)
		if !ok {
			t.Fatalf("Fetch error = %v, want *ParseError", err)
		}
		if want := subtitlesPath(slotDirPath(dir, id, slotNameA), id); parseErr.Path != want {
			t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, want)
		}
		assertFileContent(t, target, targetContent)
		assertMode(t, target, targetMode)
	})

	t.Run("symlink info in the cache", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		slotDir := slotDirPath(dir, id, slotNameA)
		writeTestFile(t, subtitlesPath(slotDir, id), subtitleDocument("old"))
		target := filepath.Join(dir, "target.info.json")
		targetContent := infoDocument(id, "target")
		writeTestFile(t, target, targetContent)
		if err := os.Chmod(target, targetMode); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		if err := os.Symlink(target, infoPath(slotDir, id)); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		placePointer(t, dir, id, slotNameA)
		source := newTestSource(t, dir, &fakeCommandExecutor{}, nil)

		_, err := source.Fetch(t.Context(), watchURL(id))
		if !errors.Is(err, ErrParseInfo) || !errors.Is(err, errNotRegularFile) {
			t.Fatalf("Fetch error = %v, want ErrParseInfo with errNotRegularFile", err)
		}
		parseErr, ok := errors.AsType[*ParseError](err)
		if !ok {
			t.Fatalf("Fetch error = %v, want *ParseError", err)
		}
		if parseErr.Path != infoPath(slotDir, id) {
			t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, infoPath(slotDir, id))
		}
		assertFileContent(t, target, targetContent)
		assertMode(t, target, targetMode)
		assertFileContent(t, pointerPath(dir, id), slotNameA)
	})
}

// firstLineIs reports an error unless the file at path exists and its first
// line is exactly want.
func firstLineIs(path, want string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	line, _, _ := strings.Cut(string(content), "\n")
	if line != want {
		return fmt.Errorf("%s: first line is %q, want %q", path, line, want)
	}
	return nil
}

// TestIntegrationTestBuildTag pins the build tag that keeps the integration
// test, which runs the real yt-dlp against the network, out of `make test`
// and `make test-ci`. The guard itself is checked to fail on a missing file
// and on a different first line.
func TestIntegrationTestBuildTag(t *testing.T) {
	const want = "//go:build integration"
	if err := firstLineIs("integration_test.go", want); err != nil {
		t.Fatalf("integration_test.go must start with %q: %v", want, err)
	}

	dir := t.TempDir()
	if err := firstLineIs(filepath.Join(dir, "missing_test.go"), want); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: error = %v, want fs.ErrNotExist", err)
	}
	wrong := filepath.Join(dir, "wrong_test.go")
	if err := os.WriteFile(wrong, []byte("//go:build test\n\npackage transcript\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := firstLineIs(wrong, want); err == nil {
		t.Error("wrong first line: error = nil, want a mismatch")
	}
}

// buildTagsFlag matches a golangci-lint --build-tags argument and its value.
var buildTagsFlag = regexp.MustCompile(`--build-tags[= ](\S+)`)

// TestLintTagsIncludeIntegration pins that every lint path (make lint,
// pre-commit, CI) checks the integration test twice: golangci-lint analyzes
// it with both build tags, and go vet compiles the `-tags integration` build
// that `make test-integration` runs, which the first never builds. The
// golangci-lint check is tied to the line that runs it, so a comment
// mentioning the flag does not satisfy it.
func TestLintTagsIncludeIntegration(t *testing.T) {
	const (
		wantTags = "test,integration"
		vet      = "vet -tags integration ./..."
	)
	root := filepath.Join("..", "..")
	for _, tc := range []struct {
		name string
		// lintLine marks the line that runs golangci-lint.
		lintLine string
	}{
		{name: "Makefile", lintLine: "golangci-lint@"},
		{name: ".pre-commit-config.yaml", lintLine: "golangci-lint@"},
		{name: filepath.Join(".github", "workflows", "ci.yml"), lintLine: "args:"},
	} {
		content, err := os.ReadFile(filepath.Join(root, tc.name))
		if err != nil {
			t.Fatalf("read %s: %v", tc.name, err)
		}
		lintRuns := 0
		for line := range strings.Lines(string(content)) {
			if !strings.Contains(line, tc.lintLine) {
				continue
			}
			for _, match := range buildTagsFlag.FindAllStringSubmatch(line, -1) {
				lintRuns++
				if match[1] != wantTags {
					t.Errorf("%s: golangci-lint --build-tags %s, want %s", tc.name, match[1], wantTags)
				}
			}
		}
		if lintRuns == 0 {
			t.Errorf("%s: no golangci-lint line with --build-tags", tc.name)
		}
		if !strings.Contains(string(content), vet) {
			t.Errorf("%s: missing %q", tc.name, vet)
		}
	}
}
