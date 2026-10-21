# fixprguard Guide

`fixprguard` is a developer guard tool for `/fixpr` and `/runpr`. It judges the authors of review threads, judges the change scope of a fix, compares the git state, commits and pushes, and posts replies and updates the PR description on GitHub, with deterministic code instead of leaving these to the session or agent reading the procedure. This guide describes the prerequisite environment, installation, the usage of each subcommand, the scope rules, what the tool guarantees and does not, and what to do when it stops.

Until the command documents are switched to call this tool (phase 6 of task 0030), `/fixpr` and `/runpr` run their previous procedure. The descriptions in this guide of how the commands use the tool describe the behavior after the switch.

The background and rationale of the design are in the architecture document of task 0030 (`docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md`).

## 1. Role

The body of a review thread can be written by anyone who can write on the PR, so it is untrusted text. `fixprguard` enforces the following three layers of defense in code.

1. **Author judgment.** Only threads in which the author of every comment is a trusted author are processed. Trusted authors are the automated reviewer (the Bot passed with `--reviewer`) and people with write permission on the repository (`role_name` is `write`, `maintain`, or `admin`). The bodies of untrusted threads are not fetched.
2. **Fenced block.** Untrusted text embedded in an agent's prompt is enclosed in delimiter lines that contain a per-run random value (nonce).
3. **Change scope and git state checks.** Before the commit and the push, the tool checks that the fix did not change files outside the scope of the processed threads, or git's hooks and config.

The division of responsibilities is as follows.

| Owner | Responsibility |
|---|---|
| `fixprguard` binary (`cmd/fixprguard`, `internal/fixprguard`) | Judgment, fetching, comparison, commit and push, writes to GitHub |
| `/fixpr` and `/runpr` commands (`.claude/commands/`) | The order in which subcommands are called, confirmation with the user, launching agents |
| Developer | Decisions about out-of-scope changes and untrusted threads, what to do when the tool stops |

The tool keeps no state file. The session holds the baseline and the judged set ID that a subcommand prints, and passes them to the next subcommand as arguments.

## 2. Prerequisites

| Item | Requirement |
|---|---|
| `git`, `gh`, `make` | On `PATH`, with `gh auth login` done |
| Working directory | Run at the top level of the work tree, with HEAD on a branch. Elsewhere than the top level, `begin` stops with exit code `2`, and `inspect`, `finish`, and `push` stop with `git-state-changed` (`config`). With a detached HEAD, `begin` stops with exit code `2` |
| `origin` | Points at this repository on GitHub. `gh repo view` returns this repository without a prompt |
| `GIT_*` environment variables | The tool removes them from the git and `gh` it starts |
| `GH_REPO`, `GH_HOST` | Not set. If set, the subcommands that call `gh` stop with exit code `2` without starting `gh` (because `gh` would target another repository) |

## 3. Installation

With an up-to-date `main` checked out, run:

