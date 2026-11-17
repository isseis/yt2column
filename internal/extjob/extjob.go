// Package extjob is the article generation of one browser extension job: it
// validates a start input, derives the value that identifies the input, runs
// the three generation steps with a given pair of LLM clients, checks the
// finished article, and assembles the result the extension shows, saves, or
// copies. It also holds the JSON entry points the article generation module
// registers for the extension (Module).
//
// It knows nothing of HTTP, the file system, or which LLM implementation a
// build uses: a Backend, given by the main package of a module, builds the
// clients and asks the API about a key. Every package it imports builds for
// js/wasm.
package extjob

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// ErrInvalidInput is wrapped by Prepare for a start input it does not accept.
// It never holds the input.
var ErrInvalidInput = errors.New("extjob: the start input is not accepted")

// MaxPrepareRequestBytes bounds a JSON request of prepare; PrepareJSON rejects
// a longer one. It is larger than the writer's prompt limit (1 MiB) even
// after the transcript's timestamp lines and the JSON escaping can multiply
// it, so an input the writer could still handle is not refused here.
const MaxPrepareRequestBytes = 4 << 20 // 4 MiB

// StartInput is a start input as received from the page, before validation.
type StartInput struct {
	VideoURL, Transcript, Title, ChannelName, Description string
}

// Prepared is a validated input. Its fields are unexported; only Prepare
// builds one, so a Prepared is always a validated input. It holds the parsed
// source and the values derived from the input, never a second copy of the
// raw input.
type Prepared struct {
	source      *transcript.CopiedSource
	videoURL    string // the normalized URL derived from the input's video URL
	videoID     string
	videoTitle  string // "" when the input had none
	inputDigest string
}

// Prepare validates in and returns the Prepared input. The steps are, in
// order: the required strings, the channel name, the video URL, and the copied
// transcript's format. Every failure wraps ErrInvalidInput, and no value is
// repaired: a rejected input is never trimmed or normalized into an accepted
// one (the video URL is normalized for the key, never replaced).
func Prepare(in StartInput) (Prepared, error) {
	if strings.TrimSpace(in.VideoURL) == "" || strings.TrimSpace(in.Transcript) == "" {
		return Prepared{}, fmt.Errorf("%w: a required value is empty", ErrInvalidInput)
	}
	// Reject here, not later: the same violation found by the writer's
	// transcript check surfaces as writer.ErrInvalidTranscript, which a run
	// classifies as internal.
	if writer.CheckChannelName(in.ChannelName) != nil {
		return Prepared{}, fmt.Errorf("%w: the channel name", ErrInvalidInput)
	}
	videoID, normalizedURL, err := transcript.ValidateVideoURL(in.VideoURL)
	if err != nil {
		return Prepared{}, fmt.Errorf("%w: the video URL", ErrInvalidInput)
	}
	source, err := transcript.NewCopiedSource(transcript.CopiedInput{
		VideoID:     videoID,
		Raw:         in.Transcript,
		Title:       in.Title,
		ChannelName: in.ChannelName,
		Description: in.Description,
	})
	if err != nil {
		return Prepared{}, fmt.Errorf("%w: the transcript", ErrInvalidInput)
	}
	return Prepared{
		source:     source,
		videoURL:   normalizedURL,
		videoID:    videoID,
		videoTitle: in.Title,
		// An absent optional member is "", so an absent and an empty member
		// give the same digest.
		inputDigest: inputDigest(normalizedURL, in.Transcript, in.Title, in.ChannelName, in.Description),
	}, nil
}

// InputDigest returns the digest that identifies the validated input: 64
// lowercase hexadecimal characters. It is used only to tell whether two
// inputs are the same, never as a secret or a lookup key from outside.
func (p Prepared) InputDigest() string { return p.inputDigest }

// VideoID returns the video ID the input's URL names.
func (p Prepared) VideoID() string { return p.videoID }

// digestDomain separates the digest's encoding from any other use of SHA-256
// in this repository and names the encoding's version.
const digestDomain = "yt2column extension input digest v1"

