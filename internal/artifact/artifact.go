// Package artifact persists the intermediate artifacts of a writer run: one
// JSON file per video ID and stage, under a directory the caller names. It
// reads an artifact back as untrusted input, verifying its format, identifier,
// and chain, and never repairs or partly accepts a malformed file.
package artifact

import (
	"errors"
	"fmt"

	"github.com/isseis/yt2column/internal/writer"
)

// maxArtifactBytes caps one artifact file. It leaves room for a generated
// article (at most 1 MiB) and a review list (at most 1 MiB) in a single file,
// with margin for JSON escaping.
const maxArtifactBytes = 4 << 20 // 4 MiB

// artifactVersion is the only format version this package reads and writes.
// Version 2 adds the usage element to version 1.
const artifactVersion = 2

// sha256HexLen is the length of a SHA-256 digest written in hex.
const sha256HexLen = 64

// Sentinels a caller identifies with errors.Is.
var (
	// ErrMissing reports that the artifacts directory or the requested
	// artifact is missing or unusable (not a directory, a symbolic link, or
	// absent).
	ErrMissing = errors.New("artifact is missing")
	// ErrMismatch reports that an artifact's stage or input identifier does
	// not match the requested stage, the current material, or the previous
	// stage's artifact.
	ErrMismatch = errors.New("artifact does not match the run")
	// ErrMalformed reports that an artifact's content breaks a rule: it is
	// empty, too large, not valid UTF-8, has an unknown or duplicate key or
	// trailing data, an unsupported version, or its title, body, model, or
	// review list fails validation. Every return wraps it together with
	// writer.ErrMalformedOutput.
	ErrMalformed = errors.New("malformed artifact")
	// errInvalidVideoID rejects a video ID that failed transcript's
	// validation before any file is opened. It is not exported: a caller
	// passes only a validated video ID, so a rejected one is a programming
	// error.
	errInvalidVideoID = errors.New("invalid video ID")
	// errInvalidStage rejects a stage this package does not know. It is not
	// exported for the same reason.
	errInvalidStage = errors.New("invalid stage")
)

// malformed returns an ErrMalformed that also wraps writer.ErrMalformedOutput
// and names the broken rule only, never an artifact value.
func malformed(reason string) error {
	return fmt.Errorf("%w: %w: %s", ErrMalformed, writer.ErrMalformedOutput, reason)
}

// malformedf is malformed with a formatted reason. The reason must not carry
// an artifact value.
func malformedf(format string, args ...any) error {
	return malformed(fmt.Sprintf(format, args...))
}

// mismatchf returns an ErrMismatch naming the mismatched identifiers and the
// recovery for the common case (the material changed). The identifiers are
// SHA-256 digests, never article text.
func mismatchf(format string, args ...any) error {
	return fmt.Errorf("%w: %s; rerun without --after, or keep the cache with --keep-cache", ErrMismatch, fmt.Sprintf(format, args...))
}

// isKnownStage reports whether stage is one this package reads and writes.
func isKnownStage(stage writer.Stage) bool {
	switch stage {
	case writer.StageGeneration, writer.StageShorten, writer.StageReview:
		return true
	default:
		return false
	}
}

// stageFromWire returns the Stage whose artifact name is wire, and whether
// wire named one. The wire value is compared against Stage.String, the single
// source of the stage spellings.
func stageFromWire(wire string) (writer.Stage, bool) {
	for _, stage := range [...]writer.Stage{writer.StageGeneration, writer.StageShorten, writer.StageReview} {
		if stage.String() == wire {
			return stage, true
		}
	}
	return writer.StageUnset, false
}
