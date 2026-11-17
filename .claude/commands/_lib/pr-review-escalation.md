---
description: "Internal: when to request a further Codex PR review after the first (not a command)"
---

# Shared pattern: PR review escalation

Codex does the PR review, and its allowance is rationed: it is shared with the
user's other ChatGPT use, and its size is not published, so after the review
made when the PR opens, a PR review is requested only when it is needed. Codex
is set to review only when the PR is created, never on a push (`CLAUDE.md`).
Changes after that are reviewed locally with the local CLI review — the
CodeRabbit CLI, or Codex CLI when it fails (`fixpr.md` Phase 4.5,
`_lib/local-cli-review.md`). The CodeRabbit CLI's allowance is separate from
the PR review's, but the fallback Codex CLI spends the same allowance as the PR
review.

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

Request a PR review when the changes in LAST_REVIEWED..HEAD do any of these.
The criteria that apply decide what the calling command does with the
answer: a change under criterion 4 gets a PR review; for a change under the
other criteria only, the calling command decides from the local CLI review's
records whether to request one.

1. **Change behavior or a contract.** Observable behavior changes — error
   handling, an exported API, a wire format, configuration, the cache format —
   as opposed to the shape of the code.
2. **Cross a module boundary.** The correctness of the change depends on how
   other packages use it. A mechanical rename is not this.
3. **Touch a requirement or a design decision.** They change or reinterpret an
   acceptance criterion, a decision recorded in a task document, or what a test
   asserts.
4. **Touch a trust boundary, security, or durability.** `yt-dlp` invocation,
   secrets, network access, cache writes and their consistency, handling of
   untrusted text, redaction. `docs/dev/security.md` and
   `docs/dev/cache_consistency.md` say what falls under this; read them when
   the diff touches any of those areas. The development tools' trust boundary
   counts too: a step or code that handles a secret, sends data off the
   machine, posts to GitHub, types into another agent's pane, deletes a file
   or a ref, or has another side effect that cannot be undone. So does a
   change to a review gate itself: this file's criteria; `runpr.md` step 3's
   rules that turn this file's answer into a decision, and its comparison of
   the PR's `headRefOid` with the local head; `_lib/review-records.sh` and the
   way the command documents run it from `origin/main`; the sections that
   define the forms of the review record lines and how they are read; and
   `fixprguard`. A change to another part of a command document is judged by
   the other criteria.
5. **Add a mechanism the earlier review never saw.** A new code path, concurrency,
   or state that did not exist at LAST_REVIEWED.

None applies to: mechanical changes (rename, move, formatting); wording in
documents (a change to what a review gate does is not wording); tests added in
an existing pattern; deleted code, unless it removes part of a review gate or
of a trust boundary's checks (criterion 4); a local fix that applies a finding
as stated once the local review has converged.

Size is not a criterion. A large rename touches none of the five; a three-line
change to a cache write or to a review gate touches the fourth.

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
