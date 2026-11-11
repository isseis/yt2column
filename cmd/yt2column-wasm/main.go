//go:build js && wasm

// Command yt2column-wasm is the WebAssembly entry point of the 0084 PoC. It
// lives on the PoC branch only and is never merged into main.
//
// It registers globalThis.yt2column with four functions (buildInfo, generate,
// longOutput, probe) and then calls globalThis.yt2columnReady, when the page
// defined it, to tell the page the functions can be called. Every function
// except buildInfo takes one JSON string and returns a Promise that resolves
// to one JSON string; none of them ever rejects. A call made while another
// call runs is refused, so the per-call header choice of browserTransport
// cannot leak into another call.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync/atomic"
	"syscall/js"
)

// busy is true while a generate, longOutput, or probe call runs. The page
// already refuses overlapping calls; this is the second line, because the
// transport's header choice is process-wide.
var busy atomic.Bool

func main() {
	transport := &browserTransport{base: http.DefaultTransport}
	http.DefaultTransport = transport

	api := js.Global().Get("Object").New()
	api.Set("buildInfo", js.FuncOf(func(js.Value, []js.Value) any {
		return mustJSON(readBuildInfo())
	}))
	api.Set("generate", asyncCall(func(request string) any {
		return runGenerate(transport, request)
	}, func(failure *failureReport) any {
		return generateResult{OK: false, Failure: failure, Stages: notRunStages()}
	}))
	api.Set("longOutput", asyncCall(func(request string) any {
		return runLongOutput(transport, request)
	}, func(failure *failureReport) any {
		return longOutputResult{Outcome: outcomeFailed, Failure: failure}
	}))
	api.Set("probe", asyncCall(func(request string) any {
		return runProbe(transport, request)
	}, func(failure *failureReport) any {
		return probeResult{Failure: failure}
	}))
	js.Global().Set("yt2column", api)

	if ready := js.Global().Get("yt2columnReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	// The functions stay callable only while main has not returned.
	select {}
}

// buildInfo is the build identity the page records with every measurement.
type buildInfo struct {
	GoVersion   string  `json:"goVersion"`
	VCSRevision *string `json:"vcsRevision"`
	VCSModified *bool   `json:"vcsModified"`
}

// readBuildInfo reads the Go version and the vcs settings stamped at build
// time. A setting the build did not stamp is null.
func readBuildInfo() buildInfo {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return buildInfo{GoVersion: "unknown"}
	}
	result := buildInfo{GoVersion: info.GoVersion}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision := setting.Value
			result.VCSRevision = &revision
		case "vcs.modified":
			modified := setting.Value == "true"
			result.VCSModified = &modified
		}
	}
	return result
}

// asyncCall wraps run as a JavaScript function that takes one string and
// returns a Promise of one JSON string. A missing or non-string argument, an
// overlapping call, and a panic inside run all resolve with the failure that
// onFailure shapes; the Promise is never rejected.
func asyncCall(run func(request string) any, onFailure func(*failureReport) any) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) any {
		var request string
		argumentOK := len(args) == 1 && args[0].Type() == js.TypeString
		if argumentOK {
			request = args[0].String()
		}
		executor := js.FuncOf(func(_ js.Value, promiseArgs []js.Value) any {
			resolve := promiseArgs[0]
			if !argumentOK {
				resolve.Invoke(mustJSON(onFailure(newFailure(kindInvalidInput, "the request must be one JSON string"))))
				return nil
			}
			if !busy.CompareAndSwap(false, true) {
				resolve.Invoke(mustJSON(onFailure(newFailure(kindInvalidInput, "another call is running"))))
				return nil
			}
			go func() {
				defer busy.Store(false)
				resolve.Invoke(mustJSON(recoverCall(run, request, onFailure)))
			}()
			return nil
		})
		defer executor.Release()
		return js.Global().Get("Promise").New(executor)
	})
}

// recoverCall runs run and turns a panic into the panic failure, so the Go
// runtime keeps running and the Promise still resolves.
func recoverCall(run func(string) any, request string, onFailure func(*failureReport) any) (result any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = onFailure(newFailure(kindPanic, fmt.Sprintf("recovered panic: %v", recovered)))
		}
	}()
	return run(request)
}

// mustJSON encodes a result. Every result type is plain data, so a failure
// here is a programming error; it still returns valid JSON for the page.
func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `{"ok":false,"failure":{"kind":"unknown","httpStatus":null,"message":"result encoding failed"}}`
	}
	return string(data)
}
