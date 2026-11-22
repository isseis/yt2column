# Developer Tools

These are the rules an agent follows when it adds a developer tool or changes
one. This document is the source of the rules; their reasons are in
[ADR-0014](../adr/0014-deterministic-checks-in-go.md), and this document links
there instead of restating a reason. Other documents hold copies of a rule
(`CLAUDE.md`, "Tool Execution Safety", and the per-tool guides, for example);
change this document before a copy.

## Kinds of tools

A **developer tool** is a Go tool for the development procedure: a
`cmd/<name>` and `internal/<name>` pair, not part of the product binaries. An
external CLI, such as `coderabbit`, is not one.

An **installed tool** is a developer tool that the user installs from `main`
with `make install-<name>`. The list of installed tools and their install
commands is in `CLAUDE.md`, "Build Commands".

A **per-run tool** is a developer tool that a make target builds into a
temporary directory each time it runs it. The list is in `CLAUDE.md`:
`make coverage-compare` under "Test Commands" and `make ext-coverage-compare`
under "Extension Commands".

## Rule 1: Write a tool in Go, with tests

Write a tool that makes a decision and has a side effect in Go, with tests.
Make it a `cmd/<name>` and `internal/<name>` pair, and keep it out of the
product binaries.

## Rule 2: Do not read a decision's baseline back from a state file

A tool does not read the baseline of its decision back from a state file. It
returns the baseline and the sets it decided as the output of a subcommand,
and the session passes them as arguments of the next call. A tool may write a
record for the user as long as it does not read that record back to decide.

There is an exception. This document states only that an exception exists; its
scope is in [ADR-0014](../adr/0014-deterministic-checks-in-go.md) and
[ADR-0046](../adr/0046-chief-of-staff.md). Examples of this rule are in
[fixprguard_guide.md](../guides/fixprguard_guide.md): chapter 1 (a tool that
writes a record without reading it back) and chapter 7 (what the tool
guarantees).

## Rule 3: Enforce the order in the tool's code

Enforce the order of a procedure in the tool's code, not in the command
definition. The command definition keeps only the order in which it calls the
subcommands and the confirmations it asks the user. Examples are in
[fixprguard_guide.md](../guides/fixprguard_guide.md): the explanations of
`finish` and `push` in chapter 5, and chapter 7.

## Rule 4: Arguments, output, and exit codes are a contract

A tool's arguments, output, and exit codes are a contract that the command
definitions and the document guards read. When a change alters one, change the
tool, the documents that call it, and the guards in the same PR. Change the
documents and the tool in an order that keeps both working while the PR is
open ([fixprguard_guide.md](../guides/fixprguard_guide.md), chapter 9).

The contract is written in `CLAUDE.md` ("Build Commands", "Test Commands",
"Development Notes"), in the "Guard tool" row of
[_context.md](../../../.claude/commands/_context.md), and in the guides of the
installed tools: [fixprguard_guide.md](../guides/fixprguard_guide.md),
[mergepr_guide.md](../guides/mergepr_guide.md),
[chief_architecture.md](../chief_architecture.md), and
[numreserve_guide.md](../guides/numreserve_guide.md).

## Rule 5: Install tools from main; agents never install or build them

### Who installs

The user installs an installed tool from an up-to-date `main` checkout with
`make install-<name>`, and reinstalls it after a merge that changed the tool.
A PR that raises the protocol number of a tool is the exception to the
checkout: the user reinstalls that tool from the PR's branch before that PR's
`/runpr`
([fixprguard_guide.md](../guides/fixprguard_guide.md), chapter 3). The
documents and the tool compare versions with the `--protocol` number; the rule
for protocol numbers is in `CLAUDE.md`, "Protocol numbers".

### What agents do not do

An agent never installs or builds an installed tool. To build means to make a
binary that runs the tool, or to run the tool from source: `make
install-<name>`, `go install`, `go build ./cmd/<name>`, and `go run
./cmd/<name>` are builds. Compiling a tool's package for tests or lint
(`make test`, `make lint`, `go test`, `go vet`) is not.

