//go:build test

package main

import (
	"os"
	"testing"

	"github.com/isseis/yt2column/internal/fixprguard/fixprguardtest"
)

// isolation is set once by TestMain before any test runs. The process's working
// directory is an empty directory outside every git repository, so a test reads
// the repository (documents, sources) through isolation.PackageDir.
// fixtureTemplate (declared in test_helpers.go) is built here once too.
var isolation fixprguardtest.Isolation

func TestMain(m *testing.M) {
	os.Exit(fixprguardtest.Run(m, func(i fixprguardtest.Isolation) error {
		isolation = i
		tpl, err := fixprguardtest.BuildTemplate(i.TemplateRoot, buildTestRepoTemplate)
		if err != nil {
			return err
		}
		fixtureTemplate = tpl
		return nil
	}))
}
