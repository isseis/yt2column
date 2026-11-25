#!/bin/sh
# One-off migration check: no user-facing text of the extension still names the
# old product. Run from the top level of the work tree.
set -u

files="
extension/src/core/i18n/ja.ts
extension/src/core/i18n/en.ts
extension/static/_locales/ja/messages.json
extension/static/_locales/en/messages.json
extension/static/popup.html
extension/static/options.html
extension/static/viewer.html
"

status=0
for file in $files; do
  if [ ! -f "$file" ]; then
    echo "old_name_absent: cannot read $file" >&2
    status=2
    continue
  fi
  matches=$(grep -n 'yt2column' "$file")
  grep_status=$?
  case $grep_status in
    0)
      echo "old_name_absent: leftover in $file"
      printf '%s\n' "$matches"
      status=1
      ;;
    1) ;;
    *)
      echo "old_name_absent: cannot search $file" >&2
      status=2
      ;;
  esac
done

case $status in
  0) echo "old_name_absent: PASS" ;;
  1) echo "old_name_absent: FAIL" >&2 ;;
esac
exit "$status"
