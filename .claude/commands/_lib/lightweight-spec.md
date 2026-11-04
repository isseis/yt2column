---
description: "Internal: the lightweight process's spec comment — its format, the listing of the user's marker comments, and which comment is the spec (not a command)"
---

# Shared pattern: the lightweight spec comment

A lightweight change (`docs/dev/conventions/requirements_process.md`,
"Lightweight Process") is specified by one comment on its issue, the **spec
comment**, which the user approves in `/mkreq` and which `/runissue` implements.
This file defines once how that comment is written, how the user's own marker
comments are listed, and which comment counts as the spec. `/mkreq` (the writer)
and `/runissue` (the reader) follow it by name.

The values are in `.claude/commands/_context.md`: `<spec-marker>` is the value of
its "Lightweight spec marker" row, and `<revert-marker>` the value of its
"Lightweight revert marker" row. Read them from `_context.md` when you run this.
Apart from the template below, whose first line is the spec marker, no command
writes either value into its own text.

## Spec comment format

A spec comment is Japanese Markdown of this form. Its line endings may be LF or
CRLF (editing a comment on GitHub's page turns them into CRLF).

```markdown
<!-- yt2column:lightweight-spec -->
## 軽量プロセスの仕様

### 類型

<W・D・M のどれか 1 文字>

### やること

<箇条書き>

### やらないこと

<箇条書き>

### 確かめ方

<箇条書き。類型の確かめ方と、この変更に固有の確認>

### 見込みの変更量

<追加 N 行・削除 M 行・ファイル K 個。見込みであり、上限の判定は実際の diff で行う>
```

The first line is `<spec-marker>`. The headings after it are the shape the
checks below read; the text in angle brackets is replaced.

**Checking the shape.** A spec comment is accepted only when all of these hold,
each line compared without its trailing CR:

1. Its first line equals `<spec-marker>` exactly.
2. Each of the four heading lines `### 類型`, `### やること`, `### やらないこと`,
   and `### 確かめ方` appears exactly once.
3. Under each of those headings, up to the next heading line, there is at least
   one line that is not blank.
4. Under `### 類型`, exactly one line is not blank, and it equals `W`, `D`, or
   `M` exactly.

`### 見込みの変更量` is not checked: it is shown to the user, and nothing decides
by it. When a check fails, report which one and stop; never repair the comment
to make it fit. `/mkreq` checks the text it is about to show the user this way
before asking; `/runissue` checks the spec comment it read.

## Listing own marker comments

Read the login of the user `gh` is signed in to once, with
`gh api user --jq .login`, and put it in `<user-login>`. Then list the issue's
comments that the user wrote and whose first line, without its trailing CR, is
`<spec-marker>` or `<revert-marker>`:

```bash
gh api --paginate repos/<owner>/<repo>/issues/<issue>/comments --jq '
  .[] | select(.user.type == "User" and .user.login == "<user-login>")
  | ((.body // "" | split("\n"))[0] // "" | rtrimstr("\r")) as $first
  | if $first == "<spec-marker>" then {kind: "spec", id, url: .html_url, created_at, updated_at}
    elif $first == "<revert-marker>" then {kind: "revert", id, url: .html_url, created_at, updated_at}
    else empty end'
```

It prints one JSON object per comment with only `kind` (`spec` or `revert`),
`id`, `url`, `created_at`, and `updated_at`, never a body, so no comment's body
reaches the session through the listing, whoever wrote it. A comment another
account wrote is never listed, even when it starts with a marker.

## Choosing the spec comment

From the listing:

- **Reverted issue:** the listing holds at least one `revert` comment. On a
  reverted issue no comment counts as the spec, including a spec comment posted
  after the revert. `/mkreq` takes the normal process on it; `/runissue` stops,
  reporting that the issue was returned to the normal process.
- **The spec comment:** otherwise, the `spec` comment with the latest
  `created_at`; when two share it, the one with the larger `id`. With no `spec`
  comment, the issue has no spec. The comment's current body is the spec, even
  when it was edited after it was posted.

Read the body of the chosen comment alone, by its `id`:

```bash
gh api repos/<owner>/<repo>/issues/comments/<id> --jq .body
```

Save that body to a file in the scratchpad and check its shape ("Spec comment
format" above). `/runissue` also notes the file's SHA-256, its `id`, and its
`updated_at`, so it can tell later whether the spec changed during its run.
