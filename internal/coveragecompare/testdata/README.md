# Real command output samples

These files are parts of the output the coverage comparison tool reads, taken
from this repository at commit `4c1eeb43fdd4f0e41bbbee0eac0a8205b43cbf30` on
2026-11-02 with `go1.27.1` (darwin/arm64) and node `v24.21.0`. The parser tests
read them so a change in a command's output shape is caught against real text,
not only against text written by hand.

- `go-profile.txt`: the first 10 lines of the profile
  `go test -tags test -json -coverpkg=./internal/slackwebhook
  -coverprofile=<file> ./internal/slackwebhook` wrote. It holds the `mode: set`
  line and 9 blocks. `grep -vc '^mode:'` on the full profile the run wrote
  reports 34 blocks.
- `go-test-json.txt`: the first 6 lines of that run's `-json` output plus the
  top-level `TestValidURL` pass line. It holds one finished top-level test and
  no failure; `grep -c '"Action":"pass".*"Test":"Test'` on the full output
  reports 23 finished tests.
- `lcov.txt`: the first two `SF` sections of the report
  `node --test --experimental-test-coverage --test-reporter=lcov
  --test-reporter-destination=<file> --test-reporter=tap
  --test-reporter-destination=<file> "test/**/*.test.ts"` wrote. The excerpt
  holds 2 sections and 15 `FN` records; `grep -c '^FN:'` and `grep -c '^SF:'` on
  the full report report 415 and 44.
- `tap.txt`: the 39 lines of that run's TAP report that end with the summary.
  They hold the `# tests` value 2006, `# fail` 0, and `# cancelled` 0.

Checked when the samples were taken: they hold no home directory path and no
absolute path. The module path in the Go samples contains the project owner's
GitHub name, `isseis`, but that is the repository's own import path, not a value
naming whoever captured the output.
