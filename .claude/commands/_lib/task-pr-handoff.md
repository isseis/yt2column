---
description: "Internal: shared branch setup and PR handoff for the commands that produce a task PR (not a command)"
---

# Shared pattern: task branch and PR handoff

The commands that produce a task PR — the task-document commands `mkreq.md`,
`mkarch.md`, `mkplan.md`, and `mkplan2.md`, and `runplan.md` at a PR
checkpoint — each work on their own feature branch and end by opening the PR
and handing it to the human reviewer. This file defines those two parts once. A
calling command invokes them by saying "follow `_lib/task-pr-handoff.md`,
Part A" (before it writes anything) and "Part B" (after its last commit), and
supplies the inputs below.

Project-specific values (feature branch name, reviewer ping, task root, green
gate) are in `.claude/commands/_context.md`. The local review is the local CLI
review, `.claude/commands/_lib/local-cli-review.md`.

`<mention-check>` is the mention-check command defined on `_context.md`'s
"Rejected mentions" row (one `-e` per rejected mention). Never write the
rejected mentions into a command's own text; read them from `_context.md` when
you run it.

These commands do not wait for the review or the merge. The reviewer reviews the
PR and then runs `/runpr <PR>` by hand, which drives it to the merge. It waits for CI and any review threads, resolves the threads, and then asks
the merge question, which covers the merge and the bookkeeping committed just
before it (for a task-document PR, setting the document to `approved`), after
a deferred question when deferred review threads are left. `/runpr` shows the
next command after the merge and never runs it (`runpr.md` steps 2, 4, and 5).

## Inputs the calling command must supply

- **TASK_DIR** — the target task directory under the task root.
- **REPORT** — where the calling command gives its final report.
- **CHECKS** — what runs before the push: for a task-document command, the
  documentation guards
  (`go test -tags test ./cmd/yt2column ./cmd/fixprguard ./internal/pipeline`);
  for `runplan.md`, the green gate (`_context.md`).
- **LOCAL_REVIEW** — `run` for a task-document command; `done` for
  `runplan.md`, which reviews each phase group locally (its step 7.7).
- **PR_TEXT** — the PR title and what the body must name: for a task-document
  command, the "Task-document PR title" of `_context.md`, "PR title and squash
  subject"; for `runplan.md`, the title its step 5a writes from the PR marker's
  `推奨タイトル`, the marker's `レビュー観点`, and the Opus review result
  (`runplan.md` step 5a, "Before the PR", item 5).
- **ISSUE** (`mkreq.md` only) — the task's issue number. Every other
  caller leaves it out, and Part A reads it from the `対応 issue` line of
  TASK_DIR's `01_requirements.md` on `origin/main`
  (`git show origin/main:<TASK_DIR>/01_requirements.md`); a task whose
  requirements name no issue has none.
- **AFTER_CREATE** (optional) — what the calling command does once the PR
  exists (`runplan.md`: mark `PR を作成した` with the
  PR URL, commit, and push).

## Authorization

Invoking a calling command approves, for its task and its PR only, the network
operations this file names: `gh api user`, `gh pr list`, `gh pr view`,
`git fetch origin main`, `gh issue develop` and the issue operations of
"Choosing the issue to link" (Part A step 4, for the task's issue and its link
sub-issues only), `gh issue view` to read the labels of the task's issue
(Part B step 3), `git push` (including `-u`), and `gh pr create`. `coderabbit review` has standing approval under `CLAUDE.md`. Anything else that touches the network is asked for
first.

## Part A — prepare the branch

