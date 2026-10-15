package writer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/transcript"
)

// Stage names one of the pipeline's post-transcript stages. The zero value is
// StageUnset, which no artifact uses.
type Stage int

const (
	// StageUnset is the zero value of Stage. It names no stage: a run with it
	// starts from generation and saves nothing on its own.
	StageUnset Stage = iota
	// StageGeneration is the generation stage, which turns the material into
	// the generated text.
	StageGeneration
	// StageShorten is the shorten step, which shortens the generated text.
	StageShorten
	// StageReview is the review step, which revises the shortened text.
	StageReview
)

// String returns the stage's name in the artifact format ("generation",
// "shorten", or "review"). StageUnset and values outside the enumeration
// return "unknown".
func (s Stage) String() string {
	switch s {
	case StageGeneration:
		return "generation"
	case StageShorten:
		return "shorten"
	case StageReview:
		return "review"
	default:
		return "unknown"
	}
}

// TemplateIdentity names the template one stage used: whether it is the
// embedded default and a digest of its text. The override file's path is not
// recorded.
type TemplateIdentity struct {
	Embedded bool
	Digest   string
}

// StageRecord is one stage's output, saved by or loaded from the store.
type StageRecord struct {
	Stage        Stage
	Title        string
	Body         string     // the body part, without the source block
	Revisions    []Revision // StageReview only
	Model        string
	ModelVersion string
	System       TemplateIdentity
	User         TemplateIdentity
	Input        string // the identifier of the input this stage consumed
	ID           string // the identifier of this record, computed from its content
}

// ArtifactChain is the loaded chain that ends at the resume stage, verified to
// form one run. Generation is always present; Shorten and Review are present
// only when the chain contains them.
type ArtifactChain struct {
	Generation StageRecord
	Shorten    *StageRecord
	Review     *StageRecord
}

// ArtifactStore saves and loads the intermediate artifacts of a run.
// Implementations must reject malformed input rather than repair it, and must
// not create the artifacts directory.
type ArtifactStore interface {
	// Load verifies the chain of videoID's artifacts that ends at stage
	// against materialID and returns it.
	Load(ctx context.Context, videoID string, stage Stage, materialID string) (ArtifactChain, error)
	// Save writes one completed stage's record for videoID and returns its
	// identifier.
	Save(ctx context.Context, videoID string, record StageRecord) (id string, err error)
}

// ArtifactError wraps every failure to save or load a stage's artifact. Error
// prefixes the stage name; Unwrap returns the original error.
type ArtifactError struct {
	Stage Stage
	Err   error
}

// Error returns "<stage> artifact: <original error>".
func (e *ArtifactError) Error() string {
	return fmt.Sprintf("%s artifact: %s", e.Stage, e.Err)
}

// Unwrap returns the original error.
func (e *ArtifactError) Unwrap() error { return e.Err }

// artifactError wraps a store failure as the failure of one stage.
func artifactError(stage Stage, err error) error {
	return &ArtifactError{Stage: stage, Err: err}
}

// ArtifactOptions configures saving the intermediate artifacts of a run and
// resuming from a saved one. The zero value saves nothing and starts from
// generation.
type ArtifactOptions struct {
	// Store saves each completed stage's output and loads the chain to
	// resume from. Required when Resume is set; nil otherwise saves nothing
	// and resumes from nothing.
	Store ArtifactStore
	// Resume is the stage whose saved output to start from. StageUnset
	// starts from generation.
	Resume Stage
}

// Digest returns the artifact identifier of the given parts: the SHA-256 of
// each part's decimal byte length, a ":", then the part, concatenated. Stating
// each length makes the parts' boundaries unambiguous, so distinct sequences
// of parts never share an identifier.
func Digest(parts ...string) string {
	var builder strings.Builder
	for _, part := range parts {
		builder.WriteString(strconv.Itoa(len(part)))
		builder.WriteByte(':')
		builder.WriteString(part)
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

// MaterialID returns the identifier of the material a generation consumes: the
// video ID and the transcript text's four values. Including the video ID binds
// the chain to it, so an artifact copied under another video ID's name still
// mismatches.
func MaterialID(t transcript.Transcript) string {
	data := newTemplateData(t)
	return Digest(t.VideoID, data.Title, data.ChannelName, data.Description, data.Transcript)
}
