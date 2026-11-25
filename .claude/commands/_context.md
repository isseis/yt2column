---
description: "Internal: project-specific values the commands depend on (not a command)"
---

# Project Context for the Development Commands

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
| Task identification guide | `docs/dev/conventions/task_identification.md` |
| Requirements process guide | `docs/dev/conventions/requirements_process.md` |
| Test organization guide | `docs/dev/conventions/test_organization.md` |
| Mermaid reference guide | `docs/dev/conventions/mermaid_reference.md` |
| Package reference guide | `docs/dev/conventions/package_reference.md` |
| Requirements document | `01_requirements.md` |
| Architecture document | `02_architecture.md` |
| Implementation plan document | `03_implementation_plan.md` |
| Design handoff document | `design_handoff.md` — design-level items for the architecture document, which records how each item is handled: review concerns raised on the requirements document (written by `fixpr.md`, as ordinary items, or as addressed items in a three-document PR), and design decisions confirmed in a simple-process `/mkreq` run, moved there when the task returns or switches to the full process (written by `runpr.md` or `mkreq.md`, as confirmed items; on a return `runpr.md` also turns addressed items into ordinary items and writes the concerns of unresolved threads on the deleted documents as ordinary items). The kinds of item and their forms are in the requirements process guide, "Design handoff" |
| Implementation handoff document | `implementation_handoff.md` — implementation-level items for the implementation plan, which records how each item is handled: review concerns raised on the requirements or architecture document (written by `fixpr.md`, as ordinary items, or as addressed items in a three-document PR), and plan decisions confirmed in a simple-process `/mkreq` run, moved there when the task returns or switches to the full process (written by `runpr.md` or `mkreq.md`, as confirmed items; on a return `runpr.md` also turns addressed items into ordinary items and writes the concerns of unresolved threads on the deleted documents as ordinary items). The kinds of item and their forms are in the requirements process guide, "Design handoff" |
| Document status values | `draft` → `approved`; `withdrawn` for every document of a task withdrawn after review (see the requirements process guide, "Document Status Format") |
| Task process record | The `Process` row of the requirements document's status table, which records whether the task takes the full process or the simple process. Its values, the states of the record, and who writes it are defined only in the requirements process guide, "Document Status Format" ("Process record"); the rules of the simple process are in its "Simple Process" section |
| Draft issue label | `draft` — a label marks a draft issue only when its name is exactly `draft` in bytes. An open issue with that label is a draft issue: do not start a task from it. `/mkreq` stops in step 1, and the `chief` tool creates no workspace for it (`internal/chief/issue.go`). A closed issue is a closed issue whatever its labels. Removing the label makes the issue a regular issue |
| Feature branch name | `<GitHub user name>/<slug>-<NN>`, where `<slug>` is the task directory name with each `_` replaced by `-`, and `-<NN>` is appended once: a two-digit sequence number starting at `01` (e.g. `isseis/0027-remove-context-menu-01`). `_lib/task-pr-handoff.md` Part A chooses `NN` for the commands that open a task PR, and `runpr.md` step 5 names the next branch after a merge |
| Reviewer ping | After a task-document command (`mkreq.md`, `mkarch.md`, `mkplan.md`, `mkplan2.md`) or `runplan.md` opens its PR (`_lib/task-pr-handoff.md` Part B), notify the human reviewer with the `notify-user` operation of `_lib/harness.md` (the PR author cannot be requested as a GitHub reviewer of their own PR) and repeat the PR URL and the next action in the final report, which is the only channel when the harness has no notification tool or the notification is not sent |
| PR merge method | Squash merge, run with `/mergepr`. The squash commit is the only commit of a PR that reaches `main`, so its message carries what CLAUDE.md requires commit messages to record, and every editorial correction line of the PR's commits, as `mergepr.md` says |
| Document language | Japanese — the language of the task documents and the guides (`CLAUDE.md`, "Document language", principle (a)); instruction files and conventions are English |
| Translation glossary | `docs/translation_glossary.md` — also the authority for the Japanese terms of a PR title and a PR description: read it before writing either, use its terms, and never use a term it marks as deprecated (旧称) or a rendering it says not to use. A term the text needs but the glossary lacks is not invented silently: name it in the final report, for the user to add to the glossary |
| Translation language pair | `README.md` (Japanese, primary) → `README.en.md` (English), the only pair `mktrans.md` translates; it stops and asks on any other file *(reference only — `mktrans.md` defines the pair itself, not from this value)* |

