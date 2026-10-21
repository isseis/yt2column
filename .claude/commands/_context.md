---
description: "Internal: project-specific values the commands depend on (not a command)"
---

# Project Context for Claude Commands

This file is the single source of truth for project-specific configuration values
the commands in this directory depend on. Commands reference the **names** defined
here (e.g. "the task root", "the build checks") so those values are configured in
one place rather than scattered across command files.

**To port these commands to another project:**
1. Edit this file — update all three sections (Process convention, Tech-stack
   convention, Domain-specific) for the new project.
2. Edit the command bodies for Domain-specific content — the illustrative examples
   embedded in `mkplan.md` and `runplan.md` (e.g. the video-ID cache key,
   `--dry-run`, `ErrNoSubtitles`) are drawn from this project's domain and must
   be replaced or removed. See the "Domain-specific" section below for where each
   example lives.
3. Edit or replace the guide documents this file points to.

The values are grouped by the layer they belong to, so you can see at a glance
what changes under which condition:

- **Process convention** — changes only if the new project adopts a different
  requirements → architecture → plan → PR workflow.
- **Tech-stack convention** — changes only if the new project is not Go, or uses
  different build tooling / source layout.
- **Domain-specific** — always changes; specific to this project.

---

## Process convention

| Name | Value |
|---|---|
| Task root | `docs/tasks/` |
| Task identification guide | `docs/dev/developer_guide/task_identification.md` |
| Requirements process guide | `docs/dev/developer_guide/requirements_process.md` |
| Test organization guide | `docs/dev/developer_guide/test_organization.md` |
| Mermaid reference guide | `docs/dev/developer_guide/mermaid_reference.md` |
| Package reference guide | `docs/dev/developer_guide/package_reference.md` |
| Requirements document | `01_requirements.md` |
| Architecture document | `02_architecture.md` |
| Implementation plan document | `03_implementation_plan.md` |
| Design handoff document | `design_handoff.md` — design-level review concerns raised on the requirements document; the architecture document records how each item is handled (written by `fixpr.md`) |
| Implementation handoff document | `implementation_handoff.md` — implementation-level review concerns raised on the requirements or architecture document; the implementation plan records how each item is handled (written by `fixpr.md`) |
| Document status values | `draft` → `approved`; `withdrawn` for every document of a task withdrawn after review (see the requirements process guide, "Document Status Format") |
| Feature branch name | `<GitHub user name>/<slug>-<NN>`, where `<slug>` is the task directory name with each `_` replaced by `-`, and `-<NN>` is appended once: a two-digit sequence number starting at `01` (e.g. `isseis/0027-remove-context-menu-01`). `_lib/task-pr-handoff.md` Part A chooses `NN` for the commands that open a task PR, and `runpr.md` step 5 names the next branch after a merge |
| Reviewer ping | After a task-document command (`mkreq.md`, `mkarch.md`, `mkplan.md`, `mkplan2.md`) or `runplan.md` opens its PR (`_lib/task-pr-handoff.md` Part B), notify the human reviewer with the `PushNotification` tool (the PR author cannot be requested as a GitHub reviewer of their own PR) and repeat the PR URL and the next action in the final report, which is the only channel when the tool is unavailable or the notification is not sent |
| PR merge method | Squash merge, run with `/mergepr`. The squash commit is the only commit of a PR that reaches `main`, so its message carries what CLAUDE.md requires commit messages to record |
| Document language | Japanese |
| Translation glossary | `docs/translation_glossary.md` |
| Translation language pair | Japanese (primary) ⇄ English *(reference only — `mktrans.md` determines direction from file extension, not this value)* |

### PR marker conventions

PR boundary markers embedded in the implementation plan use these labels:

