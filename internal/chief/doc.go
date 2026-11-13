// Package chief holds the decisions and side effects of the chief developer
// tool: it reads the Herdr workspaces of this repository, classifies them, and
// sends only a fixed set of typed inputs to their agents.
//
// The files whose names start with "screen" are the only place that chooses
// behavior by inspecting the text an agent draws on its pane. Every other file
// chooses behavior only by the types those files return, and an agent screen
// that matches no shape read from a real sample is never read more widely.
package chief
