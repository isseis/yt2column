//go:build test

package coveragecompare

import (
	"context"
	"errors"
	"testing"
)

const (
	// testModulePath is the module path the parser tests give parseGoProfile.
	testModulePath = "github.com/isseis/yt2column"
	// testExtensionDir is the checkout extension directory the TAP parser
	// tests give parseTAP.
	testExtensionDir = "/checkout/extension"
)

// requireStop fails the test unless err is an unreadable StopError at point and
// input, and returns its first reason.
func requireStop(t *testing.T, err error, point Point, input Input) StopReason {
	t.Helper()
	stop, ok := errors.AsType[*StopError](err)
	if !ok {
		t.Fatalf("error = %v, want *StopError", err)
	}
	if stop.Kind != StopUnreadable {
		t.Fatalf("stop kind = %v, want %v (reasons: %+v)", stop.Kind, StopUnreadable, stop.Reasons)
	}
	if len(stop.Reasons) == 0 {
		t.Fatal("stop has no reasons")
	}
	reason := stop.Reasons[0]
	if reason.Point != point {
		t.Errorf("reason point = %v, want %v", reason.Point, point)
	}
	if reason.Input != input {
		t.Errorf("reason input = %v, want %v", reason.Input, input)
	}
	return reason
}

// fakeSource is a source over in-memory files. ranges holds the range
// helper's answer for each file, for fakeRanges.
type fakeSource struct {
	at      Point
	files   map[string]string
	ranges  map[string]fileRanges
	readErr error
}

func (s *fakeSource) point() Point { return s.at }

func (s *fakeSource) readFile(path string) ([]byte, bool, error) {
	if s.readErr != nil {
		return nil, false, s.readErr
	}
	data, ok := s.files[path]
	if !ok {
		return nil, false, nil
	}
	return []byte(data), true, nil
}

// fakeRanges answers the range helper's call from the ranges of a fakeSource.
type fakeRanges struct {
	err error
}

func (r fakeRanges) functionRanges(_ context.Context, src source, files []string) ([]fileRanges, error) {
	if r.err != nil {
		return nil, r.err
	}
	fake, ok := src.(*fakeSource)
	if !ok {
		return nil, errFakeSourceType
	}
	answer := make([]fileRanges, 0, len(files))
	for _, file := range files {
		entry, ok := fake.ranges[file]
		if !ok {
			continue
		}
		entry.Path = file
		answer = append(answer, entry)
	}
	return answer, nil
}

// errFakeSourceType is returned when fakeRanges is given a source it did not
// expect.
var errFakeSourceType = errors.New("fakeRanges needs a *fakeSource")
