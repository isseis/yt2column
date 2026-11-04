package server

import (
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/isseis/yt2column/internal/articlecache"
)

// maxInFlight is the number of generation slots, which bound the article
// generations in progress: a generation job holds one while it generates. A
// generation start that cannot take a slot is refused at once; there is no
// queue.
const maxInFlight = 2

// maxReading is the number of reading slots: the generation starts that read
// and validate a body at once. A body is up to maxBodyBytes and is held with
// its parse while it is checked, so at most two bodies, about 8 MiB, are held
// at once.
const maxReading = 2

// acquireSlot takes one generation slot without waiting. It reports false
// when every slot is taken.
func (s *Server) acquireSlot() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

// releaseSlot gives back one generation slot.
func (s *Server) releaseSlot() {
	<-s.slots
}

// acquireReading takes one reading slot without waiting. It reports false
// when every slot is taken.
func (s *Server) acquireReading() bool {
	select {
	case s.reading <- struct{}{}:
		return true
	default:
		return false
	}
}

// releaseReading gives back one reading slot.
func (s *Server) releaseReading() {
	<-s.reading
}

// startHandler serves a generation start. It checks, in order, the method, a
// reading slot (refused without reading the body when none is free), the body,
// and then the key's record in the cache:
// a running or done key is answered with its key whatever the slots, a new or
// failed key starts a generation when a generation slot is free, and every
// other case is too_many_requests.
//
// Ownership: the handler holds the reading slot only while it reads and checks
// the body, and the generation slot and the Run from Begin until the
// s.jobs.Go call in startGeneration hands both to the job. Its recover fails
// the Run and releases the generation slot when it still holds them, so a
// panic never leaves a record running or a slot taken.
func (s *Server) startHandler(w http.ResponseWriter, r *http.Request, requestID string) {
	responded := false
	var run *articlecache.Run // non-nil while the handler owns it and its slot
	defer func() {
		p := recover()
		if p == nil {
			return
		}
		if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) && run == nil {
			// The handler's own abort after a body-read failure: let net/http
			// close the connection without writing a response.
			panic(p)
		}
		s.logger.Error("request handler panicked", "requestId", requestID, "panic", p, "stack", string(debug.Stack()))
		if run != nil {
			run.Fail(articlecache.FailureInternal)
			s.releaseSlot()
		}
		if !responded {
			if err := writeRejected(w, requestID, reasonInternal, true); err != nil {
				s.logger.Error("response write failed", "requestId", requestID, "error", err)
			}
		}
	}()

	if rej := checkMethod(r, http.MethodPost); rej != nil {
		s.reject(w, requestID, *rej)
		responded = true
		return
	}
	if !s.acquireReading() {
		s.reject(w, requestID, rejection{reason: reasonTooManyRequests, detail: detailReadSlot})
		responded = true
		return
	}
	input, rej := s.readStartRequest(w, r)
	if rej != nil {
		if rej.readFailed {
			// A cut connection, a timeout, or a malformed chunked body: no
			// response, only a log line, and an abort so net/http writes no
			// implicit 200.
			s.logger.Warn("request body read failed", "requestId", requestID, "error", rej.err)
			responded = true
			panic(http.ErrAbortHandler)
		}
		s.reject(w, requestID, *rej)
		responded = true
		return
	}

	subject := s.keyer.Subject(input.key)
	key := subject.Key()
	begun := s.cache.Begin(subject, articlecache.StartPlain, s.acquireSlot)
	run = begun.Run
	if begun.Evicted != nil {
		s.logEviction(begun.Evicted)
	}
	var outcomeCode string
	switch begun.Outcome {
	case articlecache.BeginStarted, articlecache.BeginRestarted:
		outcomeCode = "started"
		if begun.Outcome == articlecache.BeginRestarted {
			outcomeCode = "restarted"
		}
		if s.beforeGenerationJob != nil {
			s.beforeGenerationJob()
		}
		// The job owns the Run and the slot from here. Start it before
		// writing 202, so no response can be observed without its record
		// running.
		s.startGeneration(run, key, *input)
		run = nil
	case articlecache.BeginExisting:
		outcomeCode = "existing"
	case articlecache.BeginSameVideo:
		// Begin reports another article of the same video only for
		// StartInquiry, and this handler passes StartPlain, so the real Cache
		// never returns this; no test reaches it. Should it come back, the
		// cache is unchanged, and answering with that article would skip the
		// generation the start asked for, so the start is refused instead.
		s.logger.Error("generation start got a same-video article for a plain start", "requestId", requestID, "key", key.String())
		s.reject(w, requestID, rejection{reason: reasonInternal, detail: detailInternalError})
		responded = true
		return
	case articlecache.BeginNoSlot:
		s.reject(w, requestID, rejection{reason: reasonTooManyRequests, detail: detailGenerationSlot})
		responded = true
		return
	default:
		// BeginFull, the zero value: no record could be dropped to make room.
		s.reject(w, requestID, rejection{reason: reasonTooManyRequests, detail: detailCacheFull})
		responded = true
		return
	}
	if err := writeAcceptedKey(w, requestID, key.String()); err != nil {
		s.logger.Info("accepted response write failed", "requestId", requestID, "key", key.String(), "error", err)
	}
	responded = true
	s.logger.Info("generation start accepted", "requestId", requestID, "key", key.String(), "videoId", input.videoID, "outcome", outcomeCode)
}