```sh
make install-fixprguard
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

After a PR that changes the tool is merged, update `main` and reinstall. The command documents check the version with `--protocol`, but a change that alters the rules while keeping the same `protocol` number cannot be detected. The commit the installed binary was built from can be checked with `vcs.revision` in the output of:

```sh
go version -m "$(command -v fixprguard)"
```

One binary is shared by all worktrees of the same user.

## 4. Common conventions

| Item | Convention |
|---|---|
| `--protocol N` | Required by every subcommand. If `N` does not match the binary's number, the tool stops with exit code `2` (this keeps an old binary from judging by the assumptions of newer documents). The current number is `1` |
| `--reviewer LOGIN` | Required by `list`, `begin`, `inspect`, and `reply`, and not accepted by the other subcommands. Pass the automated reviewer's GraphQL `author.login` (for example `coderabbitai`). There is no default |
| `[PR]` | Only a decimal PR number is accepted. URLs and branch names are rejected. When omitted, the current branch's PR is resolved with `gh pr view`. `finish`, which does not read GitHub, cannot omit it |
| Flag position | Flags go before the positional argument (`PR`, `FILE`). Passing the same flag twice is rejected (`--accept` and `--thread` may be passed more than once) |
| Standard output | One line of JSON (only `fence` prints the fenced block itself). Every JSON output contains `protocol` |
| Standard error | A one-line diagnostic. It names only the rule that rejected the input, and contains no value from GitHub and no arguments or standard error of an external command |

The exit codes are as follows. The caller treats any other exit code, termination by a signal, and output that cannot be read as JSON as "stop".

| Exit code | Meaning | Standard output |
|---|---|---|
| `0` | Continue | The subcommand's result |
| `1` | Stop | `stop` (the stop kind) and the details of each kind (chapter 8) |
| `2` | Usage error (flags, value grammar, `--protocol` mismatch, `GH_REPO`/`GH_HOST`, an empty positional argument). Running elsewhere than the top level of the work tree, a detached HEAD, or a malformed `.git` file (`begin`), and a `--comment-id` that is not the thread's first comment (`reply`; found only after reading GitHub) also give `2` | None |
| `3` | Ask the user (`inspect` only; there are out-of-scope changes) | The result of `inspect` |

## 5. Subcommands

Each subcommand can also be run on its own. To investigate why something was rejected, run it as shown here with the same arguments as the command document.

### `head [PR]`

Checks that the PR's head is a branch of this repository (only when `isCrossRepository` is the boolean `false` and `headRepository` exists). A fork PR stops with `cross-repository`. `/runpr` runs this before `gh pr checkout`.

```sh
fixprguard head --protocol 1 42
```

Output: `{"protocol":1,"sameRepository":true}`. Side effects: reads GitHub only.

### `list --reviewer LOGIN [PR]`

Fetches every page of unresolved threads (at most 2000; beyond that it stops with `thread-cap`) and prints, for each thread, the trust judgment, the reasons, and the fingerprint. It neither fetches nor prints any body. It first makes the same check as `head`.

```sh
fixprguard list --protocol 1 --reviewer coderabbitai 42
```

Output: `threads` (each element has `threadId`, `url`, `trusted`, `fingerprint`, and `findings`) and `permissionLookups` (`used`, the permission lookups used, and `remaining`, the lookups left). The `reason` of a finding is one of the following.

| `reason` | Meaning | Extra field |
|---|---|---|
| `no-write-permission` | A user without write permission | `account` |
| `other-bot` | A Bot other than the automated reviewer | `account` |
| `no-author-name` | No author, an empty name, or a name outside the login grammar | `commentUrl` |
| `permission-unavailable` | The permission could not be looked up | `cause` (`not-found`, `forbidden`, `rate-limited`, `budget-exceeded`, `rejected-response`, `failed`) |
| `incomplete-comments` | The thread has more than 100 comments, so not all of them can be fetched | None |
| `unknown-author-kind` | The author's kind is neither `User` nor `Bot` | None |

### `begin --reviewer LOGIN [--accept ID=FINGERPRINT ...] [PR]`

Phase 1 of `/fixpr`. It runs in this order: (1) the work tree has no uncommitted change, untracked files included (if it has one, it stops with `dirty-work-tree` without querying GitHub); (2) the same check as `head`; (3) recording the baseline; (4) judging the first page (100 threads) and fetching the bodies of the threads to process.

The threads to process are the trusted threads and the threads named with `--accept` whose fingerprint matches the current one. Pass to `--accept` the thread ID and fingerprint from the report output shown to the user.

```sh
fixprguard begin --protocol 1 --reviewer coderabbitai 42
```

Output fields:

| Field | Content |
|---|---|
| `baseline` | The baseline. Pass it to `inspect`, `finish`, and `push` with `--baseline` |
| `number`, `capHit` | The PR number. When `capHit` is true, threads on the second page and later are not covered this time |
| `processCount` | The number of threads to process. `0` means there is nothing to process |
| `untrusted` | The untrusted threads that are not processed (`threadId`, `url`, `fingerprint`, `findings`). No body |
| `changed` | Threads not processed because their comments changed (`stage` is `since-accept` when they changed after being named, and `since-judgment` when they changed between the judgment and the body fetch) |
| `acceptNotFound` | IDs that were named but not found among the unresolved threads |
| `permissionLookups` | The number of permission lookups used and left |
| `nonce`, `threads` | `threads` is a string: the array of threads to process (`threadId`, `databaseId`, `authorType`, `body`, `path`, `line`, `url`, `fingerprint`, `named`) as one line of JSON, enclosed in `BEGIN-UNTRUSTED-<nonce>` and `END-UNTRUSTED-<nonce>` lines. Embed this string unchanged in an agent's prompt |

Side effects: reads GitHub and `git ls-remote` only.

### `fence FILE`

Writes the file's content, enclosed in a fenced block, to standard output as it is (not JSON). The nonce is redrawn until it is a value the file does not contain. Only a regular file (not a symbolic link, FIFO, or device) of at most 16 MiB is read. `/fixpr` writes the triage agent's output to a file, encloses it with this, and then passes it to the fix agent.

```sh
fixprguard fence --protocol 1 /path/to/triage.json
```

Side effects: none.

### `inspect --reviewer LOGIN --baseline TOKEN [--thread ID=FINGERPRINT ...] [PR]`

Run after the fix phase. It first compares the hooks, config, and info with the baseline, and if they differ, stops with `git-state-changed` without starting `gh`. It then fetches the `--thread` threads again from GitHub to get their `path` (a different fingerprint gives `thread-changed`), enumerates the changes in the work tree, and checks them against the scope (chapter 6).

```sh
fixprguard inspect --protocol 1 --reviewer coderabbitai --baseline 'v1....' --thread 'PRRT_...=<fingerprint>' 42
```

Output: `judgedSet` (the judged set ID; pass it to `finish`), `changes` (the number of changed paths), and `outOfScope` (the out-of-scope changes; each element has `position`, `path`, `kind` (`add`, `modify`, `delete`, `other`), `added`, `deleted`, and `binary`). If `outOfScope` is not empty, the exit code is `3`, and the user is shown the list and asked whether to continue. A path that fails the path checks (chapter 6) is not printed in `path`; it is shown by `position` only.

Side effects: reads GitHub and git (including `git ls-remote`) only. The work tree is not changed.

### `finish --baseline TOKEN --judged-set ID PR`

Phase 4 of `/fixpr`. It enforces the following order within one call: (1) the work tree matches the judged set and the git state matches the baseline; (2) `make fmt`; (3) recording the formatted set (it stops if the set of changed paths changed); (4) `make test` and `make lint`; (5) a strict match with the formatted set; (6) staging only the judged paths, using literal pathspecs; (7) checking what was staged (undoing the staging on a mismatch); (8) the commit; (9) checking that the commit's tree matches the formatted set. The make targets are the fixed values `fmt`, `test`, and `lint`, and are never taken from an argument. The commit subject is `fix: address PR #<PR> review comments`.

