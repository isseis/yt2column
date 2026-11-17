//go:build js

// Package main is the release article generation module of the browser
// extension: the extension job (internal/extjob) with the production
// profile's Claude clients (internal/extjob/claudebackend), registered for
// JavaScript. Only a js build compiles it; it holds no logic of its own.
// It must never depend on the fake LLM or the development backend, which
// internal/extjob's dependency test checks.
package main

import (
	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/extjob/claudebackend"
	"github.com/isseis/yt2column/internal/extjob/jsmodule"
)

func main() {
	jsmodule.Serve(extjob.NewModule(claudebackend.New()))
}