### Development exception

To develop a tool, or to investigate one, an agent may build it from source and
run it, under three conditions:

1. It puts no built binary where `PATH` finds it, `$(go env GOPATH)/bin`
   included.
2. It does not use the run in place of the installed tool inside a procedure,
   such as `/fixpr`, `/runpr`, `/mergepr`, `/chief`, or `/mkreq`.
3. The run has no external side effect. A test, `--help`, and a run on local
   input or on a copy qualify. These do not: writing to the network or to
   GitHub, creating a record tag, typing into another agent's pane, and
   sending to an outside service (an LLM API included). A run
   that acts on shared state (chief's state directory, a Herdr workspace or
   pane, a worktree, `$(go env GOPATH)/bin`) is not a run on local input or on
   a copy.

The other rules of `CLAUDE.md`, "Tool Execution Safety", still apply inside
this exception.

### When a step needs a tool

Stop and ask the user in two situations: a step comes to a point where it
would install or build an installed tool, or a command fails because a tool is
missing or its number does not match.

A procedure that decides otherwise wins. Today this does:

- When a number does not match, `/fixpr`, `/runpr`, and `/mergepr` show two
  fixes and stop: reinstall the tool from `main` when the tool is old, or
  merge `origin/main` into the branch when the branch is old (`/runpr`'s
  "not a branch of this repository" step, `/fixpr`'s "A tool of another
  protocol" bullet, and the paragraph of `/mergepr` that follows the
  install step).

To ask, name the tool and the command the user runs, and say in which
checkout to run it. The command is normally `make install-<name>` in an
up-to-date `main` checkout; add that the user does not run it in the task's
worktree, which has the PR's branch checked out. Ask for the PR's branch only
when the work in progress is a PR that is not yet merged and that changes the
tool's `const Protocol` line; then the command is `make install-<name>` in
that PR's branch's checkout. A tool whose number is not being raised is
installed from `main` even when another tool's number is.

Where to continue after the user has installed the tool is decided by the
procedure, not by this document.

### Per-run tools

A per-run tool is not subject to this section.

### The permission layer

In Claude Code, `.claude/settings.json` denies `make install-*` and
`go install`. The denial is one layer of the guard; it does not allow a form
it does not match (`go build`, `go run`, or a spelling the rule does not
match). It also covers `go install` of an outside Go tool.

### Cloud sessions

In a cloud session an agent also installs or builds nothing. It stops and
asks the user, and the user runs the command in a local clone.

### Links

- [CLAUDE.md](../../../CLAUDE.md), "Tool Execution Safety"
- [CLAUDE.md](../../../CLAUDE.md), "Protocol numbers"

## Rule 6: Exception: a side-effect-free record check may be a shell script

A decision that only reads record lines and has no side effect may be a shell
script independent of the command definitions, under four conditions:

1. It is placed under `.claude/commands/_lib/`.
2. The procedure that calls it fetches it from an `origin/main` commit and runs
   it from outside the work tree.
3. A failure is the safe verdict.
4. The whole of the rule is verified by a permanent Go test, and the test is
   confirmed to fail when the rule is broken.

Condition 4 applies to scripts added after this exception existed. The
treatment of the scripts that existed before it is in
[ADR-0014](../adr/0014-deterministic-checks-in-go.md).

## Not covered here

These rules live elsewhere:

- Protocol numbers: `CLAUDE.md`, "Protocol numbers".
- The scope of the chief exception: [ADR-0014](../adr/0014-deterministic-checks-in-go.md)
  and [ADR-0046](../adr/0046-chief-of-staff.md).
- Finding the range of an extension function in TypeScript:
  [ADR-0014](../adr/0014-deterministic-checks-in-go.md).
- Where to read an environment variable: [ADR-0024](../adr/0024-env-access-in-config.md).
- `context.Context` as the first argument and the like:
  [ADR-0019](../adr/0019-pipeline-component-interfaces.md).
