#!/bin/sh
# Checks the structure of docs/dev/conventions/package_reference.md:
#
#   1. the document has exactly six "## " kind headings, and each heading is
#      followed by exactly one table, whose header is Package|Responsibility;
#   2. every package name in the first column of a table appears on exactly
#      one row (a duplicated row fails);
#   3. the document's byte count is printed, and with a maximum on the command
#      line it must not exceed that maximum.
#
# Usage:
#
#	docs/tasks/0085_package_reference_summary/_checks/check_state.sh [MAX_BYTES]
#
# Run it from inside the repository, on the PR branch whose document it
# checks. MAX_BYTES is optional: the size limit applies only to the run that
# checks the finished document. make test and CI do not run it. It reads no
# environment variable and names no secret.
#
# Output and exit codes:
#   0  every check passed (the counts, the byte count, and a result line)
#   1  a check failed (the reason on stderr)
#   2  the arguments or the repository are unusable (a MAX_BYTES that is not a
#      non-negative integer, no repository, or a missing document)
set -eu
export LC_ALL=C

doc=docs/dev/conventions/package_reference.md

if [ "$#" -gt 1 ]; then
	echo "usage: $0 [MAX_BYTES]" >&2
	exit 2
fi
if [ "$#" -eq 1 ]; then
	case "$1" in
	'' | *[!0-9]*)
		echo "MAX_BYTES is not a non-negative integer: $1" >&2
		exit 2
		;;
	esac
fi

if ! root=$(git rev-parse --show-toplevel 2>/dev/null); then
	echo "not a git repository" >&2
	exit 2
fi
if [ ! -f "$root/$doc" ]; then
	echo "missing $doc" >&2
	exit 2
fi

# The section check and the duplicate-row check, in one pass: a table starts
# at a line whose first cell is Package, a package row is a table line whose
# first cell holds a code span, and a kind section ends at the next "## "
# heading.
structure() {
	awk '
		function finish_section() {
			if (section == "") return
			if (tables != 1) {
				printf "section %s has %d tables, want 1\n", section, tables
				bad = 1
			}
		}
		function first_cell(line,   cell) {
			cell = line
			sub(/^\|/, "", cell)
			sub(/\|.*$/, "", cell)
			return cell
		}
		function plain(cell) {
			gsub(/`/, "", cell)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", cell)
			return cell
		}
		function package_name(cell,   first, last) {
			first = index(cell, "`")
			if (!first) return ""
			cell = substr(cell, first + 1)
			last = index(cell, "`")
			if (!last) return ""
			return substr(cell, 1, last - 1)
		}
		/^## / {
			finish_section()
			sections++
			section = $0
			tables = 0
			in_table = 0
			next
		}
		{
			line = $0
			sub(/^[[:space:]]+/, "", line)
			is_row = substr(line, 1, 1) == "|"
			if (is_row && !in_table) {
				# A block of table lines is one table; its first line is the
				# header, which must be exactly the package table header.
				tables++
				header = line
				sub(/^\|/, "", header)
				sub(/\|$/, "", header)
				cells = split(header, parts, "|")
				for (i = 1; i <= cells; i++) parts[i] = plain(parts[i])
				if (cells != 2 || tolower(parts[1]) != "package" || tolower(parts[2]) != "responsibility") {
					printf "table header in %s is not Package|Responsibility: %s\n", section, line
					bad = 1
				}
			}
			in_table = is_row
			if (!is_row) next
			cell = first_cell(line)
			if (plain(cell) == "Package") next
			if (cell ~ /^[-: ]+$/) next
			if (section == "") {
				print "a table appears before the first kind heading"
				bad = 1
				next
			}
			name = package_name(cell)
			if (name == "") {
				printf "row without a package name: %s\n", line
				bad = 1
				next
			}
			rows++
			if (name in seen) {
				printf "package %s appears on more than one row\n", name
				bad = 1
			}
			seen[name] = 1
		}
		END {
			finish_section()
			if (sections != 6) {
				printf "found %d kind headings, want 6\n", sections
				bad = 1
			}
			printf "sections: %d\nrows: %d\n", sections, rows
			exit bad ? 1 : 0
		}
	' "$root/$doc"
}

ret=0
if ! output=$(structure 2>&1); then
	printf '%s\n' "$output" >&2
	ret=1
else
	printf '%s\n' "$output"
fi

bytes=$(wc -c <"$root/$doc" | tr -d ' ')
echo "bytes: $bytes"
if [ "$#" -eq 1 ] && [ "$bytes" -gt "$1" ]; then
	echo "$bytes bytes exceeds the $1-byte limit" >&2
	ret=1
fi

if [ "$ret" -ne 0 ]; then
	echo "FAIL: the document structure is not as required"
	exit 1
fi
echo "OK: six kind headings, one table each, no duplicate package row"
