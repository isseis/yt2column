# fixprguard Guide

`fixprguard` is a developer guard tool for `/fixpr` and `/runpr`. It judges the authors of review threads, judges the change scope of a fix, compares the git state, commits and pushes, and posts replies and updates the PR description on GitHub, with deterministic code instead of leaving these to the session or agent reading the procedure. This guide describes the prerequisite environment, installation, the usage of each subcommand, the scope rules, what the tool guarantees and does not, and what to do when it stops.

The background and rationale of the design are in the architecture document of task 0030 (`docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md`).

## 1. Role

The body of a review thread can be written by anyone who can write on the PR, so it is untrusted text. `fixprguard` enforces the following three layers of defense in code.

1. **Author judgment.** Only threads in which the author of every comment is a trusted author are processed. Trusted authors are the automated reviewers (a Bot with any of the names passed with `--reviewer`; `/fixpr` and `/runpr` pass every name on `Automated reviewer list` in `.claude/commands/_context.md`) and people with write permission on the repository (`role_name` is `write`, `maintain`, or `admin`). The bodies of untrusted threads are not fetched.
2. **Fenced block.** Untrusted text embedded in an agent's prompt is enclosed in delimiter lines that contain a per-run random value (nonce).
3. **Change scope and git state checks.** Before the commit and the push, the tool checks that the fix did not change files outside the scope of the processed threads, or git's hooks and config. For the config, the `branch.<name>.*` settings of other branches (every branch except the current branch and `main`) are not compared. This is so that a concurrent session operating on branches in another worktree does not stop the run. Edits that leave the list of configuration values unchanged (edits of only comments, blank lines, or indentation) are not detected either. A rewrite of another branch's settings is a residual risk (7.2).

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
| git version | 2.12 or later. 2.12 is the first version that reads `protocol.<name>.allow`; an older git silently ignores the `-c protocol.ext.allow=never` the tool passes |
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

One binary is shared by all worktrees of the same user. On the other hand, `/fixpr` and `/runpr` read the command documents in that worktree's checkout and pass the `--protocol` written there. Therefore, for a change that raises the `protocol` number (for example, from `1` to `2` in task 0035), follow these steps.

- Before that PR's `/runpr`, reinstall from that PR's branch, because that PR's command documents pass the new number. Until the merge, `/fixpr` and `/runpr` in other worktrees that have not merged `main` stop with exit code `2`.
- Right after the merge, reinstall from the updated `main`. Running from documents with the new number before reinstalling stops with exit code `2`.
- Merge `main` into other branches in progress before the next `/fixpr` or `/runpr`. Until then, that branch's documents pass the old number, so the run stops with exit code `2`.

In either case, the tool never judges by the old rules (fail-closed).

## 4. Common conventions

| Item | Convention |
|---|---|
| `--protocol N` | Required by every subcommand. If `N` does not match the binary's number, the tool stops with exit code `2` (this keeps an old binary from judging by the assumptions of newer documents). The current number is `3` |
| `--reviewer LOGIN` | Required at least once by `list`, `begin`, `inspect`, and `reply`, and not accepted by the other subcommands. Pass the GraphQL `author.login` of each trusted automated reviewer (for example `coderabbitai`), repeating the flag once per name (for example `--reviewer <login-1> --reviewer <login-2>`). The names `/fixpr` and `/runpr` pass are every name on `Automated reviewer list` in `.claude/commands/_context.md`. The order does not affect the judgment. When none is given, when the same name is given twice or more, or when a name does not fit the grammar, it is a usage error (exit code `2`). There is no default |
| `[PR]` | Only a decimal PR number is accepted. URLs and branch names are rejected. When omitted, the current branch's PR is resolved with `gh pr view`. `finish`, which does not read GitHub, cannot omit it |
| Flag position | Flags go before the positional argument (`PR`, `FILE`). Passing the same flag twice is rejected (`--accept`, `--thread`, and `--reviewer` may be passed more than once; but passing the same name to `--reviewer` twice is rejected) |
| Standard output | One line of JSON (only `fence` prints the fenced block itself). Every JSON output contains `protocol` |
| Standard error | A one-line diagnostic. It names only the rule that rejected the input, and contains no value from GitHub and no arguments or standard error of an external command |

The exit codes are as follows. The caller treats any other exit code, termination by a signal, and output that cannot be read as JSON as "stop".

