// Command yt2column-wasmprobe is the probe server of the 0084 PoC. It lives on
// the PoC branch only and is never merged into main.
//
// It answers the probe calls of cmd/yt2column-wasm, which send llmhttp.Post
// through the browser's fetch with a dummy API key, so the adapter's defenses
// (no redirect, the body cap, the deadline) can be observed under fetch
// without sending a real key anywhere. It listens on 127.0.0.1:<port> (the
// origin inside host_permissions) and on localhost:<foreign-port> (an origin
// outside them) on both 127.0.0.1 and ::1, so the foreign port is reached
// whichever address the browser resolves localhost to.
//
// Every request is logged to standard output: the method, the listener, the
// path, the Origin header, whether the dummy key header arrived, and whether
// the probe body arrived. Run it with standard output redirected to a file
// outside the repository, for example
//
//	yt2column-wasmprobe > "$HOME/.local/share/yt2column/0084-measurements/probe.log"
//
// because the log is measurement material and is never committed.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// dummyKey is the x-api-key value cmd/yt2column-wasm sends to the probe. It is
// not a real key.
const dummyKey = "poc-0084-dummy-key"

// probeBody is the request body cmd/yt2column-wasm sends.
const probeBody = `{"probe":true}`

// largeBodyBytes is past llmhttp.MaxResponseBytes (8 MiB), so the adapter's
// body cap is what stops the read.
const largeBodyBytes = 9 << 20

// stallBodyPrefix is what /stall/body sends before it stops.
const stallBodyPrefix = `{"partial":`

// Defaults and limits of the server.
const (
	defaultPort        = 8787
	defaultForeignPort = 8788
	exitUsage          = 2
	readHeaderTimeout  = 10 * time.Second
	shutdownTimeout    = 2 * time.Second
	largeChunkBytes    = 64 << 10
	maxLoggedBody      = 1 << 10
)

type config struct {
	port        int
	foreignPort int
	allowCORS   bool
}

func main() {
	var cfg config
	var preflight string
	flag.IntVar(&cfg.port, "port", defaultPort, "port on 127.0.0.1 (inside host_permissions)")
	flag.IntVar(&cfg.foreignPort, "foreign-port", defaultForeignPort, "port on localhost, 127.0.0.1 and ::1 (outside host_permissions)")
	flag.StringVar(&preflight, "preflight", "deny", "the foreign target's answer to a CORS preflight: allow or deny")
	flag.Parse()
	switch preflight {
	case "allow":
		cfg.allowCORS = true
	case "deny":
		cfg.allowCORS = false
	default:
		fmt.Fprintln(os.Stderr, "yt2column-wasmprobe: -preflight must be allow or deny")
		os.Exit(exitUsage)
	}
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	if err := run(cfg, logger); err != nil {
		fmt.Fprintln(os.Stderr, "yt2column-wasmprobe:", err)
		os.Exit(1)
	}
}

// run serves until SIGINT or SIGTERM.
func run(cfg config, logger *log.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sameOrigin := "http://127.0.0.1:" + strconv.Itoa(cfg.port)
	foreignOrigin := "http://localhost:" + strconv.Itoa(cfg.foreignPort)
	listeners := []struct {
		name    string
		address string
		handler http.Handler
	}{
		{"same", net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.port)), sameHandler(logger, sameOrigin, foreignOrigin)},
		{"foreign-v4", net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.foreignPort)), foreignHandler(logger, "foreign-v4", cfg.allowCORS)},
		{"foreign-v6", net.JoinHostPort("::1", strconv.Itoa(cfg.foreignPort)), foreignHandler(logger, "foreign-v6", cfg.allowCORS)},
	}
	errs := make(chan error, len(listeners))
	servers := make([]*http.Server, 0, len(listeners))
	for _, l := range listeners {
		server := &http.Server{Addr: l.address, Handler: l.handler, ReadHeaderTimeout: readHeaderTimeout}
		servers = append(servers, server)
		go func() {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- fmt.Errorf("%s %s: %w", l.name, l.address, err)
			}
		}()
	}
	logger.Printf("start same=%s foreign=%s preflight-allow=%t", sameOrigin, foreignOrigin, cfg.allowCORS)

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errs:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	for _, server := range servers {
		_ = server.Shutdown(shutdownCtx)
	}
	logger.Printf("stop")
	return runErr
}

// sameHandler serves the 127.0.0.1 origin: the redirect sources, the
// same-origin redirect target, the large body, and the two stalls.
func sameHandler(logger *log.Logger, sameOrigin, foreignOrigin string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/redirect/same", func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, "same", r)
		http.Redirect(w, r, sameOrigin+"/target", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/redirect/foreign", func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, "same", r)
		http.Redirect(w, r, foreignOrigin+"/target", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/target", func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, "same", r)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	mux.HandleFunc("/large", func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, "same", r)
		chunk := make([]byte, largeChunkBytes)
		for i := range chunk {
			chunk[i] = 'a'
		}
		written := 0
		var writeErr error
		for written < largeBodyBytes {
			n, err := w.Write(chunk)
			written += n
			if err != nil {
				writeErr = err
				break
			}
		}
		logger.Printf("same /large written=%d of %d err=%v", written, largeBodyBytes, writeErr)
	})
	mux.HandleFunc("/stall/header", func(_ http.ResponseWriter, r *http.Request) {
		logRequest(logger, "same", r)
		start := time.Now()
		<-r.Context().Done()
		logger.Printf("same /stall/header closed by client after %d ms", time.Since(start).Milliseconds())
	})
	mux.HandleFunc("/stall/body", func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, "same", r)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, stallBodyPrefix)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		start := time.Now()
		<-r.Context().Done()
		logger.Printf("same /stall/body closed by client after %d ms", time.Since(start).Milliseconds())
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, "same", r)
		http.NotFound(w, r)
	})
	return mux
}

// foreignHandler serves the localhost origin's redirect target. It answers
// a CORS preflight as configured and logs whether the real request arrived.
func foreignHandler(logger *log.Logger, name string, allowCORS bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, name, r)
		if r.URL.Path != "/target" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodOptions {
			if !allowCORS {
				logger.Printf("%s preflight denied", name)
				w.WriteHeader(http.StatusForbidden)
				return
			}
			logger.Printf("%s preflight allowed", name)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "content-type, x-api-key")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if allowCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
}

// logRequest records one request without printing any header value except
// the Origin. The key is reported as dummy, other, or absent; the body as
// whether it equals the probe body.
func logRequest(logger *log.Logger, listener string, r *http.Request) {
	key := "absent"
	if values := r.Header.Values("x-api-key"); len(values) > 0 {
		key = "other"
		if len(values) == 1 && values[0] == dummyKey {
			key = "dummy"
		}
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxLoggedBody))
	bodyState := "absent"
	switch {
	case err != nil:
		bodyState = "read-error"
	case string(body) == probeBody:
		bodyState = "probe"
	case len(body) > 0:
		bodyState = "other"
	}
	logger.Printf("%s %s %s origin=%q key=%s body=%s", listener, r.Method, r.URL.Path, r.Header.Get("Origin"), key, bodyState)
}