### PR marker conventions

PR boundary markers embedded in the implementation plan use these labels:

- Section heading: `### PR-N 作成ポイント: <scope label in English>`
- Fields: `**対象ステップ**`, `**推奨タイトル**`, `**レビュー観点**`, `**実装モデル要件**`, `**判定理由**`, `**自動送信**`
- `**推奨タイトル**`: the source `runplan.md` step 5a writes the PR title from. It is a PR title, not a squash subject (`mergepr.md` writes that at the merge); its form is in "PR title and squash subject" below
- Implementation model requirement (`**実装モデル要件**`), in five parts:
  1. **Values.** `high`, `standard+high-review`, and `low`. `mkplan2.md`
     step 4, "Model requirement", assigns them (`mkplan.md` runs it as its
     step 8.5, and `mkreq.md` runs it in the simple process; `mkplan.md` step
     8.5 also writes the value of a single-phase plan, and `_lib/simple-process.md`,
     "Writing the three documents", reaches it through `mkplan.md`); the meaning of
     each value, when it applies, and how its `**判定理由**` is written are
     defined there only, not here. They are read by `runplan.md` (its
     guidance before a phase group), by `_lib/high-review.md` (entry 1, which
     decides whether the high-model review runs), by `runpr.md` (the merge
     question of step 4 and the `/clear` guidance of step 5), and by the
     `chief` tool (`internal/chief/task.go`, which chooses the `/runplan` row
     of "Command routing" below from it).
  2. **Where the value is.** In a plan with PR markers, the PR marker's
     section: from its `### PR-N 作成ポイント` heading to the next heading of
     any level. In a plan without PR markers (a single-phase plan), the
     section inside §3 (the `## 3.` chapter) whose heading text, without its
     number, is `PR 構成` (`### 3.2 PR 構成` and `### 3.2. PR 構成` both
     count), up to the next heading of any level. Lines inside a fenced code
     block are not read, as headings or as values.
  3. **How it is read.** In that section, find the line that starts in its
     first column with `**実装モデル要件**: ` and take the rest of the line
     with the leading and trailing white space removed:
     - when it equals one of the three values, it is that value;
     - when it equals an old value of the table in part 4, it is read as that
       table's value; show both the old value and the value it is read as;
     - otherwise it is an **unreadable value**: never read it as another
       value by guessing, and show the text as written with the note that it
       is unreadable;
     - when there is no such line (including a single-phase plan with no
       `PR 構成` section), there is **no value**.

     `_lib/high-review-decision.sh` is the executable form of parts 2 to 4
     for entry 1 of `_lib/high-review.md`; it reads part 4 and
     "Harness model mapping" from `origin/main`'s copy of this file. The
     `chief` tool reads parts 2 to 4 in Go (`internal/chief/task.go`), also
     with part 4 from `origin/main`, and a test runs both readings on the same
     plans.
  4. **Old values.** Plans written before these values existed carry the
     old values, which are read as follows. This table is the only place
     that maps them. It may be removed once no plan has an old value in a PR
     marker whose PR has not been created yet.

     | Old value | Read as |
     |---|---|
     | `opus` | `high` |
     | `sonnet+opus-review` | `standard+high-review` |
     | `sonnet` | `low` |
     | `frontier-required` | `high` |
     | `frontier-recommended` | `standard+high-review` |
     | `standard` | `low` |

  5. **What a reading recommends.** This table names, for each reading,
     the row of "Command routing" below whose agent and model a Claude Code or
     OpenCode `/runplan` session starts with. `runplan.md` and `runpr.md` take
     the agent and model from that row, by naming it rather than repeating its
     values; `chief` maps each reading to the same row in its code
     (`internal/chief`) and reads the row's values from the table. The
     `(Claude Code, low)` row of "Harness model mapping" below is for
     lightweight subagents (`fixpr` Phase 6, etc.) only and is never used as
     the recommended implementation model for a plan PR. Antigravity sessions
     are outside `chief`'s scope and have no rows in "Command routing";
     `runplan.md` and `runpr.md` read Antigravity's recommended model for each
     reading from the `Antigravity` rows of "Harness model mapping" below
     (`high` → `high`, `standard+high-review` → `standard`, `low` → `low`) and
     note that Antigravity implementation records cannot satisfy `all-high`
     because Antigravity sessions cannot self-verify their model ID (so for
     `high`, `standard+high-review`, and unreadable values, the high-model
     review runs unless a valid done record already covers the branch). The
     high-model review column is decided in detail by entry 1 of
     `_lib/high-review.md`.

     | Reading | Command routing row | High-model review |
     |---|---|---|
     | `high` | `runplan-high` | decided by entry 1: runs when a commit is not covered by a self-verifying `high` implementation record or a valid done record (Antigravity records cannot satisfy `all-high`) |
     | `standard+high-review` | `runplan-standard+high-review` | decided by entry 1, as for `high` |
     | `low` | `runplan-low` | does not run |
     | unreadable value | none (show the text and that it is unreadable) | treated as for `standard+high-review` |
     | no value | none (show that there is no value) | does not run |