// readStartRequest reads and checks a generation start's body while the
// caller holds a reading slot, and releases the slot when it returns, a panic
// included.
func (s *Server) readStartRequest(w http.ResponseWriter, r *http.Request) (*jobInput, *rejection) {
	defer s.releaseReading()
	return s.readRequest(w, r)
}

// logEviction writes the one line for a record the cache dropped to make room.
func (s *Server) logEviction(e *articlecache.Eviction) {
	attrs := []any{"key", e.Key.String(), "state", jobStateCode(e.State)}
	if e.State == articlecache.JobDone {
		attrs = append(attrs, "publish", publishStateCode(e.Publish))
	}
	attrs = append(attrs, "age", e.Age)
	s.logger.Info("article record evicted", attrs...)
}

// statusHandler serves a status request: it checks the method, that the
// request has no body (refused without reading one), and that keyPart is a
// key in its one accepted form, then answers with the key's record. It changes
// no record, calls no LLM or Webhook, and writes no log line on success:
// clients poll it every few seconds.
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request, requestID, keyPart string) {
	responded := false
	defer func() {
		p := recover()
		if p == nil {
			return
		}
		s.logger.Error("request handler panicked", "requestId", requestID, "panic", p, "stack", string(debug.Stack()))
		if !responded {
			if err := writeRejected(w, requestID, reasonInternal, true); err != nil {
				s.logger.Error("response write failed", "requestId", requestID, "error", err)
			}
		}
	}()

	if rej := checkMethod(r, http.MethodGet); rej != nil {
		s.reject(w, requestID, *rej)
		responded = true
		return
	}
	key, rej := checkKeyRequest(r, keyPart)
	if rej != nil {
		s.reject(w, requestID, *rej)
		responded = true
		return
	}
	data, err := renderStatus(requestID, s.webhookConfigured(), s.cache.Lookup(key))
	if err != nil {
		s.logger.Error("status response build failed", "requestId", requestID, "key", key.String(), "error", err)
		s.reject(w, requestID, rejection{reason: reasonInternal, detail: detailInternalError})
		responded = true
		return
	}
	responded = true
	if err := writeBody(w, http.StatusOK, data, nil); err != nil {
		s.logger.Info("status response write failed", "requestId", requestID, "key", key.String(), "error", err)
	}
}

// checkKeyRequest checks a request that names a key and carries no body: the
// body first, refused without reading it, then the key part, which must be a
// key in its one accepted form. Checking the body first is what lets an
// invalid_key rejection keep the connection: no body is left unread. The key
// part is never echoed: it is not a key the server issued.
func checkKeyRequest(r *http.Request, keyPart string) (articlecache.Key, *rejection) {
	if rej := checkNoBody(r); rej != nil {
		return articlecache.Key{}, rej
	}
	key, err := articlecache.ParseKey(keyPart)
	if err != nil {
		return articlecache.Key{}, &rejection{reason: reasonInvalidKey, detail: detailInvalidKey}
	}
	return key, nil
}

// checkNoBody returns the invalid_body rejection for a request that carries a
// body or may: a non-zero Content-Length or any Transfer-Encoding. The body is
// not read, so the rejection closes the connection.
func checkNoBody(r *http.Request) *rejection {
	if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
		return &rejection{reason: reasonInvalidBody, detail: detailInvalidBody, bodyUnread: true}
	}
	return nil
}
