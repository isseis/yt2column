<!-- translated-from: README.md 83b30756e7691841636f7df7d49c0eef51884ba9 -->
[日本語](README.md)

# yt2column

yt2column is a browser extension that summarizes the content of a YouTube video into a magazine-column-style article you can read in a short time. This extension is free. However, the AI service that creates the articles charges a usage fee. You pay the fee directly to the company that provides the AI (Anthropic). You can read the article in the browser, and you can also save it as a file or copy it to the clipboard.

> **Status:** early development. It has not been released yet, so it is not publicly available. The installation steps for after the release are collected in [INSTALL.md](INSTALL.md) in preparation for it.

## How it works

The extension collects the part of the video's transcript (the audio turned into text) that you selected, together with the video's information, and has an AI write the article using Anthropic's paid service. The result is shown in a dedicated tab of the browser.

Because an AI writes the article, its content may differ from the video. Check anything important against the video.

## Requirements

- Chrome 142 or later, or Brave 1.84 or later
- An Anthropic API key (a workspace ID is also needed, depending on how the key is created)

An API key is your own key for using Anthropic's AI, and you create it on Anthropic's website. If someone else learns it, they can use the AI at your expense, so treat it as carefully as a password. Depending on how the API key is created, you also need a workspace ID, which you create on Anthropic's website.

Every time you use the AI, Anthropic charges you for what you used. The estimated cost per article is about 0.4 US dollars (it rises as the transcript you select gets longer). This is a usage fee for the API, separate from the monthly Claude chat subscription (claude.ai).

How to create an API key and a workspace ID, how to pay, and settings that keep you from overspending are in [INSTALL.md](INSTALL.md).

## Installing and using

The flow of use is as follows.

1. Follow [INSTALL.md](INSTALL.md) to create an API key at Anthropic (including preparing to pay), install the extension, and save the API key (only the first time).
2. On a YouTube video page, open the transcript panel and select the lines shown in the panel.
3. Click the yt2column icon in the browser's toolbar to start generating the article.
4. The article appears in a dedicated tab. You can save it as a text file you can open with Notepad or similar (Markdown format, `.md`), or copy it to the clipboard. The article disappears when you quit the browser, so if you want to keep it, save or copy it before quitting.

The detailed steps from getting the extension to saving the first article are explained in [INSTALL.md](INSTALL.md).

## Privacy

To create an article, the extension sends the transcript you selected, the video URL, the video's title, channel name, description, and so on to Anthropic. Your API key and workspace ID are also sent, to identify the usage as yours. What is sent is stored on Anthropic's servers for a certain period and then deleted. According to Anthropic, content sent through the API is, by default, not used to train AI. For the latest handling, such as the storage period, check Anthropic's published information ([storage period](https://privacy.claude.com/en/articles/7996866), [use for training](https://privacy.claude.com/en/articles/7996868), both in English).

The API key and the workspace ID are saved only inside the browser you use. However, other software running under the same user on the same computer may be able to read the saved values.

Unless you tell it to save, an article does not remain as a file and disappears when you quit the browser. When you do save it, it goes to the browser's download location.

The address of the tab that shows the article contains the video URL and the title, so it is kept in the browser history. If you use browser history sync, it is also passed to your other devices. Other extensions with permission to read your browsing history can also see it.

## Developer documents

There is also a version for developers that you use from the command line rather than the browser. How article generation works is described in [Article generation details](docs/dev/guides/generation.md), how to run the CLI is in [Using the CLI](docs/dev/guides/cli.md), and the development procedures are in [Development procedures](docs/dev/guides/development.md).