| Exit code | Meaning | Standard output |
|---|---|---|
| `0` | Continue | The subcommand's result |
| `1` | Stop | `stop` (the stop kind) and the details of each kind (chapter 8) |
| `2` | Usage error (flags, value grammar, `--protocol` mismatch, `GH_REPO`/`GH_HOST`, an empty positional argument). Running elsewhere than the top level of the work tree, a detached HEAD, or a malformed `.git` file (`begin`), and a `--comment-id` that is not the thread's first comment (`reply`; found only after reading GitHub) also give `2`. A `reply` body, or an `edit-pr` title or body, that contains the `@codex` mention also gives `2` (found before anything is written to GitHub; chapter 5). When the diagnostic contains `flag given twice: --reviewer`, the installed tool predates repeated `--reviewer`; reinstalling it with `make install-fixprguard` fixes it. The commit the installed tool was built from can be checked with the `vcs.revision` of `go version -m "$(command -v fixprguard)"` (chapter 3) | None |
| `3` | Ask the user (`inspect` only; there are out-of-scope changes) | The result of `inspect` |

## 5. Subcommands

Each subcommand can also be run on its own. To investigate why something was rejected, run it as shown here with the same arguments as the command document.

### `head [PR]`

Checks that the PR's head is a branch of this repository (only when `isCrossRepository` is the boolean `false` and `headRepository` exists). A fork PR stops with `cross-repository`. `/runpr` runs this before `gh pr checkout`.

```sh
fixprguard head --protocol 3 42
```

Output: `{"protocol":3,"sameRepository":true}`. Side effects: reads GitHub only.

### `list --reviewer LOGIN ... [PR]`

Fetches every page of unresolved threads (at most 2000; beyond that it stops with `thread-cap`) and prints, for each thread, the trust judgment, the reasons, and the fingerprint. It neither fetches nor prints any body. It first makes the same check as `head`.

```sh
fixprguard list --protocol 3 --reviewer coderabbitai 42
```

Output: `threads` (each element has `threadId`, `url`, `trusted`, `fingerprint`, and `findings`) and `permissionLookups` (`used`, the permission lookups used, and `remaining`, the lookups left). The `reason` of a finding is one of the following.

| `reason` | Meaning | Extra field |
|---|---|---|
| `no-write-permission` | A user without write permission | `account` |
| `other-bot` | A Bot matching none of the names passed with `--reviewer` | `account` |
| `no-author-name` | No author, an empty name, or a name outside the login grammar | `commentUrl` |
| `permission-unavailable` | The permission could not be looked up | `cause` (`not-found`, `forbidden`, `rate-limited`, `budget-exceeded`, `rejected-response`, `failed`) |
| `incomplete-comments` | The thread has more than 100 comments, so not all of them can be fetched | None |
| `unknown-author-kind` | The author's kind is neither `User` nor `Bot` | None |

### `begin --reviewer LOGIN ... [--accept ID=FINGERPRINT ...] [PR]`

Phase 1 of `/fixpr`. It runs in this order: (1) the work tree has no uncommitted change, untracked files included (if it has one, it stops with `dirty-work-tree` without querying GitHub); (2) the same check as `head`; (3) recording the baseline; (4) judging the first page (100 threads) and fetching the bodies of the threads to process.

The threads to process are the trusted threads and the threads named with `--accept` whose fingerprint matches the current one. Pass to `--accept` the thread ID and fingerprint from the report output shown to the user.

```sh
fixprguard begin --protocol 3 --reviewer coderabbitai 42
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
fixprguard fence --protocol 3 /path/to/triage.json
```

Side effects: none.

### `inspect --reviewer LOGIN ... --baseline TOKEN [--thread ID=FINGERPRINT ...] [PR]`

Run after the fix phase. It first compares the config, info, and hooks with the baseline, and if they differ, stops with `git-state-changed` without starting `gh`. The comparison runs in this order. (1) Where the git directories are, the type of the config files, and the info are compared from the files alone, without starting git. (2) `git config --list` reads the list of the configuration values git reads, and it is compared without the `branch.<name>.*` settings of other branches (every branch except the current branch and `main`). (3) The hooks are compared. Edits that leave the list of configuration values unchanged (edits of only comments, blank lines, or indentation) are not detected. A rewrite of another branch's settings is a residual risk (7.2). It then fetches the `--thread` threads again from GitHub to get their `path` (a different fingerprint gives `thread-changed`), enumerates the changes in the work tree, and checks them against the scope (chapter 6).