- Auto-send field (`**自動送信**`): whether the `chief` tool may send the
  `/runplan` that creates the PR by itself, after the previous PR's merge.
  Its values are `auto` and `hold`; `mkplan2.md` step 4, "Auto-send",
  assigns them and says when `hold` applies. It is read from the place of
  the implementation model requirement (its part 2) by the line rule of its
  part 3: the line that starts in its first column with `**自動送信**: `,
  the rest of the line with the leading and trailing white space removed.
  A section without such a line, and a single-phase plan with no `PR 構成`
  section, read as `auto`, so a plan written before the field existed is
  sent as before. Any other value, two such lines, two places that could
  hold the field, a value line inside a fenced code block of the place, or
  a code fence the plan never closes is unreadable, and `chief` sends nothing by itself for that PR; there are
  no old values. Only `chief` reads the field (`internal/chief/task.go`);
  `_lib/high-review-decision.sh` does not.

### Command routing

The agent and model each task command is sent to, or a session for it is
started with. This table is the only place that holds them: `runplan.md` and
`runpr.md` show them from here, and the `chief` tool
(`internal/chief/routing.go`) reads this table from `origin/main`'s copy of
this file to choose the pane and the model it sends a command to. To move a
command to another agent or model, edit its row here; nothing else changes.

This section stays outside "PR marker conventions", whose two-cell rows
`_lib/high-review-decision.sh` reads as old values.

| Row | Command | Agent | Model |
|---|---|---|---|
| `task-documents` | `/mkreq`, `/mkarch`, `/mkplan` | Claude Code | `opus` |
| `runplan-high` | `/runplan` for the requirement `high` | Claude Code | `opus` |
| `runplan-standard+high-review` | `/runplan` for the requirement `standard+high-review` | Claude Code | `sonnet` |
| `runplan-low` | `/runplan` for the requirement `low` | OpenCode | `DeepSeek V4.1 Flash 1017 OpenCode Go` |
| `runplan-takeover` | `/runplan` handed back to open the PR after an OpenCode run stopped before it | Claude Code | `sonnet` |
| `runpr` | `/runpr` | OpenCode | `DeepSeek V4.1 Flash 1017 OpenCode Go` |
| `mergemain` | `/mergemain` | Claude Code | `opus` |