```sh
fixprguard finish --protocol 1 --baseline 'v1....' --judged-set '<judgedSet>' 42
```

Output: `committed` (`false` when the judged set is empty, in which case nothing is done) and `commit` (the SHA of the commit made; pass it to `push`).

Side effects: the formatting by `make fmt`, the writes the build checks make, staging, and the commit. No push, branch switch, or reset (except undoing the staging).

### `push --baseline TOKEN --commit SHA`

Pushes the SHA that `finish` returned. Right before that, it checks that the hooks, config, and info match the baseline; that HEAD is that SHA and its only parent is the baseline's commit; and that the OID of the push target (the branch of the same name on the remote that git resolves for the current branch) matches the baseline. It pushes with an explicit refspec, without tags or submodules.

```sh
fixprguard push --protocol 1 --baseline 'v1....' --commit '<SHA>'
```

Output: `{"protocol":1,"pushed":true,"commit":"<SHA>"}`.

### `reply --reviewer LOGIN --thread-id ID --comment-id N --fingerprint FP --body-file FILE [--accept ID=FINGERPRINT ...] [PR]`

Judges the thread again in its current state, and posts the reply only when it is a trusted thread (or a thread named with `--accept` whose fingerprint matches), `--fingerprint` matches the current fingerprint, and `--comment-id` is the ID of the thread's first comment. The body is passed to `gh api` as JSON on standard input, so it never goes through a shell. After the reply, it fetches the thread again and checks that nothing changed besides the reply before resolving the thread.

The body file must be a regular file (whose last element is not a symbolic link) of at most 64 KiB holding valid UTF-8 that is not only whitespace and contains no NUL.

```sh
fixprguard reply --protocol 1 --reviewer coderabbitai --thread-id 'PRRT_...' --comment-id 123 --fingerprint '<fingerprint>' --body-file /path/to/reply.md 42
```

Output: `{"protocol":1,"written":true}`. When it stops with `reply-not-resolved`, the reply may have been posted, so do not post the same reply again.

### `edit-pr --title-file FILE --body-file FILE [PR]`

Updates the PR's title and body. The title is passed in the form `--title=<value>`, and the body is passed as the checked content on standard input (`gh` does not read the file again). The title file must pass the same checks as the body file and also be one line with no control character and no line separator.

```sh
fixprguard edit-pr --protocol 1 --title-file /path/to/title.txt --body-file /path/to/body.md 42
```