```sh
fixprguard inspect --protocol 3 --reviewer coderabbitai --baseline 'v2....' --thread 'PRRT_...=<fingerprint>' 42
```

Output: `judgedSet` (the judged set ID; pass it to `finish`), `changes` (the number of changed paths), and `outOfScope` (the out-of-scope changes; each element has `position`, `path`, `kind` (`add`, `modify`, `delete`, `other`), `added`, `deleted`, and `binary`). If `outOfScope` is not empty, the exit code is `3`, and the user is shown the list and asked whether to continue. A path that fails the path checks (chapter 6) is not printed in `path`; it is shown by `position` only.

Side effects: reads GitHub and git (including `git ls-remote`) only. The work tree is not changed.

### `finish --baseline TOKEN --judged-set ID PR`

Phase 4 of `/fixpr`. It enforces the following order within one call: (1) the work tree matches the judged set and the git state matches the baseline; (2) `make fmt`; (3) recording the formatted set (it stops if the set of changed paths changed); (4) `make test` and `make lint`; (5) a strict match with the formatted set; (6) staging only the judged paths, using literal pathspecs; (7) checking what was staged (undoing the staging on a mismatch); (8) the commit; (9) checking that the commit's tree matches the formatted set. The make targets are the fixed values `fmt`, `test`, and `lint`, and are never taken from an argument. The commit subject is `fix: address PR #<PR> review comments`.

```sh
fixprguard finish --protocol 3 --baseline 'v2....' --judged-set '<judgedSet>' 42
```

Output: `committed` (`false` when the judged set is empty, in which case nothing is done) and `commit` (the SHA of the commit made; pass it to `push`).

Side effects: the formatting by `make fmt`, the writes the build checks make, staging, and the commit. No push, branch switch, or reset (except undoing the staging).

### `push --baseline TOKEN --commit SHA`

Pushes the SHA that `finish` returned. Right before that, it checks that the hooks, config, and info match the baseline; that HEAD is that SHA and its only parent is the baseline's commit; and that the OID of the push target (the branch of the same name on the remote that git resolves for the current branch) matches the baseline. It pushes with an explicit refspec, without tags or submodules. The config is compared as in `inspect`. The `branch.<name>.*` settings of other branches (every branch except the current branch and `main`) are not compared, and edits that leave the list of configuration values unchanged are not detected. A rewrite of another branch's settings is a residual risk (7.2).

```sh
fixprguard push --protocol 3 --baseline 'v2....' --commit '<SHA>'
```

Output: `{"protocol":3,"pushed":true,"commit":"<SHA>"}`.

### `reply --reviewer LOGIN ... --thread-id ID --comment-id N --fingerprint FP --body-file FILE [--accept ID=FINGERPRINT ...] [PR]`

Judges the thread again in its current state, and posts the reply only when it is a trusted thread (or a thread named with `--accept` whose fingerprint matches), `--fingerprint` matches the current fingerprint, and `--comment-id` is the ID of the thread's first comment. The body is passed to `gh api` as JSON on standard input, so it never goes through a shell. After the reply, it fetches the thread again and checks that nothing changed besides the reply before resolving the thread.

The body file must be a regular file (whose last element is not a symbolic link) of at most 64 KiB holding valid UTF-8 that is not only whitespace and contains no NUL. It must also not contain the `@codex` mention. The comparison is a substring match with ASCII letters compared case-insensitively, and it requires no word boundary (`@codexbot` and `a@codex.example` are rejected too). The mention check runs after the file checks and before the thread is read, and a body that contains it is rejected with exit code `2`. This keeps a post from the user's account from making Codex act.

```sh
fixprguard reply --protocol 3 --reviewer coderabbitai --thread-id 'PRRT_...' --comment-id 123 --fingerprint '<fingerprint>' --body-file /path/to/reply.md 42
```

Output: `{"protocol":3,"written":true}`. When it stops with `reply-not-resolved`, the reply may have been posted, so do not post the same reply again.

### `edit-pr --title-file FILE --body-file FILE [PR]`

Updates the PR's title and body. The title is passed in the form `--title=<value>`, and the body is passed as the checked content on standard input (`gh` does not read the file again). The title file must pass the same checks as the body file and also be one line with no control character and no line separator. When either the title or the body contains the `@codex` mention, it is rejected with exit code `2` before `gh pr edit` is started, by the same rule as the `reply` body.

