# Package Reference

This document lists the packages under `cmd/` and `internal/` and the
responsibility of each. Update it in the same commit that adds, removes, or
changes the responsibility of a package.

The planned layout and the responsibility of each not-yet-created package are
described in [project_overview.md](../project_overview.md) ("想定ディレクトリ構成" and
"パイプライン").

| Package | Responsibility |
|---|---|
| `internal/secret` | Holds a secret (API key, Webhook URL) and guarantees it never appears in `fmt`, `log/slog`, or JSON output |
