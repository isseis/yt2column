import type { Catalog } from "./catalog.ts";
import { formatCount } from "./locale.ts";

// Steps several job API messages share; each mirrors the English one of the
// same name.
const sameVersionStep =
  "拡張・ホスト・サーバを同じビルドのものにそろえてください。ビルドしたバイナリから yt2column-nmhost install をもう一度実行してください。";
const startServerStep =
  "yt2column-server を起動してください。終了処理中のサーバはジョブの完了を待つので、完了するまで新しいサーバは起動できません。";
const serverPermissionsStep =
  "サーバが動いている場合は、サーバが自分のユーザーアカウントで動いていること、ソケットのディレクトリの権限が README の説明どおりであることを確かめてください。";
const installHostStep =
  "多くの場合、ホストがインストールされていないか、そのバイナリが移動されています。ビルドしたバイナリから yt2column-nmhost install chrome（または brave）を実行してください。バイナリを移動した後も、もう一度実行してください。";
// The server returns the existing article of the same input while it is
// generating or done; any other case generates again, so this is not promised.
const sendSameInputStep =
  "ポップアップから同じ入力をもう一度送信できます。サーバがその記事を生成している間か、生成した記事を持っている間は、生成し直さずにその記事を返します。入力が変わった場合、記事が失敗していた場合、サーバが再起動した場合は、生成し直します。";
const sendAgainStep =
  "ポップアップから同じ入力をもう一度送信すると、記事を生成し直します。";
const repeatedPostStep =
  "この記事の前の版を投稿していた場合、新しい記事を投稿すると Slack で重複します。";
const checksAgainStep = "状態は自動でもう一度取得します。";
const setWebhookStep =
  "SLACK_WEBHOOK_URL を設定して、yt2column-server を起動し直してください。";
const publishUnknownStep = "投稿の状態は自動でもう一度取得します。";
const publishAgainStep =
  "もう一度押しても二重に投稿されないのは、サーバが記事を投稿中か投稿済みとして持っている場合だけです。投稿が失敗した後にやり直すと、届いたことを確かめられなかった 1 通が重複することがあります。";