```sh
fixprguard edit-pr --protocol 3 --title-file /path/to/title.txt --body-file /path/to/body.md 42
```

Output: `{"protocol":3,"written":true}`.

## 6. Scope rules

For each `path` of the threads passed with `--thread` (as fetched again from GitHub), `inspect` decides which files are in scope with the following rules.

1. The `path` itself.
2. Files in the same directory as `path` (not inside subdirectories). However, only files that match the allow list (the table below).
3. If `path` is inside `docs/tasks/NNNN_<name>/`, files under that directory (including inside subdirectories). However, only files that match the allow list.
4. However, a file involved in running commands (an execution-related file; the table below) is out of scope unless it is the `path` itself. For a named untrusted thread, it is out of scope even when it is the `path` itself.
5. For a renamed file, the deletion before the rename and the addition after it are each judged.
6. A change that is not to a regular file (a symbolic link, a submodule, a file type change, a FIFO or device, a file over 8 MiB) is out of scope regardless of its path.

The allow list does not apply to rule 1. A file that is not an execution-related file is in scope when it is the `path` itself, whatever its kind.

**Path checks.** Only paths that are valid UTF-8 with no control character, not absolute, with no `..` element, and unchanged by normalization are judged. Any other path is out of scope, and its `path` is not printed in the output. A path containing a non-ASCII character is in scope only when it matches `path` byte for byte.

**Execution-related files.** The files that rule 4 puts out of scope unless they are the `path` itself (the deny list). They consist of the following three parts.

- The files that the build checks and CI read as configuration, compile, or run. These include the Go build inputs (the files other than `.go` files that Go handles when it builds a package).
- Instruction files: the files that Claude Code and the automated reviewers automatically read as instructions for the directory they are in. These include `CLAUDE.md` and `AGENTS.md` at the repository root.
- Paths with an element (a directory name or a file name) that starts with `.`. Files under `.claude/` and `.github/`, and `.gitignore` and `.gitattributes`, match this in any directory.

Matching treats ASCII upper and lower case as the same (because on a case-insensitive file system `makefile` and `Makefile` refer to the same file). The list is the same as the definition in `internal/fixprguard/scope.go`.

| Form | Items |
|---|---|
| A file with this name in any directory | `makefile`, `gnumakefile`, `go.mod`, `go.sum`, `go.work`, `go.work.sum` |
| An instruction file with this name in any directory | `claude.md`, `agents.md`, `claude.local.md`, `agent.md`, `gemini.md`, `agents.override.md` |
| A file with this extension in any directory | `*.mk` |
| A Go build input with this extension in any directory | `*.c`, `*.h`, `*.cc`, `*.cpp`, `*.cxx`, `*.hh`, `*.hpp`, `*.hxx`, `*.m`, `*.s`, `*.sx`, `*.f`, `*.for`, `*.f90`, `*.swig`, `*.swigcxx`, `*.syso` |
| A path with an element that starts with `.`, in any directory | `.*` (for example: every file under `.claude/` and `.github/`; `.golangci.yml`, `.gitignore`, `.gitattributes`, `.tool-versions`, `.mise.toml`; `extension/.npmrc`, `extension/.node-version`, `extension/.prettierrc.json`, `extension/.prettierignore`) |
| Every file under this directory | `extension/scripts/` |
| A file directly under `extension/` | `extension/package.json`, `extension/package-lock.json`, `extension/eslint.config.js`, `extension/tsconfig.json`, `extension/tsconfig.core.json`, `extension/tsconfig.build.json` |

**Allow list.** The kinds of files that can be in scope under rules 2 and 3. Only a file that is not an execution-related file and matches one of the following is in scope under rules 2 and 3. (a) and (b) are matched case-sensitively (a file named `README.MD` or under `TestData/` does not match). (c) treats upper and lower case as the same, as execution-related files do.

| Form | Items |
|---|---|
| (a) The name ends with this extension (case-sensitive) | `*.md`, `*.txt` |
| (b) A directory element has this name (case-sensitive) | `testdata/` |
| (c) The `.go` rule (below; upper and lower case are treated as the same) | `*.go` |