// inputDigest returns the SHA-256 of digestDomain, a 0x00 byte, and the five
// values in field order, each as its byte length in 8 big-endian bytes
// followed by its bytes. The length prefix keeps the encoding unambiguous:
// moving the boundary between two values changes the bytes. It is the
// article key of the localhost server without the per-process secret and
// with its own domain string.
func inputDigest(videoURL, transcriptText, title, channelName, description string) string {
	h := sha256.New()
	h.Write([]byte(digestDomain))
	h.Write([]byte{0})
	for _, value := range []string{videoURL, transcriptText, title, channelName, description} {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		h.Write(length[:])
		h.Write([]byte(value))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Observer receives each step change of a run, from the goroutine that runs
// it, in order.
type Observer interface {
	StepChanged(steps Steps)
}

// Deps are what a run needs besides its input: the generation and review
// clients built from the loaded configuration.
type Deps struct {
	Generation llm.LLMClient
	Review     llm.LLMClient
}

// The status values of the JSON results.
const (
	statusDone     = "done"
	statusFailed   = "failed"
	statusRejected = "rejected"
)

// StepState is the state of one step of a run. The zero value is StepPending.
type StepState int

// The StepState values.
const (
	StepPending StepState = iota // not started
	StepRunning                  // started and not finished
	StepDone                     // finished
	StepSkipped                  // not needed for this article
	StepFailed                   // running when the run failed
)

// String returns the state's JSON value. A value outside the enumeration is
// pending, the state that claims nothing.
func (s StepState) String() string {
	switch s {
	case StepRunning:
		return "running"
	case StepDone:
		return statusDone
	case StepSkipped:
		return "skipped"
	case StepFailed:
		return statusFailed
	default:
		return "pending"
	}
}

// allStepStates lists every StepState, so the size check renders the widest.
var allStepStates = []StepState{StepPending, StepRunning, StepDone, StepSkipped, StepFailed}

// Steps holds the state of each step of a run.
type Steps struct {
	Draft   StepState // writer.StageGeneration
	Shorten StepState // writer.StageShorten
	Review  StepState // writer.StageReview
}

// stepsJSON is Steps as JSON: each member is a state's JSON value.
type stepsJSON struct {
	Draft   string `json:"draft"`
	Shorten string `json:"shorten"`
	Review  string `json:"review"`
}

func (s Steps) json() stepsJSON {
	return stepsJSON{Draft: s.Draft.String(), Shorten: s.Shorten.String(), Review: s.Review.String()}
}

// field returns the state of the step a writer stage names, or nil for a
// stage that is not one of the three steps.
func (s *Steps) field(stage writer.Stage) *StepState {
	switch stage {
	case writer.StageGeneration:
		return &s.Draft
	case writer.StageShorten:
		return &s.Shorten
	case writer.StageReview:
		return &s.Review
	default:
		return nil
	}
}

// Failure is why a run failed. The zero value is FailureInternal.
type Failure int

// The Failure values. The order is part of the contract: the zero value comes
// first and a value is appended, never inserted.
const (
	FailureInternal Failure = iota
	FailureTranscriptTooLong
	FailureLLMTimeout
	FailureLLMEmpty
	FailureLLMTruncated
	FailureLLM
	FailureArticleInvalid
	FailureArticleTooLarge
	FailureAPIKeyRejected
	FailureSettingsInvalid // the stored settings did not pass the check
)

// String returns the failure's JSON value. A value outside the enumeration is
// internal.
func (f Failure) String() string {
	switch f {
	case FailureTranscriptTooLong:
		return "transcript_too_long"
	case FailureLLMTimeout:
		return "llm_timeout"
	case FailureLLMEmpty:
		return "llm_empty"
	case FailureLLMTruncated:
		return "llm_truncated"
	case FailureLLM:
		return "llm_failed"
	case FailureArticleInvalid:
		return "article_invalid"
	case FailureArticleTooLarge:
		return "article_too_large"
	case FailureAPIKeyRejected:
		return "api_key_rejected"
	case FailureSettingsInvalid:
		return "settings_invalid"
	default:
		return "internal"
	}
}

// InternalDetail is the kind of an internal failure. It is for the job record
// and the developer log; the empty value means the failure has none.
type InternalDetail string

// The InternalDetail values Go produces.
const (
	DetailNone          InternalDetail = ""
	DetailPanic         InternalDetail = "panic"
	DetailInputMismatch InternalDetail = "input_mismatch"
	DetailRequestShape  InternalDetail = "request_shape"
)