- Section heading: `### PR-N 作成ポイント: <scope label in English>`
- Fields: `**対象ステップ**`, `**推奨タイトル**`, `**レビュー観点**`, `**実装モデル要件**`, `**判定理由**`
- `**推奨タイトル**`: the title `runplan.md` opens the PR with. It is a PR title, not a squash subject (`mergepr.md` writes that at the merge); its form is in "PR title and squash subject" below
- Implementation model tiers (`**実装モデル要件**` values): `frontier-required`, `frontier-recommended`, `standard` — assigned by `mkplan2.md` (step 4, "Model requirement"; `mkplan.md` runs it as its step 8.5), read by `runplan.md` when choosing the implementing model and shown in the merge question when `/runpr` offers the next `/runplan`

### PR title and squash subject

A PR title is written in Japanese, like everything else on GitHub
(`CLAUDE.md`, "Language"). The squash commit is the only commit of a PR that
reaches `main`, so its subject stays in English; `mergepr.md` step 2 writes it
from the PR material and reads only the prefix from the PR title. This section
is the one place that defines these forms: the commands refer to it by name
instead of restating them.

| Item | Form |
|---|---|
| PR title | `<type>(<scope>): <Japanese summary>`: an accepted prefix that has a `<scope>`, followed by a summary in Japanese. A PR title a command writes never uses the scope-less `<type>: ` form; that form exists only among accepted prefixes and squash subjects. The summary may contain command names, file names, and identifiers spelled as they are, without backquotes (see "Passing a title to `gh`") |
| Accepted prefix | The prefix table below |
| Task-document PR title | `docs(<task-id>): <Japanese summary>`, where the summary says what the documents the PR adds or changes contain. `mkreq.md`, `mkarch.md`, `mkplan.md`, and `mkplan2.md` use it through `_lib/task-pr-handoff.md` (PR_TEXT) |
| Plan PR title | The prefix is the `推奨タイトル`'s own when that value starts with an accepted prefix that has a `<scope>`, and `<type>(<task-id>): ` otherwise; the summary is in Japanese. `runplan.md` step 5a (with a PR marker) and step 8 (without one) give the procedure |
| `推奨タイトル` | The PR title `runplan.md` opens the PR with, not a squash subject. A value written now has the form `<type>(<task-id>): <Japanese summary>`, with `<type>` from the prefix table. A value an older plan wrote in another form stays valid input for `runplan.md` step 5a, which never rewrites the plan |
| Squash subject | `<type>(<scope>): <English summary> (#<number>)` or `<type>: <English summary> (#<number>)`, made only of printable ASCII characters. `mergepr.md` step 2 decides it and checks it as "Checking a squash subject" says |

The accepted prefix:

| Part | Accepted value |
|---|---|
| Whole | `<type>(<scope>): ` or `<type>: `, all ASCII, with exactly one ASCII space (U+0020) right after the `:` |
| `<type>` | `feat`, `fix`, `docs`, `refactor`, `test`, `perf`, `build`, `ci`, `chore`, `style`, `revert` |
| `<scope>` | One or more ASCII lowercase letters, digits, and `-` |
| Anything else | Not accepted. Do not repair it; treat the title as having no prefix. Examples: a `!` after the type or the scope; an uppercase letter or a Japanese character; no space after the `:`, or two or more; empty parentheses; a full-width `：`, `（`, `）`, digit, or letter; a full-width space (U+3000) or a no-break space (U+00A0) after the `:`; a non-ASCII look-alike such as the Cyrillic `е`, `‐`, or `−` |

To add a `<type>`, change the `<type>` row above and the expression in
"Checking a squash subject" together.

#### Passing a title to `gh`