The `.go` rule: a `.go` file is in scope under rule 2 only when `path` is a `.go` file in the same directory (fixing a test of the same package), and never under rule 3. For a named untrusted thread, every `.go` file other than the `path` itself is out of scope. This is because `make test` compiles and runs the in-scope `.go` files. Only this rule applies to a `.go` file, regardless of (b) (a `.go` file under `testdata` is judged by this rule too).

**Order of judgment.** `inspect` does not print why a change is out of scope. Checking a listed path against the order below and the two tables above tells you why. A change that falls under rule 6 and a path that fails the path checks are out of scope before this order. Every other changed path is checked, for the `path` of one thread, in the following order. If it is in scope for any thread, the change is in scope.

1. If it is an execution-related file, it is in scope only when the thread is not a named one and the changed path is the `path` itself. Otherwise it is out of scope.
2. If the changed path is the `path` itself, it is in scope (rule 1).
3. If the changed path contains a non-ASCII character, it is out of scope.
4. If it is a `.go` file, the `.go` rule decides.
5. If it is in neither the place of rule 2 nor that of rule 3, it is out of scope.
6. If it matches (a) or (b) of the allow list, it is in scope. Otherwise it is out of scope.

## 7. What the tool guarantees and does not

### 7.1 Guarantees

| Guarantee | Mechanism |
|---|---|
| The bodies of untrusted threads are not fetched | The judgment is made with a query that contains no body, and bodies are fetched only for the threads to process. A thread whose comments changed between the judgment and the body fetch is not processed, because the fingerprints do not match |
| A body cannot end the fenced block | The tool checks that the nonce does not occur in the body. `begin`'s `threads` is one line of JSON, so newlines and quotes in a body cannot form a delimiter line |
| Out-of-scope changes are not committed without the user's confirmation | `inspect` lists the out-of-scope changes and returns exit code `3`, and `finish` commits only the changes that match the judged set |
| Changes that appear after the judgment are not committed | `finish` checks the strict match with the judged set before and after the build checks, and stages only the judged paths |
| git is not started with rewritten hooks or config | `inspect`, `finish`, and `push` first compare where the git directories are and the type of the config files from the files alone, without starting git. Next, they start only `git config --list`, which starts no command, and compare the list of configuration values without the `branch.<name>.*` settings of other branches (every branch except the current branch and `main`) with the baseline. The only git started before the config comparison is `git config --list`; if they differ, no other git, `gh`, or make is started. The hooks are compared after the list of configuration values matches, by locating them with `git rev-parse --git-path hooks`, before any command that could run a hook. Edits that leave the list of configuration values unchanged are not detected. A rewrite of another branch's settings is a residual risk (7.2) |
| Text that contains the mention is not sent to GitHub | When a `reply` body or an `edit-pr` title or body contains `@codex` (ASCII letters compared case-insensitively, even as part of a word), the tool rejects it with exit code `2` before anything is written to GitHub. This keeps a post from the user's account from making Codex act. It is a backstop layered on the check the command documents make before posting (added in task 0031) |
| Replies and the PR description do not become shell commands | `reply` and `edit-pr` do not go through a shell, and pass the body on standard input |
| The head of a fork PR is not handled | `head`, `list`, and `begin` check it first |

### 7.2 Not guaranteed

#### 7.2.1 Author judgment and writes to GitHub

- That the set of names passed with `--reviewer` is the same as the names on `Automated reviewer list` in `.claude/commands/_context.md`. The tool does not read `_context.md`. A name left out only makes that Bot's threads untrusted as `other-bot`.
- Re-edits within the same second. The times in the fingerprint have a resolution of one second.
- The content of reply and PR description bodies other than the mention. The tool checks only the file's type, size, and encoding, and the `@codex` mention. A full-width `＠codex`, or a form with a zero-width character between `@` and `codex`, is not a GitHub mention, so it is not checked (whether Codex interprets a body in its own way was not checked). The session can also run `gh` directly with Bash, so the mention check is not a boundary without a way around it.

#### 7.2.2 The fenced block and the command documents

- That an agent does not follow instructions inside the fenced block. Forging the delimiter lines is prevented, but the behavior of an LLM cannot be guaranteed.
- The order in which `inspect` through `finish` are called, the confirmation with the user, and limiting naming to the threads in the immediately preceding report. These depend on the command documents (the session).

#### 7.2.3 The change scope

**Hidden changes.**

