//go:build js

// Package jsmodule registers an extjob.Module's four JSON entry points on the
// JavaScript global object, for the two article generation modules
// (cmd/yt2column-wasm and cmd/yt2column-wasm-dev), which differ only in the
// Backend they give the Module. It holds the syscall/js glue and nothing
// else: every request is parsed and every result is built by extjob, which
// the native tests cover. Only a js build compiles this package.
package jsmodule

import (
	"context"
	"syscall/js"

	"github.com/isseis/yt2column/internal/extjob"
)

// GlobalName is the property of globalThis that holds the namespace object
// with the four functions: checkSettings, prepare, generate, and buildInfo.
const GlobalName = "yt2column"

// Serve registers m's entry points under GlobalName and blocks forever, so
// the Go program keeps running to serve later calls. It never returns.
//
// Contract: every function takes JSON strings and returns a JSON string
// (checkSettings and generate return a Promise of one) that extjob built, and
// none ever rejects its Promise or throws. An argument that is not a string
// is passed to extjob as the empty string, which every entry point rejects as
// a malformed request, so a wrong call is reported in the result rather than
// repaired. A progress callback that is not a function is not called.
func Serve(m *extjob.Module) {
	namespace := js.Global().Get("Object").New()
	namespace.Set("checkSettings", js.FuncOf(func(_ js.Value, args []js.Value) any {
		request := stringArg(args, 0)
		return promise(func() string {
			return m.CheckSettingsJSON(context.Background(), request)
		})
	}))
	namespace.Set("prepare", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return m.PrepareJSON(stringArg(args, 0))
	}))
	namespace.Set("generate", js.FuncOf(func(_ js.Value, args []js.Value) any {
		request := stringArg(args, 0)
		onProgress := progressCallback(args, 1)
		return promise(func() string {
			return m.GenerateJSON(context.Background(), request, onProgress)
		})
	}))
	namespace.Set("buildInfo", js.FuncOf(func(js.Value, []js.Value) any {
		return m.BuildInfoJSON()
	}))
	js.Global().Set(GlobalName, namespace)
	select {}
}

// stringArg returns args[i] when it is a JavaScript string, and "" otherwise.
func stringArg(args []js.Value, i int) string {
	if i >= len(args) || args[i].Type() != js.TypeString {
		return ""
	}
	return args[i].String()
}

// progressCallback returns a function that passes each progress event to
// args[i], or nil when args[i] is not a JavaScript function.
func progressCallback(args []js.Value, i int) func(string) {
	if i >= len(args) || args[i].Type() != js.TypeFunction {
		return nil
	}
	callback := args[i]
	return func(event string) { callback.Invoke(event) }
}

// promise returns a JavaScript Promise that resolves with run's result. run
// runs on its own goroutine, because a function JavaScript calls must return
// before Go can wait on a network call.
func promise(run func() string) js.Value {
	executor := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve := args[0]
		go func() { resolve.Invoke(run()) }()
		return nil
	})
	defer executor.Release()
	return js.Global().Get("Promise").New(executor)
}
