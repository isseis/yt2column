package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

// materialVideo is the synthetic video of the material tests.
const materialVideo = "abcdefghijk"

// writeMaterialRun writes one article's outputs as run.sh and the CLI leave
// them: the article file with its front matter, the artifacts, the review log,
// and the run's cache copy. The model names and config IDs sit in the front
// matter, the artifacts, and the review log's model lines.
func writeMaterialRun(t *testing.T, root string, c config, article string, withShorten bool) {
	t.Helper()
	title, body := "Title of "+article, "\nBody of "+article+".\n"
	work := filepath.Join(root, "work", c.ID, article)
	gen := testRecord(writer.StageGeneration, title, body, c.LLM.Model, writer.Digest("m"))
	genID := writeRecord(t, work, article, gen)
	input := genID
	if withShorten {
		sh := testRecord(writer.StageShorten, "Short "+title, "\nShort body.\n", c.LLM.Model, genID)
		input = writeRecord(t, work, article, sh)
	}
	writeRecord(t, work, article, testRecord(writer.StageReview, title, body, c.ReviewLLM.Model, input))
	writeFile(t, filepath.Join(root, "out", c.ID, article+".md"), "---\nsource: \"x\"\nmodel: \""+c.LLM.Model+
		"\"\nmodel_version: \"\"\nreview_model: \""+c.ReviewLLM.Model+"\"\nreview_model_version: \"\"\nestimated_cost_usd: \"0.1000\"\n---\n\n# "+
		title+"\n"+body+"\nsource line\n")
	writeFile(t, filepath.Join(root, "review", c.ID, article+".md"), "# Review log\n\n- Article: "+title+
		"\n- Source: x\n- Review model: "+c.ReviewLLM.Model+"\n- Review model version: (none)\n- Revisions: 0\n")
	cache := filepath.Join(root, "runcache", c.ID, article)
	vid := videoOf(article)
	writeFile(t, filepath.Join(cache, vid+".current"), "a")
	writeFile(t, filepath.Join(cache, vid+".a", vid+".ja.json3"), `{"events":[{"tStartMs":61000,"dDurationMs":1000,"segs":[{"utf8":"spoken words"}]}]}`)
	writeFile(t, filepath.Join(cache, vid+".a", vid+".info.json"), `{"id":"`+vid+`","title":"Video title","channel":"Channel","description":"Description"}`)
}

// TestBuildMaterial writes every article under a name that hides its config,
// with its title and body, its shorten output when one exists, and its review
// log; the mapping restores every name; and no material file holds a model
// name or config ID.
func TestBuildMaterial(t *testing.T) {
	root := t.TempDir()
	configs := mustConfigs(t)
	cs, _ := configs.byID("C-sol")
	as, _ := configs.byID("A-sol")
	for _, a := range []string{materialVideo + "-1", materialVideo + "-2"} {
		writeMaterialRun(t, root, cs, a, a == materialVideo+"-1")
		writeMaterialRun(t, root, as, a, false)
	}
	out := filepath.Join(root, "score", "1")
	reverse := func(n int) ([]int, error) {
		p := make([]int, n)
		for i := range p {
			p[i] = n - 1 - i
		}
		return p, nil
	}
	q := materialRequest{Root: root, OutDir: out, Configs: []config{cs, as}, Videos: []string{materialVideo}, Shuffler: reverse}
	if err := buildMaterial(q); err != nil {
		t.Fatal(err)
	}
	mapping, err := os.ReadFile(filepath.Join(out, mappingName))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(mapping)), "\n")[1:]
	if len(lines) != 4 {
		t.Fatalf("mapping = %q; want four articles", mapping)
	}
	seen := map[string]bool{}
	for _, l := range lines {
		f := strings.Split(l, "\t")
		seen[f[1]+" "+f[2]] = true
		art, err := os.ReadFile(filepath.Join(out, "material", materialVideo, f[0]+"."+materialArticle))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(art), "# Title of "+f[2]) || !strings.Contains(string(art), "Body of "+f[2]) {
			t.Errorf("%s: article material = %q", f[0], art)
		}
		_, err = os.Stat(filepath.Join(out, "material", materialVideo, f[0]+"."+materialShorten))
		if hasShorten := err == nil; hasShorten != (f[1] == "C-sol" && f[2] == materialVideo+"-1") {
			t.Errorf("%s: shorten material present = %v", f[0], hasShorten)
		}
	}
	if len(seen) != 4 {
		t.Errorf("the mapping does not restore every article: %v", seen)
	}
	if _, err := os.Stat(filepath.Join(out, "material", mappingName)); err == nil {
		t.Errorf("the mapping must sit outside the material directory")
	}
	err = filepath.WalkDir(filepath.Join(out, "material"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path) //nolint:gosec // a file the test wrote
		if err != nil {
			return err
		}
		for _, w := range []string{"C-sol", "A-sol", "gpt-6.1-sol", "claude-opus-5-5", "Review model"} {
			if strings.Contains(string(data), w) {
				t.Errorf("%s contains %q", path, w)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := os.ReadFile(filepath.Join(out, "material", materialVideo, materialTx))
	if err != nil || !strings.Contains(string(tx), "01:01 spoken words") {
		t.Errorf("transcript = %q, %v", tx, err)
	}
}

// TestBuildMaterialRefusesLeak stops when a text would carry a model name the
// material must hide.
func TestBuildMaterialRefusesLeak(t *testing.T) {
	root := t.TempDir()
	configs := mustConfigs(t)
	cs, _ := configs.byID("C-sol")
	writeMaterialRun(t, root, cs, materialVideo+"-1", false)
	q := materialRequest{Root: root, OutDir: filepath.Join(root, "score", "1"), Configs: []config{cs}, Videos: []string{materialVideo}, Shuffler: cryptoShuffle}
	if err := buildMaterial(q); err != nil {
		t.Fatalf("the same input without a leak: %v", err)
	}
	writeFile(t, filepath.Join(root, "review", "C-sol", materialVideo+"-1.md"), "# Review log\n\n- Note: made by gpt-6.1-sol\n")
	q.OutDir = filepath.Join(root, "score", "2")
	if err := buildMaterial(q); !errors.Is(err, errLeak) {
		t.Errorf("err = %v, want errLeak", err)
	}
}