- Files ignored by `.gitignore`, files hidden by a `.gitignore` inside the work tree or by the global exclude file, and changes hidden by forging the stat information of the index. Hidden changes are not committed, but the build checks running them cannot be prevented.
- A `.gitignore` that ignores itself (part of the item above). When a `.gitignore` placed in a lower directory ignores itself and another file (such as `foo.syso` or `CLAUDE.md`), neither appears in the set of changes, so the scope rules (chapter 6) do not judge them. The ignored files are not committed, but they remain in the work tree. `finish`'s `make test` links an ignored `.syso` (Go does not read `.gitignore`), and later sessions read an ignored instruction file. Writes to files git ignores are handled by task 0034 (#424).

**What is committed and pushed as in scope.**

- Preventing in-scope test code from being run by the build checks. `finish` and `push` do detect that code rewriting in-scope files or hooks.
- Rewrites of `.md` documents read as instructions. Documents that a command document tells the session to read and follow (this guide, which `/fixpr` reads, the requirements process guide, and so on), the documents `AGENTS.md` points the automated reviewers to, and `docs/dev/security.md`, which PR-Agent reads, are not instruction files, so they are in scope under rules 2 and 3. Instructions written there are committed and pushed, and later sessions on the same branch and the automated reviewers read them. The change appears in the PR diff, so a human checks it before the merge.
- Files under `testdata`. Except for execution-related files and `.go` files (judged by the `.go` rule), they are in scope under rules 2 and 3 whatever their kind ((b) of the allow list). Tests read them, but they do not become code that runs unless a test runs them.
- Files in scope under rule 1. A file that is not an execution-related file is in scope when it is the `path` itself, whatever its kind. For example, when a thread's `path` is a `.sh` file, a rewrite of it is in scope.
- Files read or run that are not on the deny list. The deny list cannot be exhaustive. A file that is in scope under rule 1 and is not an execution-related file can still be read as configuration or instructions by another tool, and this is not prevented. An instruction file whose name is not in the table of chapter 6 but ends in `.md`, and a file read under a `.txt` name (such as `vendor/modules.txt` or `CMakeLists.txt`; none exists in the repository today), match (a) of the allow list, so they are in scope under rules 2 and 3.
- `.go` files of the same package. For a trusted thread whose `path` is a `.go` file, the `.go` rule puts the `.go` files in the same directory in scope, and a cgo preamble (the C code and the `#cgo` directives before `import "C"`) can be written there too. Even for a named thread, a cgo preamble can be written into the `.go` file that is the `path` itself, under rule 1.
- Aliases from the file system's Unicode folding. Matching against the deny list only lowercases the path with `strings.ToLower`, which does not match the folding of a case-insensitive macOS volume. On APFS, `agentſ.md` (U+017F) opens as `AGENTS.md`, and `makeﬁle` (U+FB01) as `makefile`. Under rules 2 and 3, a path containing a non-ASCII character is out of scope, so the effect is limited to rule 1. That path must already be in the PR diff with a thread on it.

**What the user can miss in the out-of-scope list.**

- Disguising a path with bidirectional format characters. The path checks reject control characters but not format characters such as U+202E. In the list of out-of-scope changes `/fixpr` shows, a `.syso` path can look like a `.go` path. The reason a change is out of scope is not printed, so the user judges by looking at the name.
- Habituation to the confirmation. A legitimate change outside the allow list (such as a `.ts` test) goes to the confirmation as out of scope every time. A user who gets used to answering "continue" can miss a dangerous path (an instruction file or a Go build input) in a long list. Check each listed path against the order of judgment and the two tables of chapter 6 to see why it is out of scope.

#### 7.2.4 The git state comparison

