---
name: mergepr
description: "Squash-merge a PR with a generated commit message and clean up its branch"
---

# /mergepr

This is a thin entry point for Antigravity CLI. The procedure lives in one
place, [`.claude/commands/mergepr.md`](../../../.claude/commands/mergepr.md); do not
look for it here, and follow these four steps in order:

1. Read [`.claude/commands/_context.md`](../../../.claude/commands/_context.md),
   the canonical source of every project-specific path, value, and gate.
2. Read [`.claude/commands/_lib/harness.md`](../../../.claude/commands/_lib/harness.md),
   follow its deterministic detection rule, and load the adapter it selects
   (in an Antigravity session,
   [`.claude/commands/_lib/harness-antigravity.md`](../../../.claude/commands/_lib/harness-antigravity.md)).
   Run every abstract operation the procedure names through that adapter.
3. Read [`.claude/commands/mergepr.md`](../../../.claude/commands/mergepr.md) and carry
   it out as written, including its argument specification, its
   authorization section, and its stop conditions.
4. Obey the prohibitions in [`AGENTS.md`](../../../AGENTS.md) and
   [`CLAUDE.md`](../../../CLAUDE.md) ("Tool Execution Safety"): never install
   or build an installed developer tool, never run a mutating `herdr`
   subcommand or a `git worktree` add, remove, or move, and never touch a
   record tag, merge, or call a real service without approval.
