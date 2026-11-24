<!-- translated-from: README.md 7df3f99b9e73a5009bcd927bd9b629c55f88d279 -->
[日本語](README.md)

# yt2column

yt2column turns a YouTube video into a magazine-column-style article with an LLM. When you open the video's transcript panel in a browser with the extension, select the caption lines, and start generation from the popup, the article appears in the viewer tab. You can save the article as a `.md` file or copy its Markdown to the clipboard.

> **Status:** early development; no release yet.

## How it works

Article generation completes only inside the browser. The extension collects the selected captions and the video's information, and the article generation module built into the extension (Go code compiled to WebAssembly) calls the LLM API in the viewer tab to create the article. Neither a server nor a native messaging host is needed for article generation, and nothing besides the browser needs to be installed.

The API key and the material for the article go only to `https://api.anthropic.com`. Nothing is sent to the developers.

## Requirements

- Chrome 142 or later, or Brave on Chromium 142 or later
- An Anthropic API key

How to prepare an API key and the estimated cost per article are in [INSTALL.md](INSTALL.md).

## Installing and using

The steps from getting the extension to saving a first article are in [INSTALL.md](INSTALL.md). INSTALL.md is written in Japanese only for now.

## Privacy

**What is sent:** to generate an article, the extension sends the selected captions, the video URL, and the video's title, channel name, and description only to `https://api.anthropic.com`. Nothing is sent to the developers. The developers do not provide a server that relays the API.

**API key and workspace ID:** they are saved only in `chrome.storage.local` of this browser profile and are not synced to other devices. However, processes that run as the same OS user can read the saved values.

**The article generation record:** the state, the steps, and the finished article are kept in the browser's memory and disappear when the browser exits. They are not written to disk.

**Browser history:** the viewer URL holds the video URL and the page title and stays in the browser's history. The history may be passed to other devices through browser sync, and other extensions with the `tabs` or `history` permission can read that history.

**A crafted viewer URL:** a viewer whose URL values were crafted can show any heading and title. The heading does not guarantee the article's content.

**Article files:** the extension writes an article file only when the user chooses to save it.

## Developer documents

There is also a CLI for developers. How article generation works is described in [Article generation details](docs/dev/guides/generation.md), how to run the CLI is in [Using the CLI](docs/dev/guides/cli.md), and the development procedures are in [Development procedures](docs/dev/guides/development.md).
