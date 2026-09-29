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
| Design handoff document | `design_handoff.md` — review concerns on the requirements document that belong to design; the architecture document records how each item is handled (written by `fixpr.md`) |
| Implementation handoff document | `implementation_handoff.md` — review concerns on the architecture document that belong to implementation; the implementation plan records how each item is handled (written by `fixpr.md`) |
| Document status values | `draft` → `approved` |
| Document language | Japanese |
| Translation glossary | `docs/translation_glossary.md` |
| Translation language pair | Japanese (primary) ⇄ English *(reference only — `mktrans.md` determines direction from file extension, not this value)* |

### PR marker conventions

PR boundary markers embedded in the implementation plan use these labels:

- Section heading: `### PR-N 作成ポイント: <scope label in English>`
- Fields: `**対象ステップ**`, `**推奨タイトル**`, `**レビュー観点**`, `**実装モデル要件**`, `**判定理由**`
- Conventional commit scope format: `feat(<task-id>): <concise English title>`
- Implementation model tiers (`**実装モデル要件**` values): `frontier-required`, `frontier-recommended`, `standard` — assigned by `mkplan2.md` (step 4, "Model requirement"), read by `runplan.md` when choosing the implementing model

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
| Conditional-guide trigger | the feature invokes an external command (`yt-dlp`), handles API keys or the Slack Webhook URL, reads or writes the cache directory, sends data over the network (LLM API, Webhook), or embeds untrusted text (transcript, video metadata) into a prompt or a published article |
| Target client environments | Slack (`markdown` block via Incoming Webhook) |

### Domain examples referenced by commands

The methodology commands (`mkplan`, `runplan`) include illustrative examples
drawn from this project's domain (the transcript cache, `--dry-run`, yt-dlp
working files, error messages). They appear in:

- `runplan.md` step 5 ("State invariants before coding"): the video-ID cache
  key, `--dry-run` side-effect, and yt-dlp working-directory cleanup examples.
- `mkplan.md` step 5 (symbol-verification `rg` example) and step 6
  (`ErrNoSubtitles` before/after string-literal example).