const closePopupStep =
  "このポップアップは閉じてかまいません。結果はビューアのタブに表示されます。";

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
  jobReject: {
    too_many_requests: {
      text: "サーバはほかの記事を処理しているため、この記事の生成を始めませんでした。",
      steps: ["しばらく待ってから、ポップアップからもう一度送信してください。"],
    },
    body_too_large: {
      text: "リクエストが大きすぎるため、サーバが拒否しました。",
      steps: ["選択する字幕の行を減らして、もう一度試してください。"],
    },
    invalid_body: {
      text: "サーバがリクエストを受け付けませんでした。",
      steps: [
        sameVersionStep,
        "すでに同じビルドのものである場合は、拡張の不具合の可能性があります。",
      ],
    },
    invalid_video_url: {
      text: "サーバが動画の URL を受け付けませんでした。",
      steps: [],
    },
    invalid_transcript: {
      text: "選択したテキストが、文字起こしパネルの字幕の書式ではありません。",
      steps: [
        "文字起こしパネルを開き、字幕の行を選択して、もう一度試してください。",
      ],
    },
    invalid_key: {
      text: "サーバが記事のキーを受け付けませんでした。",
      steps: [
        sameVersionStep,
        "ポップアップから記事をもう一度送信してください。",
      ],
    },
    webhook_not_configured: {
      text: "サーバに Slack の Webhook が設定されていないため、投稿できません。",
      steps: [setWebhookStep],
    },
    article_not_ready: {
      text: "記事がまだできていないため、投稿できません。",
      steps: ["記事が表示されてから投稿してください。"],
    },
    article_not_found: {
      text: "サーバがこの記事をもう持っていないため、投稿できません。",
      steps: [sendAgainStep, repeatedPostStep],
    },
    article_not_postable: {
      text: "サーバがこの記事の投稿を拒否しました。",
      steps: [],
    },
    internal_error: {
      text: "サーバで内部エラーが起きました。",
      steps: ["サーバのログを確かめてください。"],
    },
  },
  startExchange: {
    notSent: {
      "server-unavailable": {
        text: "yt2column のサーバに接続できなかったため、何も送信していません。",
        steps: [startServerStep, serverPermissionsStep],
      },
      "message-too-large": {
        text: "リクエストがホストには大きすぎるため、何も送信していません。",
        steps: ["選択する字幕の行を減らして、もう一度試してください。"],
      },
    },
    noResponse: {
      text: "サーバが時間内に応答しませんでした。サーバはリクエストを受け取り、記事を生成している可能性があります。",
      steps: [sendSameInputStep],
    },
    hostFailed: {
      text: "Native Messaging のホストを起動できなかったか、ホストが応答せずに終了しました。リクエストはすでに送信された可能性があります。",
      steps: [installHostStep, sendSameInputStep],
    },
    unexpected: {
      text: "応答を解釈できませんでした。サーバはリクエストを受け取り、記事を生成している可能性があります。",
      steps: [sameVersionStep, sendSameInputStep],
    },
  },
  statusExchange: {
    notSent: {
      "server-unavailable": {
        text: "状態の取得のために yt2column のサーバに接続できませんでした。",
        steps: [startServerStep, checksAgainStep],
      },
      "message-too-large": {
        text: "ホストが状態の取得のリクエストを大きすぎるとして拒否したため、状態の取得を止めました。",
        steps: [
          sameVersionStep,
          "ポップアップから記事をもう一度送信してください。",
        ],
      },
    },
    noResponse: {
      text: "サーバが状態の取得に時間内に応答しませんでした。",
      steps: [checksAgainStep],
    },
    hostFailed: {
      text: "状態の取得のための Native Messaging のホストを起動できなかったか、ホストが応答せずに終了しました。",
      steps: [installHostStep, checksAgainStep],
    },
    unexpected: {
      text: "状態の取得の応答を解釈できなかったため、状態の取得を止めました。",
      steps: [
        sameVersionStep,
        "ポップアップから記事をもう一度送信してください。",
      ],
    },
  },
  publishExchange: {
    notSent: {
      "server-unavailable": {
        text: "yt2column のサーバに接続できなかったため、記事を投稿していません。",
        steps: [startServerStep, "その後、もう一度投稿してください。"],
      },
      "message-too-large": {
        text: "ホストが投稿のリクエストを大きすぎるとして拒否したため、記事を投稿していません。",
        steps: [sameVersionStep],
      },
    },
    noResponse: {
      text: "サーバが投稿のリクエストに時間内に応答しませんでした。記事を投稿している可能性があります。",
      steps: [publishUnknownStep, publishAgainStep],
    },
    hostFailed: {
      text: "Native Messaging のホストを起動できなかったか、ホストが応答せずに終了しました。投稿のリクエストはすでに送信された可能性があります。",
      steps: [installHostStep, publishUnknownStep, publishAgainStep],
    },
    unexpected: {
      text: "投稿のリクエストの応答を解釈できませんでした。サーバは記事を投稿している可能性があります。",
      steps: [sameVersionStep, publishUnknownStep, publishAgainStep],
    },
  },
  startResult: {
    accepted: {
      text: "サーバが記事を受け付けました。進み具合はビューアのタブに表示されます。",
      steps: [],
    },
    noResult: {
      text: "送信の結果が届かなかったため、サーバが記事の生成を始めたかどうか分かりません。",
      steps: [sendSameInputStep],
    },
  },
  failure: {
    internal: {
      text: "記事の生成中にサーバで失敗が起きました。",
      steps: ["サーバのログを確かめてください。", sendAgainStep],
    },
    transcript_too_long: {
      text: "文字起こしがプロンプトに対して長すぎます。",
      steps: [
        "選択する字幕の行を減らして、ポップアップからもう一度送信してください。",
      ],
    },
    llm_timeout: {
      text: "LLM の呼び出しが時間切れになりました。",
      steps: [sendAgainStep],
    },
    llm_empty: {
      text: "LLM が空の応答を返しました。",
      steps: [sendAgainStep],
    },
    llm_truncated: {
      text: "LLM の応答が途中で切れました。",
      steps: [
        sendAgainStep,
        "繰り返す場合は、サーバのログを確かめてください。",
      ],
    },
    llm_failed: {
      text: "LLM の呼び出しが失敗しました。",
      steps: [
        "サーバのログで、プロバイダのエラーを確かめてください。",
        sendAgainStep,
      ],
    },
    article_invalid: {
      text: "生成した記事が、完了の時点の検査に通りませんでした。",
      steps: [sendAgainStep],
    },
    article_too_large: {
      text: "生成した記事が大きすぎて表示できません。",
      steps: [
        "選択する字幕の行を減らして、ポップアップからもう一度送信してください。",
      ],
    },
  },
  stepNames: {
    draft: "下書きの生成",
    shorten: "字数の調整",
    review: "検証・推敲",
  },
  stepStates: {
    pending: "未着手",
    running: "実行中",
    done: "完了",
    skipped: "省略",
    failed: "失敗",
  },
  publishStates: {
    not_posted: { text: "まだ Slack に投稿していません。", steps: [] },
    posting: { text: "Slack に投稿しています…", steps: [] },
    posted: { text: "Slack に投稿しました。", steps: [] },
    failed: {
      text: "前回の Slack への投稿は、終わる前に止まりました。",
      steps: [
        "もう一度投稿すると、届かなかった最初のメッセージから送ります。届いたことを確かめられなかった 1 通は、2 度届くことがあります。",
      ],
    },
    unpostable: { text: "この記事は Slack に投稿できません。", steps: [] },
  },
  publishReasons: {
    mention: {
      text: "記事がメンションを含むため、サーバは投稿しません。",
      steps: [],
    },
    other: {
      text: "投稿の前のサーバの検査が、記事を拒否しました。",
      steps: [],
    },
    webhook: {
      text: "Slack がメッセージを受け取ったと返しませんでした。",
      steps: [],
    },
    internal: {
      text: "投稿のジョブが異常に終わりました。",
      steps: ["サーバのログを確かめてください。"],
    },
  },
  viewer: {
    sending: { text: "リクエストをサーバに送信しています…", steps: [] },
    loading: { text: "記事の状態を読み込んでいます…", steps: [] },
    invalidLink: {
      text: "このビューアのリンクは正しくないため、記事を表示しません。",
      steps: ["ポップアップから記事をもう一度送信してください。"],
    },
    running: "記事を生成しています",
    lost: {
      text: "サーバがこの記事をもう持っていません。サーバが再起動したか、記事をキャッシュから捨てました。",
      steps: [sendAgainStep, repeatedPostStep],
    },
    webhookNotConfigured: {
      text: "サーバに Slack の Webhook が設定されていません。",
      steps: [setWebhookStep],
    },
    videoLink: "YouTube で動画を開く",
    elapsed: "経過時間",
    duration: "所要時間",
    cost: "推定の料金",
    costUnknown: "不明",
    publish: "Slack に投稿する",
    save: ".md で保存する",
    saved: "ダウンロードを始めました。",
    copy: "Markdown をコピーする",
    copied: "Markdown をコピーしました。",
    copyFailed: "Markdown をコピーできませんでした。",
  },
  popup: {
    primary: {
      generate: "記事を生成する",
      openViewer: "ビューアを開く",
      retry: "やり直す",
    },
    sending: {
      text: "リクエストをサーバに送信しています…",
      steps: [closePopupStep],
    },
    viewerNotice: { text: closePopupStep, steps: [] },
    articleReady: { text: "記事ができました。", steps: [] },
  },
};
