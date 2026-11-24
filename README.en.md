<!-- translated-from: README.md abe3ff1a3ea2bfcd8cea5345eee4ffea2617b250 -->
[日本語](README.md)

# yt2column

yt2column turns a YouTube video into a magazine-column-style article with AI. When you open the video's transcript panel in a browser with the extension, select the caption lines, and start generation from the popup, the article appears in the viewer tab. You can save the article as a `.md` file or copy its Markdown to the clipboard.

> **Status:** early development; no release yet.

## How it works

The extension collects the selected captions and the video's information, and uses your API key to have Anthropic's AI write the article. The result appears in the viewer tab.

The API key and the material for the article go only to Anthropic (the company that provides Claude).

## Requirements

- Chrome 142 or later, or Brave 1.84 or later
- An Anthropic API key

How to prepare an API key and the estimated cost per article are in [INSTALL.md](INSTALL.md).

## Installing and using

The steps from getting the extension to saving a first article are in [INSTALL.md](INSTALL.md). INSTALL.md is written in Japanese only for now.

## Privacy

**What is sent:** to create an article, the extension sends the selected captions, the video URL, and the video's title, channel name, and description only to Anthropic.

**API key and workspace ID:** they are saved only in `chrome.storage.local` of this browser profile and are not synced to other devices. The popup and the service worker never read the API key's value. However, processes that run as the same OS user can read the saved values.

**The article generation record:** the state, the steps, and the finished article are kept in the browser's memory and disappear when the browser exits. They are not saved to disk.

**Browser history:** the viewer URL holds the video URL and the page title and stays in the browser's history. The history may be passed to other devices through browser sync, and other extensions with the `tabs` or `history` permission can read that history.

**Article files:** an article file is saved only when the user asks for it.

## Developer documents

There is also a CLI for developers. How article generation works is described in [Article generation details](docs/dev/guides/generation.md), how to run the CLI is in [Using the CLI](docs/dev/guides/cli.md), and the development procedures are in [Development procedures](docs/dev/guides/development.md).
