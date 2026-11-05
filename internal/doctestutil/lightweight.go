//go:build test

package doctestutil

// Headings of the lightweight process's documents that the guard tests find a
// section by: the conditions table of
// docs/dev/conventions/requirements_process.md, and the spec comment template
// and the listing of .claude/commands/_lib/lightweight-spec.md. These documents
// are English, so the headings are too.
const (
	// LightweightConditionsHeading is the requirements_process.md heading the
	// conditions table follows directly.
	LightweightConditionsHeading = "#### Mechanically checked conditions"
	// LightweightSpecFormatHeading is the lightweight-spec.md section that
	// holds the spec comment template.
	LightweightSpecFormatHeading = "## Spec comment format"
	// LightweightListingHeading is the lightweight-spec.md section that holds
	// the listing of the user's marker comments.
	LightweightListingHeading = "## Listing own marker comments"
)
