//go:build js

package main

import (
	"os"

	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/extjob/devbackend"
	"github.com/isseis/yt2column/internal/extjob/jsmodule"
)

// main registers the module's functions for the embedded scenario. When the
// scenario does not parse, it writes the reason to stderr (the browser's
// console) and returns without registering anything, so the page reports a
// failure to load the module rather than running a scenario nobody chose.
func main() {
	scenario, err := embeddedScenario()
	if err != nil {
		reportRejected(os.Stderr, err)
		return
	}
	jsmodule.Serve(extjob.NewModule(devbackend.New(scenario)))
}