- **Row** is the name the other tables and `chief` use for the row; `chief`
  reads every row, so each of the seven rows above must stay, once.
- **The `mergemain` row's agent is Claude Code only.** Before `chief` sends
  `/mergemain`, it is to check the human's input after the last task command
  and keep the pane's report, both read from Claude Code's transcript. This
  row is the one exception to "edit its row here; nothing else changes":
  with OpenCode in it, the table is still read, and `chief` is to send no
  `/mergemain`.
- **Agent** is `Claude Code` or `OpenCode`.
- **Model** is, for Claude Code, the argument of `/model` (`opus` or
  `sonnet`); for OpenCode, the whole text its status line shows between the
  first and the last ` · ` (after the only ` · ` for a model with no
  reasoning effort), printable ASCII words separated by single spaces.
- A table `chief` cannot read (a missing or repeated row, another column, an
  agent or a model outside these sets) makes it send nothing.

### Harness model mapping

The concrete model, launch mechanism, and model-ID verification pattern for
each harness and role. This table is the single source of truth read by
`_lib/high-review-decision.sh` and `_lib/review-records.sh` (from
`origin/main`'s copy of this file) and by the harness adapters. It sits in its
own section outside "PR marker conventions" and "Command routing", with
`Harness` as its first column header, so `ParseOldValues`, `ParseRouting`, and
`contextRow` do not scan it.

| Harness | Role | Launch | Model | Verification pattern |
|---|---|---|---|---|
| `Claude Code` | `high` | `agent` | `opus` | `claude-opus-*` |
| `Claude Code` | `standard` | `agent` | `sonnet` | `claude-sonnet-*` |
| `Claude Code` | `low` | `agent` | `haiku` | `claude-haiku-*` |
| `Claude Code` | `cross-check` | `agent` | `fable` | `claude-fable-*` |
| `Antigravity` | `high` | `swarm` | `gemini-4-neon-high` | `gemini-4-neon-high` |
| `Antigravity` | `standard` | `swarm` | `gemini-4-neon-medium` | `gemini-4-neon-medium` |
| `Antigravity` | `low` | `swarm` | `gemini-4-hydrogen-medium` | `gemini-4-hydrogen-medium` |
| `Antigravity` | `cross-check` | `swarm` | `claude-opus-5-5-medium` | `claude-opus-5-5-medium` |
| `OpenCode` | `low` | `none` | `DeepSeek V4.1 Flash 1017 OpenCode Go` | `-` |

Strict grammar (validated fail-closed across the whole table by the decision scripts):

- Exactly one unfenced `### Harness model mapping` heading in this file, with
  the fixed five-column header `| Harness | Role | Launch | Model | Verification pattern |`
  and separator `|---|---|---|---|---|`, and every data cell wrapped in a
  single code span (no embedded backquote or `|`).
- **Harness**: `Claude Code`, `Antigravity`, or `OpenCode`.
- **Role**: `high`, `standard`, `low`, or `cross-check`.
- **Launch**: `agent`, `swarm`, or `none`.
- **Model**: non-empty printable ASCII string (not whitespace-only).
- **Verification pattern**: `-` (only when `Launch` is `none`); otherwise
  either a literal exact match `^[a-z0-9][a-z0-9.-]{0,63}$` or a
  trailing-wildcard prefix `^[a-z0-9][a-z0-9.-]{0,62}\*$`.
- Each `(Harness, Role)` pair may appear at most once.
- **Non-regex matching**: patterns are never evaluated as regular expressions
  or unquoted globs (`.` matches only a literal `.`). A literal pattern
  matches by exact byte equality; a trailing `*` pattern strips the literal
  prefix and matches the remaining suffix against `^[a-z0-9.-]{1,48}$`.
- `_lib/high-review-decision.sh` validates the whole table before any early
  return and matches `Implementation model:` records only against rows with
  `Role` = `high` and `Launch` = `agent` (self-verifying harnesses).
  `_lib/review-records.sh` validates the whole table in its partial-review
  mode (`cross-check` and its compatibility alias `fable`), collects non-`-`
  rows for both `Role` = `cross-check` (for partial-review run records) and
  `Role` = `high` (for merge check run records), and fails closed if either
  set is empty.

### PR title and squash subject

A PR title is written in Japanese, like everything else on GitHub
(`CLAUDE.md`, "Language"). The squash commit is the only commit of a PR that
reaches `main`, so its subject stays in English; `mergepr.md` step 2 writes it
from the PR material and reads only the prefix from the PR title. This section
is the one place that defines these forms: the commands refer to it by name
instead of restating them. The one exception is the PR marker template that
`mkplan2.md` inserts into a plan, which spells out the `推奨タイトル` form.

| Item | Form |
|---|---|
| PR title | `<type>(<scope>): <Japanese summary>`: an accepted prefix that has a `<scope>`, followed by a summary in Japanese. A PR title a command writes never uses the scope-less `<type>: ` form; that form exists only among accepted prefixes and squash subjects. The summary may contain command names, file names, and identifiers spelled as they are, without backquotes (see "Passing a title to `gh`") |
| Accepted prefix | The prefix table below |
| Task-document PR title | `docs(<task-id>): <Japanese summary>`, where the summary says what the documents the PR adds or changes contain. `mkreq.md`, `mkarch.md`, `mkplan.md`, and `mkplan2.md` use it through `_lib/task-pr-handoff.md` (PR_TEXT) |
| Plan PR title | The prefix is the `推奨タイトル`'s own when that value starts with an accepted prefix that has a `<scope>`, and `<type>(<task-id>): ` otherwise; the summary is in Japanese. `runplan.md` step 5a (with a PR marker) and step 8 (without one) give the procedure |
| `推奨タイトル` | The source `runplan.md` step 5a writes the PR title from: a PR title, not a squash subject. A value written now has the form `<type>(<task-id>): <Japanese summary>`, with `<type>` from the prefix table. A value an older plan wrote in another form stays valid input for `runplan.md` step 5a, which never rewrites the plan |
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
Write the title to a file in the scratchpad (outside the tracked tree) with the
`file-io` and `scratchpad` operations of `_lib/harness.md`, and pass it by
command substitution:
`--title "$(cat "<title-file>")"`. The substituted text is not expanded again
inside the double quotes, so a `$`, a backquote, a `"`, or a `\` in the title
reaches GitHub as written instead of being expanded by the shell (which could
publish an environment variable's value as the title). Never write the title
text itself inside `--title "..."`.

Separately, a PR title and a `推奨タイトル` never contain backquotes: a
`推奨タイトル` sits inside a backquoted code span in the PR marker, which a
backquote would break.

#### Checking a squash subject

After writing the subject file and before the approval question, `mergepr.md`
step 2 runs the commands below on it. They pass only when the file holds
exactly one line (one line ending in a newline, or one line without one: no
second line), when every byte is a newline or printable ASCII (a CR, a tab, a
byte-order mark, or any other byte makes them fail), and when the line matches
the expression below in the C locale. `command grep` bypasses a shell function
named `grep`, which may run a different grep. Set `R` to the expression, in
single quotes, and `f` to the subject file:

```bash
R='<the squash subject regular expression>'
f='<subject-file>'
[ "$(awk 'END { print NR }' "$f")" = 1 ] &&
	[ "$(LC_ALL=C tr -d '\n -~' <"$f" | wc -c)" -eq 0 ] &&
	LC_ALL=C command grep -Eqx -- "$R" "$f"
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
marker after that line is unchecked. `runpr.md` step 2, `mergepr.md` step 2,
and `fixpr.md` Phase 1 (through `_lib/simple-process.md`, "Identifying the PR
kind") use this definition.

- **Tree.** `runpr.md` reads its checkout of the PR's head branch, which its
  step 1 brought up to date (for a PR its step 1 finds merged, whose head
  branch is gone, `origin/main`). `fixpr.md` reads its work tree's `HEAD`,
  which `begin` checked is clean. `mergepr.md` reads the tree of the `headRefOid`
  that `prepare` recorded, the commit `merge` is pinned to with
  `--match-head-commit`, so the tree judged is the tree merged.
- **Match.** Count only a line where the whole URL matches: looking for
  `.../pull/41` does not count a line with `.../pull/410`. For example,
  `git grep -nE 'PR を作成した.*https://github\.com/<owner>/<repo>/pull/41([^0-9]|$)' <rev> -- 'docs/tasks/*/03_implementation_plan.md'`.
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
| Conditional-guide trigger | the feature invokes an external command (`yt-dlp`), handles API keys (`DEEPSEEK_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`), reads or writes the cache directory, sends data over the network (LLM API), or embeds untrusted text (transcript, video metadata) into a prompt or into the article the tool displays, copies, or saves |
| Target client environments | Mattermost (pasting the Markdown copied in the Viewer). Slack is best effort and not a verification target |
| Automated reviewer list | `coderabbitai`, `sourcery-ai` — the automated reviewers (CodeRabbit and Sourcery AI), in the GraphQL `author.login` form (without `[bot]`). `fixpr.md` and `runpr.md` pass every name on this list to `fixprguard`, each as its own `--reviewer`. A Bot's review-thread comments are trusted when its login is any name on this list |
| Awaited PR reviewer | `coderabbitai` — the automated reviewer for manual PR reviews (`runpr.md` steps 3 and 4), in the GraphQL `author.login` form. The REST `user.login` form is this name followed by `[bot]`, which `runpr.md` step 3 filters CodeRabbit's observed status posts by |
| PR review request | `@coderabbitai review` — the body of the comment that asks the awaited PR reviewer for a manual PR review. Posted only when the user chooses it in `runpr.md` step 4; it is the sole text a command may post without passing the mention check |
| Rejected mentions | `@codex`, `@coderabbitai`, `@sourcery-ai`, `@sourceryai` — mentions forbidden in any text a command posts from the user's account other than `PR review request`, compared case-insensitively in ASCII; Sourcery AI answers both of its forms, so both are listed; to name a reviewer, write `Codex`, `CodeRabbit`, or `Sourcery AI` without the `@`. The reason: a mention posted from the user's account can trigger an automated reviewer. Two placeholders are built from this row's values (never repeat the literals elsewhere): `<mention-check>` expands to `grep -qiF` with one `-e '<mention>'` per value in this row followed by `--` (so `<mention-check> <file>` exits `1` only when clear); `<rejected-mentions>` expands to the JSON array of this row's values for `runpr.md` step 3's `--jq` filters |
| Guard tool | `fixprguard`, installed with `make install-fixprguard` ([fixprguard_guide.md](../../docs/dev/guides/fixprguard_guide.md)). `fixpr.md` and `runpr.md` call it for every author judgment, review-thread fetch, scope and git-state check, the fix round's commit and push, and every write to a review thread or the PR description; `runpr.md` also calls it to read the state of the repository's interaction limits and manual PR reviews. In `fixpr.md` Phase 4.5 (the local CLI review of the fix), `check` compares the git state with the baseline at each point the phase names before it runs git, and `push-review` pushes the phase's commits; those commits get no scope check |

### Domain examples referenced by commands

The methodology commands (`mkplan`, `runplan`) include illustrative examples
drawn from this project's domain (the transcript cache, `--dry-run`, yt-dlp
working files, error messages). They appear in:

- `runplan.md` step 5 ("State invariants before coding"): the video-ID cache
  key, `--dry-run` side-effect, and yt-dlp working-directory cleanup examples.
- `mkplan.md` step 5 (symbol-verification `rg` example) and step 6
  (`ErrNoSubtitles` before/after string-literal example).
