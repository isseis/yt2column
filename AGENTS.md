# AGENTS.md

Guidance for automated agents, including code reviewers, working on this repository. The project's conventions, design principles and testing rules are in [CLAUDE.md](CLAUDE.md); follow them. The review-relevant guides are [requirements_process.md](docs/dev/developer_guide/requirements_process.md) (approval gates, AC traceability), [test_organization.md](docs/dev/developer_guide/test_organization.md), and [security.md](docs/dev/security.md).

## Review guidelines

- Report defects by their worst outcome when the code runs: broken behavior, a security hole, a failing build, or a test that cannot fail for its stated reason.
- Do not re-raise a finding that has been answered in its review thread unless the code it concerns has changed since the answer.
- An implementation commit that predates the `approved` status of its governing document (`01_requirements.md`, `02_architecture.md`, or `03_implementation_plan.md`) is a finding, regardless of what the commit history says about being told to proceed.
- For a diff that invokes an external command (`yt-dlp`), handles a secret (`DEEPSEEK_API_KEY`, `SLACK_WEBHOOK_URL`), reads or writes the cache, sends data over the network, or embeds untrusted text in a prompt or a published article, review it against `docs/dev/security.md`'s attack-vector list; a finding there is valid even when no acceptance criterion mentions it.

### Static guards in `internal/pipeline/pipeline_test.go`

These guards detect mistakes that developers and agents of this repository make while writing ordinary code. They are not a sandbox against code written to get around them. The threat model is in `docs/tasks/0001_pipeline_skeleton/02_architecture.md` §7.1.

- Do not report bypasses that need deliberately contrived code, for example a type alias declared only to route around a rule, `reflect`, or `unsafe`. (The guards themselves use `reflect`; that use is not a finding either.)
- The guards list the forms they allow and reject the rest. A form the guards reject although ordinary code would legitimately write it (a false positive) is worth reporting. An accepted form is worth reporting only if ordinary code would plausibly write it by mistake; say why.
- The guards pin design contracts across packages, not behavior: the exported field sets of the common types, the `context.Context`-first methods of the interfaces, the English contract clauses in the interface docs, `secret.Secret`'s exported method set, and the `//go:build test` first line of every `testutil` file. A legitimate contract change is expected to update its guard in the same change (including `TestFakesCarryBuildTag`'s fixed file count when a fake is added); report a contract change that leaves its guard stale, not a guard that the same change updated.
- Before reporting a package as untested, check the task's `02_architecture.md` test strategy (§7): some packages intentionally carry no `_test.go`, and their contracts are pinned by the guards instead.