Output: `{"protocol":1,"written":true}`.

## 6. Scope rules

For each `path` of the threads passed with `--thread` (as fetched again from GitHub), `inspect` decides which files are in scope with the following rules.

1. The `path` itself.
2. Files in the same directory as `path` (not inside subdirectories).
3. If `path` is inside `docs/tasks/NNNN_<name>/`, every file under that directory (including inside subdirectories).
4. However, an execution-related file (the table below) is out of scope unless it is the `path` itself. For a named untrusted thread, it is out of scope even when it is the `path` itself.
5. For a renamed file, the deletion before the rename and the addition after it are each judged.
6. A change that is not to a regular file (a symbolic link, a submodule, a file type change, a FIFO or device, a file over 8 MiB) is out of scope regardless of its path.

A `.go` file is in scope under rule 2 only when `path` is a `.go` file in the same directory (fixing a test of the same package), and never under rule 3. For a named untrusted thread, every `.go` file other than the `path` itself is out of scope. This is because `make test` compiles and runs the in-scope `.go` files.

**Path checks.** Only paths that are valid UTF-8 with no control character, not absolute, with no `..` element, and unchanged by normalization are judged. Any other path is out of scope, and its `path` is not printed in the output. A path containing a non-ASCII character is in scope only when it matches `path` byte for byte.

**Execution-related files.** The files that the build checks and CI read as configuration, compile, or run. Matching treats ASCII upper and lower case as the same (because on a case-insensitive file system `makefile` and `Makefile` refer to the same file). The list is the same as the definition in `internal/fixprguard/scope.go`.

| Form | Items |
|---|---|
| A file with this name in any directory | `makefile`, `gnumakefile`, `go.mod`, `go.sum`, `go.work`, `go.work.sum`, `.tool-versions`, `.mise.toml` |
| A file with this extension in any directory | `*.mk` |
| The `.go` rule (above) | `*.go` |
| Every file under this directory | `.claude/`, `.github/`, `extension/scripts/` |
| A file at the repository root | `/.*` (every file whose name starts with `.`), `/claude.md`, `/agents.md` |
| A file directly under `extension/` | `extension/package.json`, `extension/package-lock.json`, `extension/.npmrc`, `extension/.node-version`, `extension/.prettierrc.json`, `extension/.prettierignore`, `extension/eslint.config.js`, `extension/tsconfig.json`, `extension/tsconfig.core.json`, `extension/tsconfig.build.json` |

## 7. What the tool guarantees and does not

### 7.1 Guarantees

| Guarantee | Mechanism |
|---|---|
| The bodies of untrusted threads are not fetched | The judgment is made with a query that contains no body, and bodies are fetched only for the threads to process. A thread whose comments changed between the judgment and the body fetch is not processed, because the fingerprints do not match |
| A body cannot end the fenced block | The tool checks that the nonce does not occur in the body. `begin`'s `threads` is one line of JSON, so newlines and quotes in a body cannot form a delimiter line |
| Out-of-scope changes are not committed without the user's confirmation | `inspect` lists the out-of-scope changes and returns exit code `3`, and `finish` commits only the changes that match the judged set |
| Changes that appear after the judgment are not committed | `finish` checks the strict match with the judged set before and after the build checks, and stages only the judged paths |
| git is not started with rewritten hooks or config | `inspect`, `finish`, and `push` compare with the baseline before starting git or `gh` |
| Replies and the PR description do not become shell commands | `reply` and `edit-pr` do not go through a shell, and pass the body on standard input |
| The head of a fork PR is not handled | `head`, `list`, and `begin` check it first |

### 7.2 Not guaranteed

- That an agent does not follow instructions inside the fenced block. Forging the delimiter lines is prevented, but the behavior of an LLM cannot be guaranteed.
- The order in which `inspect` through `finish` are called, the confirmation with the user, and limiting naming to the threads in the immediately preceding report. These depend on the command documents (the session).
- Only the state at the time of a check is guaranteed. Another process rewriting the hooks or config between the check and the execution, and something changed and changed back during the fix phase, are not detected.
- A comparison before `head`, `list`, `reply`, and `edit-pr`, which take no baseline, start `gh`. `/fixpr` calls them after `finish` and `push`, so the comparisons up to then detect a rewrite, but a rewrite after `push` is not detected.
- Preventing in-scope test code from being run by the build checks. `finish` and `push` do detect that code rewriting in-scope files or hooks.
- Files ignored by `.gitignore`, files hidden by a `.gitignore` inside the work tree or by the global exclude file, and changes hidden by forging the stat information of the index. Hidden changes are not committed, but the build checks running them cannot be prevented.
- Files outside the hooks directory that a hook runs (such as the cache of the pre-commit framework).
- That the value of `--reviewer` is the same as the value in `_context.md`. The tool does not read `_context.md`.
- Re-edits within the same second. The times in the fingerprint have a resolution of one second.
- The content of reply and PR description bodies. The tool checks only the file's type, size, and encoding.

