Squash-merge PR `$ARGUMENTS` (or, if no argument was given, the PR for the
current branch) with a generated commit message, then clean up the branch.

> **Project context**: the merge method and commit-message rules are in
> `.claude/commands/_context.md` ("Process convention", "PR merge method") and
> `CLAUDE.md`. This repository squash-merges every PR, so the squash commit is the
> only commit of the PR that reaches `main`.

Invoking this command authorizes the network operations it lists (`git fetch`,
`gh` queries, `git pull`) but **not the merge itself**: step 4 asks the user to
approve the commit message first (CLAUDE.md, "Tool Execution Safety").

Work in order. If any step fails, stop and report; do not work around it.

1. **Identify the PR.**
   - `gh pr view $ARGUMENTS --json number,title,body,state,headRefName,headRefOid,baseRefName,url`
   - Stop unless `state` is `OPEN`.
   - If the current branch is the PR's head branch, check that `git status --porcelain`
     is empty and that local `HEAD` equals `headRefOid`. Uncommitted or unpushed work
     would be lost when the branch is deleted in step 6, so stop and report instead of
     pushing on your own.

2. **Check CI.** Run `gh pr checks <number> --watch --fail-fast`. Wait for pending
   checks. Stop and report if any check fails. Skipped checks (e.g. the lint job of a
   docs-only change) are not failures.

3. **Draft the squash commit message.** Run `git fetch origin`, then inspect the
   change against the up-to-date base, not a possibly stale local `main`:
   - `git log --no-show-signature --format='%h %s%n%n%b' origin/<baseRefName>..<headRefOid>`
     for every commit message in full;
   - `git diff --stat origin/<baseRefName>...<headRefOid>`;
   - read individual file diffs (`git diff origin/<baseRefName>...<headRefOid> -- <path>`)
     only where the log and stat are not enough. Do not dump the whole diff: a large
     PR does not fit in context.

   Write the message in English:
   - **Subject**: `<type>(<scope>): <summary> (#<number>)`, conventional-commit
     style as in `git log origin/<baseRefName>`. When the PR title already fits,
     use it. For a plan-driven PR, `<scope>` is the task ID (`_context.md`, "PR
     marker conventions").
   - **Body**: what changed and why, as short paragraphs or bullets, describing the
     final state of the PR, not the history of its review rounds.
   - **Carry forward what CLAUDE.md requires a commit message to record.** Squashing
     drops the individual commits from `main`, so copy from them, condensed but not
     dropped: each "verified by breaking X, test Y failed" record, each stated
     optimization obligation, each new dependency's justification, and any deleted
     test's coverage check.
   - End with the `Co-Authored-By:` trailer(s) that appear in the PR's commits,
     deduplicated.

   Write the body to a temporary file (the job's temp directory, or `mktemp`), not
   an inline argument (see the long-command rule).

4. **Ask for approval.** Show the user the PR URL, the CI result, and the full
   subject and body, and ask whether to merge with this message. Revise it as asked.
   Do not merge without an explicit yes.

5. **Merge.**
   `gh pr merge <number> --squash --subject "<subject>" --body-file <file> --match-head-commit <headRefOid>`.
   `--match-head-commit` makes the merge fail if the branch moved after the message
   was drafted. Do not pass `--delete-branch` here: step 6 deletes the branches
   after confirming the merge.

6. **Clean up.**
   - Confirm `gh pr view <number> --json state,mergeCommit` shows `MERGED`.
   - `git push origin --delete <headRefName>` if the remote branch still exists (the
     repository may delete it automatically).
   - `git checkout <baseRefName> && git pull --ff-only`.
   - `git branch -D <headRefName>` if the local branch exists. `-D` is required
     because a squash merge leaves the branch's commits unmerged by ancestry; step 1
     established that nothing local would be lost. Delete only this PR's branch.
   - `git fetch --prune origin` to drop the stale remote-tracking ref.

7. **Report** the merged commit (`git log --no-show-signature -1 --oneline`), the
   deleted branches, and that the local base branch is up to date. When the PR came
   from `/runplan`, the next step is `/runplan`'s PR checkpoint: create the next
   branch from this updated base branch.