- A rewrite of the `branch.<name>.*` settings of other branches (every branch except the current branch and `main`). This is a residual risk. These settings are not compared, so that branch operations of concurrent sessions do not stop the run. A rewrite of another branch's settings by the fix phase or by code that build checks run is not detected, and it remains after the run. It can change the destination or behavior of `git push`, `git pull`, or `git merge` run later on that branch. Running `/fixpr` after switching to that branch makes it the current branch, so a rewrite after `begin` is detected. The git the tool starts during a run does not use other branches' settings to decide a destination. However, a `branch.<name>.remote`, `merge`, or `pushRemote` without a value, or a `[branch ""]` section, makes git fail, and the run stops with `git-failed` or `response-rejected` (chapter 8).
- Edits that leave the list of configuration values unchanged (edits of only comments, blank lines, or indentation). They do not change git's behavior, so they are not detected.
- When an include target is replaced by a FIFO, `git config --list` waits until it times out (2 minutes), so it takes up to 2 minutes until the tool stops with `git-state-changed` (`config`). The 2 minutes equal the default timeout of Claude Code's Bash tool, so the stop can reach the session late.
- The extent to which the git started by `gh` was checked. Given a PR number, `inspect` starts `gh repo view`, the `gh api graphql` of the thread query, and the `gh api` of the permission lookup. Checked with `gh` 2.96.0: the git that `gh repo view` starts is only `git remote -v` and a read of `remote.*`, and the latter two started no git. `git remote -v` reads other branches' settings but does not use them to decide a destination. It fails on a setting without a value or a `[branch ""]` section. The git started by `gh pr view --json number` when the PR number is omitted was not checked (`/fixpr` passes the PR number). This can change with a different `gh` version. A concurrent session's `gh pr checkout` was checked to write only `branch.<that branch>.*` for a regular PR. A fork's PR was not checked. If it also writes other settings, the run only stops too often with `git-state-changed`.
- Only the state at the time of a check is guaranteed. Another process rewriting the hooks or config between the check and the execution, and something changed and changed back during the fix phase, are not detected.
- A comparison before `head`, `list`, `reply`, and `edit-pr`, which take no baseline, start `gh`. `/fixpr` calls them after `finish` and `push`, so the comparisons up to then detect a rewrite, but a rewrite after `push` is not detected.
- Files outside the hooks directory that a hook runs (such as the cache of the pre-commit framework).

## 8. What to Do When the Tool Stops

The `stop` field of the exit code `1` output gives the stop kind. When the tool stops, do not work around it; remove the cause, then redo the round from `begin`.

| `stop` | Cause | Detail fields | What to do |
|---|---|---|---|
| `dirty-work-tree` | At `begin`, the work tree has uncommitted changes (untracked files included) | None | Examine the changes with `git status`, discard or commit them, and redo. When the previous round was stopped by out-of-scope changes, the changes remain, so the run stops here too |
| `cross-repository` | The PR's head is in another repository, such as a fork | None | The tool does not handle it. Check the PR's content yourself |
| `git-state-changed` | One of HEAD, the branch, the push target, the hooks, the config, or the info differs from the baseline. For the config, the list of configuration values without the `branch.<name>.*` settings of other branches (every branch except the current branch and `main`), where the git directories are, and the type of the config files are compared. This includes the case where the list of configuration values can no longer be read after `begin` (a syntax error, a timeout on a FIFO include target, and so on). Edits that leave the list of configuration values unchanged do not stop the run. A rewrite of another branch's settings is a residual risk (7.2) | `parts` (`head`, `remote`, `hooks`, `config`, `info`) | Examine the parts that differ. For `config`, examine the settings with `git config --list --show-origin`, and remove an unintended change to the hooks or config. If the remote only advanced legitimately, clean up the work tree and redo |
| `judged-set-changed` | The work tree or the commit differs from the judged set (a change appeared after the judgment, the build checks rewrote an in-scope file, a commit hook rewrote the index, and so on) | `changes` (the list of changes found) | Examine the listed changes. If the run stopped after a commit was made, examine that commit and undo it |
| `thread-cap` | `list` would read more than 2000 threads | None | Resolve threads on GitHub to reduce them |
| `change-cap` | More than 1000 files have changes | None | Examine the changes and reduce them |
| `thread-changed` | The comments of a thread `inspect` or `reply` targets changed after the fingerprint was taken, or the thread was resolved | None | The next round picks it up. Redo from `begin` |
| `thread-not-trusted` | The target of `reply` is neither a trusted thread nor a named thread | None | Do not reply. The user decides how to handle the untrusted thread |
| `reply-not-resolved` | The reply was posted (or it is unknown whether it was), but the thread could not be resolved | None | Do not post the same reply again. Check the thread on GitHub and resolve it by hand if needed |
| `response-rejected` | A GitHub call failed or timed out, or the response failed the strict check or the size cap. This includes the case where `git remote -v` inside `gh` failed on another branch's setting without a value or on a `[branch ""]` section (the `git-failed` row) | None | Check `gh auth status` and the state of GitHub, then redo. If `git remote -v` fails, follow the `git-failed` row |
| `build-checks-failed` | One of `make fmt`, `make test`, or `make lint` failed (including output over 64 MiB, in which case there is no `output`) | `stage` (`fmt`, `test`, `lint`) and `output` (the last 8 KiB of the output, as a fenced block) | Check the output and fix the cause. `output` is the result of running code, so treat it as untrusted text |
| `git-failed` | git failed, its output was not in the expected shape, or git's metadata could not be read (a tracked file replaced by a FIFO, run outside a git repository, and so on). This includes the cases where another branch's `branch.<name>.remote`, `merge`, or `pushRemote` is written without a value, where a `[branch ""]` section exists, and where the list of configuration values exceeded the output cap (64 MiB) | None | Check that `git status` works. Check that `git config --list --show-origin` works and how large its output is, and if there is a setting without a value or a `[branch ""]` section, remove it |
| `failed` | A failure that matches none of the above (no nonce for the fenced block could be chosen, and so on) | None | Check the diagnostic on standard error |