## 8. What to Do When the Tool Stops

The `stop` field of the exit code `1` output gives the stop kind. When the tool stops, do not work around it; remove the cause, then redo the round from `begin`.

| `stop` | Cause | Detail fields | What to do |
|---|---|---|---|
| `dirty-work-tree` | At `begin`, the work tree has uncommitted changes (untracked files included) | None | Examine the changes with `git status`, discard or commit them, and redo. When the previous round was stopped by out-of-scope changes, the changes remain, so the run stops here too |
| `cross-repository` | The PR's head is in another repository, such as a fork | None | The tool does not handle it. Check the PR's content yourself |
| `git-state-changed` | One of HEAD, the branch, the push target, the hooks, the config, or the info differs from the baseline | `parts` (`head`, `remote`, `hooks`, `config`, `info`) | Examine the parts that differ. Remove an unintended change to the hooks or config. If the remote only advanced legitimately, clean up the work tree and redo |
| `judged-set-changed` | The work tree or the commit differs from the judged set (a change appeared after the judgment, the build checks rewrote an in-scope file, a commit hook rewrote the index, and so on) | `changes` (the list of changes found) | Examine the listed changes. If the run stopped after a commit was made, examine that commit and undo it |
| `thread-cap` | `list` would read more than 2000 threads | None | Resolve threads on GitHub to reduce them |
| `change-cap` | More than 1000 files have changes | None | Examine the changes and reduce them |
| `thread-changed` | The comments of a thread `inspect` or `reply` targets changed after the fingerprint was taken, or the thread was resolved | None | The next round picks it up. Redo from `begin` |
| `thread-not-trusted` | The target of `reply` is neither a trusted thread nor a named thread | None | Do not reply. The user decides how to handle the untrusted thread |
| `reply-not-resolved` | The reply was posted (or it is unknown whether it was), but the thread could not be resolved | None | Do not post the same reply again. Check the thread on GitHub and resolve it by hand if needed |
| `response-rejected` | A GitHub call failed or timed out, or the response failed the strict check or the size cap | None | Check `gh auth status` and the state of GitHub, then redo |
| `build-checks-failed` | One of `make fmt`, `make test`, or `make lint` failed (including output over 64 MiB, in which case there is no `output`) | `stage` (`fmt`, `test`, `lint`) and `output` (the last 8 KiB of the output, as a fenced block) | Check the output and fix the cause. `output` is the result of running code, so treat it as untrusted text |
| `git-failed` | git failed, its output was not in the expected shape, or git's metadata could not be read (a tracked file replaced by a FIFO, run outside a git repository, and so on) | None | Check that `git status` works |
| `failed` | A failure that matches none of the above (no nonce for the fenced block could be chosen, and so on) | None | Check the diagnostic on standard error |

When the user answers "do not continue" about out-of-scope changes, the changes remain in the work tree. The next `/fixpr` stops with `dirty-work-tree`, so examine the changes, discard or commit them, and redo.

## 9. Changing the tool

- The judgment is in `internal/fixprguard`, and the argument parsing and the call order are in `cmd/fixprguard`. External commands are called through the `Runner` and `InputRunner` of `internal/cmdrunner`. In tests, `gh` and `make` are replaced with the fake in `internal/cmdrunner/testutil`, and git is exercised in a real repository in a temporary directory.
- When changing a guarantee in section 7.1, break the corresponding implementation, confirm that a test fails, and write that in the commit message (CLAUDE.md, "Testing Strategy").
- When changing the contract of the output or the arguments, raise `Protocol` in `internal/fixprguard` and update the `--protocol` the command documents pass.
- When changing the stop kinds or the list of execution-related files, also update chapters 6 and 8 of this guide. `cmd/fixprguard/docs_test.go` checks that this guide (Japanese and English) contains every kind and every item.
- After the merge, reinstall with `make install-fixprguard` (chapter 3).