The calling command runs Part A before it reads any task document, so it reads
the documents (their status, the plan's checkboxes) from the branch the work
lands on, not from a stale checkout.

1. Check the working tree with `git status --short`. It must be clean; stop and
   report otherwise. (Approving the previous document is not a working-tree
   edit: `/runpr` commits it at the merge, on the reviewer's answer.)
2. `git fetch origin main`.
3. Use the current branch when its name follows the feature branch convention
   (`_context.md`) for TASK_DIR and one of these holds:
   - the branch has no commits of its own (`HEAD` is an ancestor of
     `origin/main`: the branch `/runpr` created after the previous merge, even
     if more was merged since); fast-forward it with
     `git merge --ff-only origin/main`; or
   - the calling command is `runplan.md`, the branch has commits beyond
     `origin/main`, none of them a task-document command's commit (a subject
     starting `docs(<task-id>): draft` or a PR-boundary commit of `mkplan2.md`),
     and it has no PR yet (`gh pr view <branch>` finds none): the PR group in
     progress, continued with its next phase group; or
   - the calling command is `mkreq.md`, continuing a stopped simple-process
     run on this branch because the branch meets every condition of
     `_lib/simple-process.md`, "Resuming a stopped run" (`mkreq.md` step 1
     checked them and checked it out): keep it as it is, without merging
     `origin/main` into it.

   A convention-named branch for TASK_DIR in any other state (commits beyond
   `origin/main` with a PR, or with no PR when the caller is a task-document
   command and the case above does not apply) is a reason to stop and report.
4. Otherwise create a new branch from `origin/main`:
   - Get the GitHub user name with `gh api user --jq .login`.
   - Choose `NN` one above the highest sequence number this task's branches have
     used, looking at local branches, `origin` branches, and the head branches
     of all PRs (`gh pr list --state all --limit 1000 --json headRefName`); merged
     branches are deleted, so the PR list is what remembers them. With none,
     `NN` is `01`.
   - When the task has an issue (ISSUE), create the branch as one linked to
     it, so the PR opened from it appears in the issue's Development section
     without a closing keyword in its body. An issue holds at most ten linked
     pull requests, so first choose the issue to link (LINK_ISSUE) by
     "Choosing the issue to link" below, then run
     `gh issue develop <LINK_ISSUE> --name <branch> --base main --checkout`. This
     creates the branch on `origin` at the head of `main` and checks it out
     with upstream tracking. The repository's "Auto-close issues with merged
     linked pull requests" setting is off, so merging such a PR leaves the
     issue open; `/runpr` closes it after the task's final plan PR
     (`runpr.md` step 5). Confirm `git rev-parse HEAD` equals
     `git rev-parse origin/main` after a `git fetch origin main`; if `main`
     moved in between, fast-forward with `git merge --ff-only origin/main`.
   - Without an issue: `git switch -c <branch> origin/main`.

### Choosing the issue to link

A task usually needs more than ten PRs only when its plan has many PR
checkpoints. The PRs beyond ten are linked to a sub-issue of ISSUE made for
holding them (a **link sub-issue**), not to ISSUE itself.

1. Count an issue's links: its linked PRs, open or closed, plus its linked
   branches that have no PR yet (a branch's entry is replaced by its PR's
   once the PR is opened):

   ```bash
   gh api graphql -F owner='{owner}' -F repo='{repo}' -F n=<issue> -f query='
     query($owner: String!, $repo: String!, $n: Int!) {
       repository(owner: $owner, name: $repo) {
         issue(number: $n) {
           closedByPullRequestsReferences(first: 20, includeClosedPrs: true) {
             nodes { headRefName }
           }
           linkedBranches(first: 20) { nodes { ref { name } } }
         }
       }
     }'
   ```

   The count is the PR nodes plus the branch nodes whose name is no PR
   node's `headRefName`.
2. When ISSUE has fewer than ten links, LINK_ISSUE is ISSUE.
3. Otherwise list ISSUE's sub-issues
   (`gh api repos/{owner}/{repo}/issues/<issue>/sub_issues --paginate`) and
   take the open link sub-issues, those titled `#<issue> の PR リンク (<k>)`.
   LINK_ISSUE is the one with the highest `<k>` when it has fewer than ten
   links (step 1).
4. When there is none, or that one is full, create the next one with `<k>` one
   above the highest (`2` for the first) and add it as a sub-issue of ISSUE.
   Write its body (below) to `<file>` first, and run the mention check on it:
   **Mention check.** Before posting, run `<mention-check> <file>`
   on each file of text to post. Post only when it exits `1` for every file: exit
   code `0` means the text contains a rejected mention, and any other code means the
   check failed. Never rewrite the text to get past the check.
   When the check fails, do not create the link sub-issue: stop and report.
   Otherwise run:

   ```bash
   gh issue create --title "#<issue> の PR リンク (<k>)" --body-file <file>
   gh api -X POST repos/{owner}/{repo}/issues/<issue>/sub_issues \
     -F sub_issue_id="$(gh api repos/{owner}/{repo}/issues/<new> --jq .id)"
   ```

   The body, in Japanese, says that the issue exists only to hold the task's
   PRs beyond the ten the parent issue can link, that the task is tracked on
   the parent issue, and that `/runpr` closes it together with the parent.
   `sub_issue_id` is the issue's numeric `id`, not its number. LINK_ISSUE is
   the new issue.

## Part B — open the PR and hand it over

Run this after the calling command's last commit, with a clean working tree.

1. When LOCAL_REVIEW is `run`, review the branch with the local CLI review
   (CodeRabbit CLI), with the
   Fable partial review in parallel: follow
   `.claude/commands/_lib/local-cli-review.md` with
   **BASE** = `$(git merge-base origin/main HEAD)`, **SCOPE** = branch,
   **SUMMARY** = REPORT, **RECORD** = `commit`, so its record lines are in a
   commit of their own before the push, and **FABLE** = KIND `documents`,
   TASK_DIR, and SELF_REJECTED = the findings the calling command's
   self-review (and, in the simple process, its review of the three
   documents) rejected in this session, each as its location and your own
   summary (`_lib/fable-review.md`, "Inputs and outputs"). This runs after
   the calling command committed the fixes of its self-review and of
   `japrose`. When it stops, stop before step 2: nothing is pushed, and the
   Fable partial review leaves no record, so a run again reviews again.

   **README sync.** Run this paragraph even when the sentence above does not
   apply (LOCAL_REVIEW is `done`). Run the README sync check as
   `.claude/commands/mktrans.md`, "When a caller runs it", says. For
   `translated`, commit `README.en.md` alone, with a message such as
   `docs: sync README.en.md with README.md`; CHECKS then
   runs on that commit, and step 3 lists the reported terms in the PR body.
   For `stopped`, stop before step 2: nothing is pushed.

2. Run CHECKS. If they fail, fix, commit, and run them again; do not go on to
   the push until they pass.
2.5. Check the commits still awaiting local CLI review. Resolve HEAD and the
   merge-base with origin/main. Run these commands separately, using a new
   scratchpad file outside the work tree and the full ID from the first
   command. Supply PLAN only for runplan.

   ```bash
   git rev-parse --verify 'refs/remotes/origin/main^{commit}'
   git --no-replace-objects show <origin-main-commit>:.claude/commands/_lib/review-records.sh > <records-script>
   bash <records-script> coverage none <HEAD> [<PLAN>]
   ```

   On success with `ALL`, continue to step 3. For `PARTIAL <n>`, take
   BASE as the first parent of the oldest listed commit if that parent
   belongs to origin/main..HEAD, otherwise use the merge-base. If any
   command fails or its result is unreadable, use the merge-base.
   Run `_lib/local-cli-review.md` once with this BASE, SCOPE = commits
   awaiting review before the PR, SUMMARY = REPORT and RECORD = commit.
   Pass no FABLE. Its two-attempt cap applies. When `_lib/local-cli-review.md`
   stops, stop before step 3: nothing is pushed. A CLI failure records the
   unreviewed range and continues. After fix commits, rerun CHECKS until they
   pass. Include any further CHECKS fixes in step 3's unreviewed ranges.
   Proceed to step 3 without repeating this review.

3. Push and create the PR for the current branch. Ignore the calling command's
   `$ARGUMENTS` here (it is a task or an issue, not a PR or a branch). First
   write the title and the body to files, as below, and run the mention check
   on both files:
   **Mention check.** Before posting, run `<mention-check> <file>`
   on each file of text to post. Post only when it exits `1` for every file: exit
   code `0` means the text contains a rejected mention, and any other code means the
   check failed. Never rewrite the text to get past the check.
   When the check fails, do not create the PR: stop and report. Otherwise run
   `git push -u origin <current-branch>` and
   `gh pr create --base main --head <current-branch> --title "$(cat "<title-file>")" --body-file <file> --assignee @me <label-flags>`,
   writing the title and the body to files in the scratchpad with the Write tool
   (`_context.md`, "Passing a title to `gh`"). `--assignee @me` assigns the PR
   to the GitHub user `gh` is authenticated as, the user who runs the command.
   `<label-flags>` carries over the labels of the task's issue (ISSUE, or the
   `対応 issue` Part A read; not LINK_ISSUE, a link sub-issue): read them with
   `gh issue view <issue> --json labels --jq '.labels[].name'` and pass one
   `--label '"<name>"'` per line printed. `gh` reads the value as CSV and
   splits it at commas, so the name is one CSV field in double quotes (write
   each `"` in a name as `""`), and the whole value is in single quotes so the
   shell expands nothing in it (write each `'` as `'\''`). With no issue or no
   label, it is empty. Do not pause for the user to
   adjust the title or body; the reviewer can edit them on GitHub. The title is
   PR_TEXT's and follows `_context.md`, "PR title and squash subject". The title
   and the body use the terms of the translation glossary, as `_context.md`,
   "Translation glossary", says. The body is in Japanese (`CLAUDE.md`,
   "Language"):
   - summarizes the change;
   - lists the decisions made with the reviewer during this run, if any;
   - names the points the reviewer should check (for `runplan.md`, the
     `レビュー観点`), including every `unclear` or `deferred` finding of the
     local CLI reviews, including earlier records: this run's SUMMARY, and the finding
     lines earlier sessions recorded in commit messages on the branch
     (`git log origin/main..HEAD`; the `CodeRabbit local review:` and
     `Codex local review:` lines, read as `_lib/local-cli-review.md`,
     "Reading the record lines", says; a `not run` line is not a finding),
     and every `unclear` or `deferred` Fable finding (its `Fable finding:`
     line, read as `_lib/fable-review.md`, "Reading the record lines", says).
     Describe a Codex CLI or a Fable finding only by its location and your
     own summary, as the record line does, never with Codex's or Fable's
     text;
   - names the commits of this PR that the local CLI did not review, if any. When this
     session reviewed the whole branch (step 1 or `runplan.md` step 5a), give
     only the unreviewed ranges in that review's SUMMARY: a range that review
     covered counts as reviewed even when a commit message has a `not run`
     line. Otherwise give the ranges of the `not run` lines in
     `git log origin/main..HEAD`, of either form (`Local CLI review: not run`,
     or an earlier session's `CodeRabbit local review: not run`, which stands
     for the whole branch). Account for step 2.5's result: omit ranges it covered,
     include its remaining unreviewed ranges and any subsequent CHECKS fixes;
   - for `runplan.md`, gives PR_TEXT's Opus review result: whether the Opus
     review ran, and with what result, or which condition kept it from
     running;
   - carries the Fable partial review section, written from the Fable lines
     of `git log --reverse origin/main..HEAD`, read as
     `_lib/fable-review.md`, "Reading the record lines", says (the
     seen-range lines do not go into the body):
     - the heading `## Fable の部分レビュー`, and under it, for each Fable
       record, whether it ran (`run`, or `not run` with its reason) and its
       range; the places put in scope with their criteria and the places
       left out with their reasons; the number of fixed findings; each
       rejected finding with its reason; and the `deferred` and `unclear`
       findings, each by its location and your own summary;
     - after it, `<details><summary>Fable の行</summary>`, holding a `text`
       code block with every Fable line, oldest commit first, exactly as
       written in the commit messages, and `</details>`. A later script
       reads this block from the merged PR's body;
     - when the body would exceed GitHub's limit (65,536 characters),
       leave the `Fable scope:` lines out of the code block and say so, and
       that they are in the branch's commits; when it would exceed the
       limit still, leave the code block out and say so;
     - with no Fable record on the branch, the section says only `記録なし`;
   - lists the terms the README sync (step 1) reported as
     missing from the translation glossary, if any;
   - when `git log origin/main..HEAD` has correction lines (read from the
     commits, not from what this session remembers), carries the correction
     paragraph written from them, and when the branch's change-history lines
     (`git diff origin/main...HEAD`) record a `draft` revert or a session
     re-approval, names those decision changes; both as
     `docs/dev/conventions/requirements_process.md`, "Recording in the PR
     description", says. The correction paragraph is in the body file the
     mention check reads;
   - contains `Refs #<issue>` for the issue the requirements document names
     as its own (`対応 issue`), never a closing keyword (`Closes`, `Fixes`,
     `Resolves`): the issue stays open until the task is implemented. The
     Development link comes from the branch (Part A step 4), not from the
     body. A task whose requirements name no issue gets no such line.
4. Run AFTER_CREATE, if supplied.
5. Ping the human reviewer (`_context.md`, "Reviewer ping") in Japanese with the
   PR URL and the next action: review the PR, then run `/runpr <PR>`. If the ping tool is
   unavailable or the ping is not sent, the report in step 6 carries the same
   message.
6. In REPORT, give the PR URL, the review results, and the same next action.
   Then stop: do not wait for a PR review and do not merge.
