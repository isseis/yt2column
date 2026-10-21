package mergepr

import "github.com/isseis/yt2column/internal/cmdrunner"

// NewOSRunner returns the production Runner: the shared runner with its
// default, unbounded options.
func NewOSRunner() Runner { return cmdrunner.NewOS(cmdrunner.Options{}) }
