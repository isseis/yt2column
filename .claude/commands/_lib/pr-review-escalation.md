---
description: "Internal: when to request a further Codex PR review after the first (not a command)"
---

# Shared pattern: PR review escalation

Codex does the PR review, and its allowance is rationed: it is shared with the
user's other ChatGPT use, and its size is not published, so after the review
made when the PR opens, a PR review is requested only when it is needed. Codex
is set to review only when the PR is created, never on a push (`CLAUDE.md`).
Changes after that are reviewed locally with the `coderabbit` CLI (`fixpr.md`
Phase 4.5, `_lib/local-cli-review.md`), whose allowance is separate.

A further PR review is requested in exactly two cases: the user asks for it
(`runpr.md` step 4 offers it), or this file's judgement says the changes since
the last PR review are of a kind the local review cannot stand in for. `runpr.md`
step 3 invokes this file.

## Inputs the calling command must supply

- **PR** — the PR number.
- **LAST_REVIEWED** — the commit Codex's last completed PR review was for, as
  `runpr.md` step 3, "LAST_REVIEWED", defines it. When it is none, `runpr.md`
  takes the decision as `ask` without running this file.
- **HEAD** — the PR's current head commit.
- **FINDINGS** — the review threads handled since LAST_REVIEWED (their topics and
  how each was closed), or "none".

## Criteria

Request a PR review when the changes in LAST_REVIEWED..HEAD do any of these:

1. **Change behavior or a contract.** Observable behavior changes — error
   handling, an exported API, a wire format, configuration, the cache format —
   as opposed to the shape of the code.
2. **Cross a module boundary.** The correctness of the change depends on how
   other packages use it. A mechanical rename is not this.
3. **Touch a requirement or a design decision.** They change or reinterpret an
   acceptance criterion, a decision recorded in a task document, or what a test
   asserts.
4. **Touch security or durability.** `yt-dlp` invocation, secrets, network
   access, cache writes and their consistency, handling of untrusted text,
   redaction. `docs/dev/security.md` and `docs/dev/cache_consistency.md` say
   what falls under this; read them when the diff touches any of those areas.
5. **Add a mechanism the earlier review never saw.** A new code path, concurrency,
   or state that did not exist at LAST_REVIEWED.

Do not request one for: mechanical changes (rename, move, formatting); wording in
documents; tests added in an existing pattern; deleted code; a local fix that
applies a finding as stated once the local review has converged.

Size is not a criterion. A large rename touches none of the five; a three-line
change to a cache write touches the fourth.

## Procedure

1. If LAST_REVIEWED is not an ancestor of HEAD (`git merge-base --is-ancestor`),
   the history was rewritten and the range cannot be judged: the decision is
   `ask`, with that as the reason. If LAST_REVIEWED equals HEAD, nothing has
   changed: the decision is `skip`.
2. Spawn one subagent with the Agent tool and `model: "opus"`. Its prompt is
   self-contained: the criteria above copied verbatim, the commands to read the
   range (`git log --oneline LAST_REVIEWED..HEAD`, `git diff LAST_REVIEWED..HEAD`
   with the actual SHAs), FINDINGS, and the instruction to read the two
   documents named in criterion 4 when it applies. It must decide from what the
   changes do, not from how many lines or files they touch, and reply with only:

   ```json
   {"decision": "request" | "skip" | "ask", "criteria": [1], "reason": "one or two sentences"}
   ```

   `request`: at least one criterion clearly applies. `skip`: none does.
   `ask`: a criterion might apply and the diff does not settle it. Tell the
   subagent that a wrong `ask` costs the human one question, while a wrong
   `skip` lets an unreviewed change merge, so it should choose `ask` when in
   doubt.
3. Return the decision and its reason to the calling command, which reports the
   reason whatever the decision is.
