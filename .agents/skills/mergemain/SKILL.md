---
name: mergemain
description: "Merge origin/main into a PR branch, resolve allowed conflicts, run checks, and push"
---

# /mergemain

This is a thin entry point for Antigravity CLI. The procedure lives in one
place, [`.claude/commands/mergemain.md`](../../../.claude/commands/mergemain.md); do not
look for it here, and follow these four steps in order:

1. Read [`.claude/commands/_context.md`](../../../.claude/commands/_context.md),
   the canonical source of every project-specific path, value, and gate.
2. Read [`.claude/commands/_lib/harness.md`](../../../.claude/commands/_lib/harness.md),
   follow its deterministic detection rule, and load the adapter it selects
   (in an Antigravity session,
   [`.claude/commands/_lib/harness-antigravity.md`](../../../.claude/commands/_lib/harness-antigravity.md)).
   Run every abstract operation the procedure names through that adapter.
3. Read [`.claude/commands/mergemain.md`](../../../.claude/commands/mergemain.md) and carry
   it out as written, including its argument specification, its
   authorization section, and its stop conditions. Antigravity does not
   substitute `$ARGUMENTS` (or `$1`): wherever the definition uses one,
   read it as the arguments the user gave when invoking this skill, and
   as empty only when the user gave none.
4. Obey the prohibitions in [`AGENTS.md`](../../../AGENTS.md) and
   [`CLAUDE.md`](../../../CLAUDE.md) ("Tool Execution Safety"): never install
   or build an installed developer tool, never run a mutating `herdr`
   subcommand or a `git worktree` add, remove, or move, and never touch a
   record tag (except through the `numreserve task` and `numreserve adr`
   calls `CLAUDE.md` allows without approval), merge, or call a real service
   without approval.
