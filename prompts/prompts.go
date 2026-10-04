// Package prompts embeds the default prompt templates of the article writer.
// It only returns their text; internal/writer checks them with the same rules
// it applies to an override file.
package prompts

import _ "embed" // the templates are embedded with go:embed

// The embedded templates are unexported so that no other package can replace
// them; the checks in internal/writer always see the committed files.
var (
	//go:embed system.tmpl
	system string

	//go:embed user.tmpl
	user string
)

// System returns the embedded default system prompt template.
func System() string {
	return system
}

// User returns the embedded default user prompt template.
func User() string {
	return user
}
