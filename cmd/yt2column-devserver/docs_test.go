//go:build test

package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/llm/devfake"
)

// developmentDocPath is the development guide from this package's directory.
const developmentDocPath = "../../docs/dev/guides/development.md"

// TestDevelopmentDocsDevServer pins the values of the development guide's
// section on the development server to the code: the argument names, every
// value the arguments accept, the longest delay, the model name, the title
// marker, and the make target. The values come from this package's argument
// names and from devfake (except the make target, which names a Makefile
// rule), so a value that changes in the code fails this test until the guide
// follows, and a guide that states a wrong value fails it too.
func TestDevelopmentDocsDevServer(t *testing.T) {
	development := doctestutil.ReadDoc(t, developmentDocPath)
	section := doctestutil.SectionOutsideBlocks(development, doctestutil.DevServerHeading)
	if section == "" {
		t.Fatalf("development.md has no %q section", doctestutil.DevServerHeading)
	}

	maxDelay := int(devfake.MaxDelay / time.Second)
	var documented []string
	for _, name := range []string{argArticle, argFail, argFailStep, argDelay} {
		documented = append(documented, "`-"+name+"`")
	}
	for _, article := range devfake.Articles() {
		documented = append(documented, "`"+article.String()+"`")
	}
	for _, failure := range devfake.Failures() {
		documented = append(documented, "`"+failure.String()+"`")
	}
	for _, failAt := range devfake.FailAts() {
		documented = append(documented, "`"+failAt.String()+"`")
	}
	documented = append(documented,
		"`0` から `"+strconv.Itoa(maxDelay)+"` まで",
		"`"+devfake.ModelName+"`",
		"`"+devfake.TitleMarker+"`",
		"`make build-dev`",
	)

	for _, want := range documented {
		if !strings.Contains(section, want) {
			t.Errorf("development.md %q does not mention %s", doctestutil.DevServerHeading, want)
		}
	}
}