`gh pr create` and `gh pr edit` have no flag that reads a title from a file.
Write the title with the Write tool to a file in the scratchpad (outside the
tracked tree), and pass it by command substitution:
`--title "$(cat <title-file>)"`. The substituted text is not expanded again
inside the double quotes, so a `$`, a backquote, a `"`, or a `\` in the title
reaches GitHub as written instead of being expanded by the shell (which could
publish an environment variable's value as the title). Never write the title
text itself inside `--title "..."`.

Separately, a PR title and a `推奨タイトル` never contain backquotes: a
`推奨タイトル` sits inside a backquoted code span in the PR marker, which a
backquote would break.

#### Checking a squash subject

After writing the subject file and before the approval question, `mergepr.md`
step 2 checks that the file holds exactly one line (one line ending in a
newline, or one line without one: no second line), and that this line matches
the expression below in the C locale. A CR or any other byte outside printable
ASCII makes it fail. With `R` set to the expression:

```bash
[ "$(awk 'END { print NR }' <subject-file>)" = 1 ] && LC_ALL=C grep -Eqx "$R" <subject-file>
```

Squash subject regular expression:

```
(feat|fix|docs|refactor|test|perf|build|ci|chore|style|revert)(\([a-z0-9-]+\))?: [!-~]([ -~]*[!-~])? \(#[0-9]+\)
```

### Plan PR

A PR is a **plan PR** when, in the tree of the PR's head, a task's
`03_implementation_plan.md` has the PR's URL on a `PR を作成した` line
(`runplan.md` writes it, at a PR checkpoint or, for a plan without PR markers,
in its single-PR record). Its task ID is the number of the task directory that
holds that plan. It is the task's **final PR** when no `### PR-N 作成ポイント`
marker after that line is unchecked. `runpr.md` step 2 and `mergepr.md` step 2
use this definition.

- **Tree.** `runpr.md` reads its checkout of the PR's head branch, which its
  step 1 brought up to date. `mergepr.md` reads the tree of the `headRefOid`
  that `prepare` recorded, the commit `merge` is pinned to with
  `--match-head-commit`, so the tree judged is the tree merged.
- **Match.** Count only a line where the whole URL matches: looking for
  `.../pull/41` does not count a line with `.../pull/410`.
- **No match**: not a plan PR.
- **Stop**, report, and do not guess when the matching lines are in the plans
  of two or more tasks.
- **Stop**, report, and do not guess when the tree cannot be read (a `git`
  failure, which is not the same as no match: `git grep` exits 1 for no match
  and above 1 for a failure).

---

## Tech-stack convention

| Name | Value |
|---|---|
| Build checks (run after edits) | `make fmt` (Go only) → `make test` → `make lint` |
| Dead-code check | `make deadcode` |
| Green gate (must pass before PR) | `make test && make lint` |
| Source layout | `cmd/`, `internal/` |
| Cross-package test helpers | `testutil/` |
| Package-internal test helpers | `test_helpers.go` / `test_helpers_<category>.go` with `//go:build test` |
| Source-language rule | Go comments, identifiers, and string literals must be English |

---

## Domain-specific (replace wholesale when porting)

| Name | Value |
|---|---|
| Project overview | `docs/dev/project_overview.md` |
| Conditional security guide | `docs/dev/security.md` |
| Conditional-guide trigger | the feature invokes an external command (`yt-dlp`), handles API keys or the Webhook URL (`SLACK_WEBHOOK_URL`), reads or writes the cache directory, sends data over the network (LLM API, Webhook), or embeds untrusted text (transcript, video metadata) into a prompt or a published article |
| Target client environments | Mattermost (Slack-compatible Incoming Webhook, Markdown `text` posted with `silent`). Slack is best effort and not a verification target |

### Domain examples referenced by commands

The methodology commands (`mkplan`, `runplan`) include illustrative examples
drawn from this project's domain (the transcript cache, `--dry-run`, yt-dlp
working files, error messages). They appear in:

- `runplan.md` step 5 ("State invariants before coding"): the video-ID cache
  key, `--dry-run` side-effect, and yt-dlp working-directory cleanup examples.
- `mkplan.md` step 5 (symbol-verification `rg` example) and step 6
  (`ErrNoSubtitles` before/after string-literal example).
