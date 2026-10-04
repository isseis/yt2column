package writer

// maxPromptBytes caps each expanded prompt (system and user separately).
// The prompt expansion that enforces it replaces the provisional Write (plan
// step 3-3); until then only the prompts/README.md guard reads it.
const maxPromptBytes = 1048576 //nolint:unused // enforced by the expansion added with the real Write; 1 MiB

// templateData is the only value passed to a prompt template. Its fields are
// the four values a template may reference; the template checks derive the
// allowed field names from this type.
type templateData struct {
	Title       string
	ChannelName string
	Description string
	Transcript  string
}
