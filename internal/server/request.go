package server

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/isseis/yt2column/internal/strictjson"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// maxBodyBytes bounds the request body. It is larger than the writer's prompt
// limit (1 MiB) even after the transcript's timestamp lines and the JSON
// escaping can multiply it, so a request the writer could still handle is not
// refused here.
const maxBodyBytes = 4 << 20 // 4 MiB

// jobInput is one validated request, ready for a job. It holds the parsed
// source and the values derived from the request, never the raw request body.
type jobInput struct {
	source    *transcript.CopiedSource
	videoURL  string // the normalized URL derived from videoUrl
	videoID   string
	requestID string
}

// readRequest reads and validates the body of an already-authenticated request.
// It returns the job input on success, or a rejection. A rejection with
// readFailed set is a body-read failure (a cut connection or a timeout), which
// the handler answers with no response and no notice.
func (s *Server) readRequest(w http.ResponseWriter, r *http.Request, requestID string) (*jobInput, *rejection) {
	// A declared length over the limit is refused without reading a byte.
	if r.ContentLength > maxBodyBytes {
		return nil, &rejection{reason: reasonBodyTooLarge, detail: detailBodyTooLarge}
	}
	// MaxBytesReader is still needed for a body with no Content-Length (chunked
	// transfer). It stops the read at the limit instead of buffering it all.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return nil, &rejection{reason: reasonBodyTooLarge, detail: detailBodyTooLarge}
		}
		return nil, &rejection{readFailed: true, err: err}
	}
	return s.validateBody(body, requestID)
}

// validateBody checks the body and builds the job input. The steps are: the
// whole-body checks (strict UTF-8, no unpaired surrogate escape, one object
// with no trailing value), the declared members (duplicates rejected, unknown
// members ignored), the required strings, the optional strings, the video URL,
// and the copied transcript's format. Every failure maps to a reason by the
// step that produced it, never by the error's content.
func (s *Server) validateBody(body []byte, requestID string) (*jobInput, *rejection) {
	object, err := strictjson.ParseObject(body)
	if err != nil {
		return nil, &rejection{reason: reasonInvalidBody, detail: detailInvalidBody}
	}
	members, err := object.Collect("videoUrl", "transcript", "title", "channelName", "description")
	if err != nil {
		return nil, &rejection{reason: reasonInvalidBody, detail: detailInvalidBody}
	}
	rawVideoURL, err := strictjson.RequiredString(members, "videoUrl")
	if err != nil || strings.TrimSpace(rawVideoURL) == "" {
		return nil, &rejection{reason: reasonInvalidBody, detail: detailInvalidBody}
	}
	rawTranscript, err := strictjson.RequiredString(members, "transcript")
	if err != nil || strings.TrimSpace(rawTranscript) == "" {
		return nil, &rejection{reason: reasonInvalidBody, detail: detailInvalidBody}
	}
	title, rej := optionalMember(members, "title")
	if rej != nil {
		return nil, rej
	}
	channelName, rej := optionalMember(members, "channelName")
	if rej != nil {
		return nil, rej
	}
	// Reject here, not later: the same violation found by validateTranscript
	// surfaces as ErrInvalidTranscript, which the job classifies as internal.
	if writer.CheckChannelName(channelName) != nil {
		return nil, &rejection{reason: reasonInvalidBody, detail: detailInvalidBody}
	}
	description, rej := optionalMember(members, "description")
	if rej != nil {
		return nil, rej
	}

	videoID, normalizedURL, err := transcript.ValidateVideoURL(rawVideoURL)
	if err != nil {
		return nil, &rejection{reason: reasonInvalidVideoURL, detail: detailInvalidVideoURL}
	}
	source, err := transcript.NewCopiedSource(transcript.CopiedInput{
		VideoID:     videoID,
		Raw:         rawTranscript,
		Title:       title,
		ChannelName: channelName,
		Description: description,
	})
	if err != nil {
		if errors.Is(err, transcript.ErrCopiedTranscriptFormat) {
			return nil, &rejection{reason: reasonInvalidTranscript, detail: detailInvalidTranscript, videoID: videoID}
		}
		return nil, &rejection{reason: reasonInternal, detail: detailInternalError, videoID: videoID}
	}
	return &jobInput{source: source, videoURL: normalizedURL, videoID: videoID, requestID: requestID}, nil
}

// optionalMember returns the string value of a declared optional member, or ""
// when it is absent. A value that is present but null or not a string is
// rejected; an empty string is accepted.
func optionalMember(members map[string]strictjson.Value, key string) (string, *rejection) {
	value, ok := members[key]
	if !ok {
		return "", nil
	}
	text, err := value.AsString()
	if err != nil {
		return "", &rejection{reason: reasonInvalidBody, detail: detailInvalidBody}
	}
	return text, nil
}
