import type { Catalog } from "./catalog.ts";
import { formatCount } from "./locale.ts";

/**
 * The Japanese catalog. Its terms follow docs/translation_glossary.md, and
 * the YouTube button names are the labels observed on YouTube in Japanese.
 */
export const ja: Catalog = {
  rejection: {
    "not-watch-page": {
      text: "YouTube の動画ページではありません。",
      steps: [
        "https://www.youtube.com/watch?v=... のような動画ページを開いてください。",
        "文字起こしパネルを開き、字幕の行を選択してください。",
        "もう一度 yt2column を起動してください。",
      ],
    },
    "empty-selection": {
      text: "テキストが選択されていません。",
      steps: [
        "「文字起こしを表示」で文字起こしパネルを開いてください。",
        "字幕の行を選択してください。",
        "もう一度 yt2column を起動してください。",
      ],
    },
    "empty-title": { text: "タブにタイトルがありません。", steps: [] },
    "collection-failed": {
      text: "このタブから選択範囲を読み取れませんでした（ページが変わった可能性があります）。",
      steps: [],
    },
  },
  preflight: {
    "body-too-large": {
      text: "選択したテキストが大きすぎます（4 MiB を超えています）。",
      steps: ["選択する字幕の行を減らして、もう一度試してください。"],
    },
  },
  serverReject: {
    too_many_requests: {
      text: "サーバは別の記事を処理しています。",
      steps: [
        "送り直す前に、前回の送信の通知が Slack に届いていないか確かめてください。",
        "しばらく待ってから、もう一度送信してください。",
      ],
    },
    body_too_large: {
      text: "リクエストの本文が大きすぎるため、サーバが拒否しました。",
      steps: [
        "選択する字幕の行を減らして、もう一度試してください。",
        "サーバが Slack に失敗の通知を投稿している場合もあります。",
      ],
    },
    invalid_body: {
      text: "サーバがリクエストの本文を受け付けませんでした（拡張の不具合の可能性があります）。",
      steps: ["サーバが Slack に失敗の通知を投稿している場合もあります。"],
    },
    invalid_video_url: {
      text: "サーバが動画の URL を受け付けませんでした。",
      steps: ["サーバが Slack に失敗の通知を投稿している場合もあります。"],
    },
    invalid_transcript: {
      text: "選択したテキストが、文字起こしパネルの字幕の書式ではありません。",
      steps: [
        "文字起こしパネルを開き、字幕の行を選択して、もう一度試してください。",
        "サーバが Slack に失敗の通知を投稿している場合もあります。",
      ],
    },
    internal_error: {
      text: "サーバで内部エラーが起きました。",
      steps: [
        "サーバのログを確かめてください。",
        "サーバが Slack に失敗の通知を投稿している場合もあります。",
      ],
    },
  },
  notSent: {
    "server-unavailable": {
      text: "yt2column のサーバに接続できなかったため、何も送信していません。",
      steps: [
        "yt2column-server を起動してください。終了処理中のサーバはジョブの完了を待つので、完了するまで新しいサーバは起動できません。",
        "サーバが動いている場合は、サーバが自分のユーザーアカウントで動いていること、ソケットのディレクトリの権限が README の説明どおりであることを確かめてください。",
      ],
    },
    "message-too-large": {
      text: "リクエストがホストには大きすぎるため、何も送信していません。",
      steps: ["選択する字幕の行を減らして、もう一度試してください。"],
    },
  },
  accepted: {
    text: "サーバが記事を受け付けました。記事の生成と投稿の結果は Slack に届きます。",
    steps: [],
  },
  noResponse: {
    text: "サーバが時間内に応答しませんでした。サーバはリクエストを受け取り、処理している可能性があります。",
    steps: [
      "送り直す前に、記事または失敗の通知が Slack に届いていないか確かめてください。",
      "何も届かない場合は、この時刻の前後のサーバのログを確かめてください。",
    ],
  },
  hostFailed: {
    text: "Native Messaging のホストを起動できなかったか、ホストが応答せずに終了しました。",
    steps: [
      "多くの場合、ホストがインストールされていないか、そのバイナリが移動されています。ビルドしたバイナリから yt2column-nmhost install chrome（または brave）を実行してください。バイナリを移動した後も、もう一度実行してください。",
      "リクエストはすでに送信された可能性があります。送り直す前に、記事または失敗の通知が Slack に届いていないか確かめてください。",
    ],
  },
  unexpected: {
    text: "応答を解釈できませんでした。サーバはリクエストを受け取り、処理している可能性があります。",
    steps: [
      "ホスト・サーバ・拡張が同じビルドのものであることを確かめてください。ビルドしたバイナリから yt2column-nmhost install をもう一度実行してください。",
      "送り直す前に、記事または失敗の通知が Slack に届いていないか確かめてください。",
    ],
  },
  sending: {
    text: "記事をサーバに送信しています…",
    steps: [
      "結果が表示されるまで、このポップアップを閉じないでください。",
      "閉じた場合は、送り直す前に Slack を確かめてください。",
    ],
  },
  expandPrompt: {
    text: "折りたたまれているため、冒頭だけが送られます。",
    steps: [
      "動画の下の「もっと見る」をクリックしてください。",
      "このポップアップを開き直してください。",
    ],
  },
  checkNames: {
    selection: "選択した字幕",
    description: "概要欄",
  },
  checkStates: {
    ok: "問題なし",
    warning: "注意",
  },
  selectionSize: (characters, lines) =>
    `${formatCount(characters, "ja")} 文字・${formatCount(lines, "ja")} 行`,
  descriptionFull: (characters) =>
    `全文・${formatCount(characters, "ja")} 文字`,
  descriptionNotSent: {
    text: "取得できないため、送られません。",
    steps: [],
  },
  details: {
    summary: "詳細",
    url: "URL",
    tabTitle: "タブのタイトル",
  },
  primaryButton: "記事を生成する",
};
