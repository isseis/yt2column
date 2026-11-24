<!-- translated-from: README.md be42dd3fba84b2e98430aadb7e59e13134361827 -->
[日本語](README.md)

Developer documents: [Article generation details](docs/dev/guides/generation.md) · [Using the CLI](docs/dev/guides/cli.md) · [Development procedures](docs/dev/guides/development.md)

# yt2column

yt2column turns a YouTube video into a magazine-column-style article with an
LLM. The primary path is complete within the browser extension: the extension
collects the captions the user selected in YouTube's transcript panel, and
the article generation module in the extension (Go code built to WebAssembly)
calls the Anthropic API inside the viewer tab to generate the article. The API
key is set on the extension's options page and saved only in that browser. The
article is shown in the extension's viewer tab, and saving it as a `.md` file
and copying its Markdown to the clipboard are the user's choices in the
viewer.

- The extension needs nothing installed besides the browser: no server and no
  host for generating articles.

> **Status:** early development; no release yet.

## Quick start

The steps from getting the extension to saving a first article are in
[INSTALL.md](INSTALL.md). INSTALL.md is written in Japanese only for now.

If you used the earlier version (the one with the localhost server and the
native messaging host), do the
[migration from the earlier version](#migrating-from-the-earlier-version)
before you update the extension.

## Requirements

**For the primary path (browser extension):**

- Chrome 142 or later, or Brave on Chromium 142 or later
- An Anthropic API key (see
  [INSTALL.md](INSTALL.md#使う前に必要なもの))

## Privacy

The API key and the workspace ID are saved only in `chrome.storage.local` of
this browser profile. They survive a browser restart and are not synced to other
devices. The only place the extension sends the API key to is
`https://api.anthropic.com`; the popup and the service worker do not read the
API key's value. However, processes that run as the same OS user can read the
profile's files.

The extension keeps a record of each article generation (its state, steps, and
the finished article) in the browser's memory and does not write it to disk. The
viewer URL holds the collected URL and the page title and stays in the browser's
history. A viewer whose URL values were crafted can show any heading and title,
so the heading does not vouch for the article's content.

## Migrating from the earlier version

The earlier version generated articles with the localhost server
(`yt2column-server`) and the native messaging host (`yt2column-nmhost`). The
current extension uses neither. Before you update the extension, migrate with
these steps.

1. Stop the running `yt2column-server`.
2. With the earlier version's host binary, remove the host's manifest from each
   browser you used (the commands below). When you cannot run it, for example
   because the binary is gone, remove the manifest files below by hand.
3. Rebuild and reload the extension (see
   [Building and loading the extension](docs/dev/guides/development.md#拡張のビルドと読み込み)),
   and save the API key on the options page (see
   [API key and workspace ID](INSTALL.md#api-キーとワークスペース-id)).
4. Once you have made one article with the extension alone, delete the server's
   configuration file `~/.config/yt2column/config.env`. This file holds a
   plaintext copy of the API key. Until you delete it, you can go back to the
   earlier version's server and host. If the same directory has files whose
   names start with `.config.env.tmp-`, delete them too: they are temporary
   files left when `config init` was stopped partway, and one can point to the
   same content as the configuration file.

```sh
./build/yt2column-nmhost uninstall chrome
./build/yt2column-nmhost uninstall brave
```

The manifest files to remove by hand:

- Chrome and Brave on macOS: `$HOME/Library/Application Support/Google/Chrome/NativeMessagingHosts/io.github.isseis.yt2column.json`
- Chrome on other Unix: `$HOME/.config/google-chrome/NativeMessagingHosts/io.github.isseis.yt2column.json`
- Brave on other Unix: `$HOME/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts/io.github.isseis.yt2column.json`

Even if the shared token and port (`sharedToken`, `serverPort`) an earlier
version saved in the browser are left, the current extension neither reads nor
deletes them. To delete them, remove the extension once on
`chrome://extensions/` (Brave: `brave://extensions/`), then load
`extension/dist` again. Removing the extension also deletes the saved API key,
so save it again.
