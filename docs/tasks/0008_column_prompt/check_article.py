#!/usr/bin/env python3
"""Mechanical checks for task 0008's evaluation (01_requirements.md F-006).

Usage: python3 -I check_article.py <cache dir> <video ID> <article file>...

Each article file is the output of `yt2column --out`. The video title and
duration come from the info.json in the cache slot that <id>.current names,
the same entry the CLI read when it generated the article. The script prints
one Markdown table row per article; the LLM and the human judge the rest.

Length counts the code points of the body part (the LLM output without its
title line and without the source block the system appends), excluding line
breaks. Markdown markup such as "## " and "**" is counted.
"""

import json
import re
import sys
from pathlib import Path

TITLE_MAX = 40
LONG_VIDEO_SECONDS = 15 * 60
LONG_MIN, BODY_MAX = 2500, 4000
SECTIONS_MIN, SECTIONS_MAX = 3, 5

HEADER_RE = re.compile(
    r"\A# (?P<title>[^\n]*)\n\n- Model: (?P<model>[^\n]*)\n- Model version: (?P<version>[^\n]*)\n\n"
)
SOURCE_RE = re.compile(r"\n\n出典: <[^>\n]+>\n\Z")
URL_RE = re.compile(r"https?://|www\.|youtube\.com|youtu\.be", re.IGNORECASE)
# Pictographic emoji blocks. Kaomoji are left to the LLM.
EMOJI_RE = re.compile("[\U0001F000-\U0001FAFF☀-➿⬀-⯿️]")


def load_info(cache_dir: Path, video_id: str) -> dict:
    slot = (cache_dir / f"{video_id}.current").read_text(encoding="utf-8")
    if slot not in ("a", "b"):
        sys.exit(f"{video_id}.current holds {slot!r}, not a or b")
    info_path = cache_dir / f"{video_id}.{slot}" / f"{video_id}.info.json"
    return json.loads(info_path.read_text(encoding="utf-8"))


def outside_fences(body: str) -> list[str]:
    lines, in_fence = [], False
    for line in body.split("\n"):
        if line.startswith("```"):
            in_fence = not in_fence
            continue
        if not in_fence:
            lines.append(line)
    return lines


def check(path: Path, video_title: str, duration: float) -> list[str]:
    text = path.read_text(encoding="utf-8")
    header = HEADER_RE.match(text)
    source = SOURCE_RE.search(text)
    if header is None or source is None:
        return [path.name, "unrecognized --out format"]
    title = header["title"]
    body = text[header.end() : source.start()]
    lines = outside_fences(body)

    problems = []
    if len(title) > TITLE_MAX:
        problems.append(f"title is {len(title)} chars (AC-06)")
    if title == video_title:
        problems.append("title equals the video title (AC-07)")

    first_heading = next((i for i, l in enumerate(lines) if l.startswith("#")), None)
    if first_heading is None or not any(l.strip() for l in lines[:first_heading]):
        problems.append("no lead before the first heading (AC-06)")
    sections = sum(1 for l in lines if l.startswith("## "))
    if not SECTIONS_MIN <= sections <= SECTIONS_MAX:
        problems.append(f"{sections} level-2 sections (AC-06)")
    if any(re.match(r"#(?!# )", l) for l in lines if l.startswith("#")):
        problems.append("a heading other than level 2 in the body (AC-06)")

    length = len(body.replace("\r", "").replace("\n", ""))
    if duration >= LONG_VIDEO_SECONDS and not LONG_MIN <= length <= BODY_MAX:
        problems.append(f"body is {length} chars, outside {LONG_MIN}-{BODY_MAX} (AC-10)")
    if duration < LONG_VIDEO_SECONDS and length > BODY_MAX:
        problems.append(f"body is {length} chars, over {BODY_MAX} (AC-11)")

    if EMOJI_RE.search(body):
        problems.append("emoji (AC-05)")
    if URL_RE.search(body):
        problems.append("URL (AC-13)")

    verdict = "pass" if not problems else "; ".join(problems)
    return [path.name, header["model"], header["version"], title, str(sections), str(length), verdict]


def main() -> None:
    if len(sys.argv) < 4:
        sys.exit(__doc__.split("\n\n")[1])
    cache_dir, video_id = Path(sys.argv[1]), sys.argv[2]
    info = load_info(cache_dir, video_id)
    duration = info.get("duration")
    if not isinstance(duration, (int, float)):
        sys.exit(f"{video_id}: info.json has no numeric duration")
    print("| file | model | model version | title | sections | length | mechanical checks |")
    print("|---|---|---|---|---|---|---|")
    for name in sys.argv[3:]:
        cells = check(Path(name), info.get("title", ""), duration)
        print("| " + " | ".join(c.replace("|", "\\|") for c in cells) + " |")


if __name__ == "__main__":
    main()