When the user answers "do not continue" about out-of-scope changes, the changes remain in the work tree. The next `/fixpr` stops with `dirty-work-tree`, so examine the changes, discard or commit them, and redo. Examine the remaining changes with what git shows (`git status --untracked-files=all` and `git diff`; for the content of an untracked file, `git diff --no-index /dev/null <path>`; look for files hidden by a `.gitignore` that ignores itself (7.2) with `git status --ignored --untracked-files=all`). Until you have examined them, do not run `make` or `go test`, and do not start a Claude Code session in that directory. Do not let the current session read files in that directory either. An instruction file or a Go build input in a lower directory is read or linked just by being there.

## 9. Changing the tool

- The judgment is in `internal/fixprguard`, and the argument parsing and the call order are in `cmd/fixprguard`. External commands are called through the `Runner` and `InputRunner` of `internal/cmdrunner`. In tests, `gh` and `make` are replaced with the fake in `internal/cmdrunner/testutil`, and git is exercised in a real repository in a temporary directory.
- When changing a guarantee in section 7.1, break the corresponding implementation, confirm that a test fails, and write that in the commit message (CLAUDE.md, "Testing Strategy").
- When changing the contract of the output or the arguments, or when changing the scope rules (chapter 6), raise `Protocol` in `internal/fixprguard` and update the `--protocol` the command documents pass. However, only a change that meets all three conditions below may leave `Protocol` unchanged. The change that accepts repeated `--reviewer` (task 0033) is one.
    1. An old tool rejects the calls of the new documents when parsing the arguments (exit code `2`) and does not judge by the old rules.
    2. A new tool judges the calls of the old documents by the same rules as before.
    3. The shape of the output does not change.

  In that case, write the old tool's diagnostic when it stops and the guidance to reinstall in the description of exit code `2` in chapter 4.

  Also, a change that only narrows the content a body or title file accepts may leave `Protocol` unchanged regardless of the three conditions above, provided the narrowed part is content the command documents do not send. An old tool accepts that content, but the documents do not send it, so no wrong post results; and the only calls of the old documents that a new tool rejects are those with content that must not be sent. The rejection of the `@codex` mention (task 0031) is one.
- When adding an automated reviewer to the list or removing one from it, change the following in the same commit, and run the document guards (`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`) on that commit. `fixpr.md` and `runpr.md` write neither the names nor their number, and the tool sets no upper limit on the number of names, so neither needs to change.
    1. The `Automated reviewer list` row of `.claude/commands/_context.md`.
    2. The expected names (`wantReviewers`) of the content guard (`cmd/yt2column/fixpr_docs_test.go`).
    3. The names written in the rows of the table of broken copies (`TestContextReviewerRowsMalformed`) in the same file.

  Do not loosen the check of the expected names to something like "parses and is not empty". The content guard would then no longer detect a rewrite of the list alone.
- A Bot whose name is removed from the list does not stop being trusted at once. Each session reads the `_context.md` of the branch it works on, so on an open branch that has not merged `main`, that Bot's threads are still trusted. To withdraw trust quickly, put the removal into `main`, then merge `main` into the branch of each open PR. Until it is merged, do not use `/fixpr` or `/runpr` on that branch.
- When changing the stop kinds, the list of execution-related files, or the allow list, also update chapters 6 and 8 of this guide. `cmd/fixprguard/docs_test.go` checks that this guide (Japanese and English) contains every kind and every item.
- After the merge, reinstall with `make install-fixprguard` (chapter 3).
