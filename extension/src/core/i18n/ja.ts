import type { Catalog } from "./catalog.ts";
import { formatCount } from "./locale.ts";

// Button names a step quotes, kept in one place with the button's own name.
const generateButton = "記事を生成する";
const generateFromSelectionButton = "現在の選択範囲で生成する";
const openExistingButton = "既存の記事を表示する";
const setupButton = "API キーを設定する";
// Steps several messages share; each mirrors the English one of the same name.
const pressAgainStep = `動画のタブでポップアップを開き、「${generateButton}」をもう一度押してください。`;
const fewerLinesStep =
  "選択する字幕の行を減らして、動画のタブでポップアップを開き、ボタンをもう一度押してください。";
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
        "もう一度ヨムドウガを起動してください。",
      ],
    },
    "empty-selection": {
      text: "テキストが選択されていません。",
      steps: [
        "「文字起こしを表示」で文字起こしパネルを開いてください。",
        "字幕の行を選択してください。",
        "もう一度ヨムドウガを起動してください。",
      ],
    },
    "empty-title": { text: "タブにタイトルがありません。", steps: [] },
    "collection-failed": {
      text: "このタブから選択範囲を読み取れませんでした（ページが変わった可能性があります）。",
      steps: [],
    },
    "settings-unprotected": {
      text: "保存した設定をウェブページから読めないようにできなかったため、このタブを読み取りませんでした。",
      steps: [
        "このポップアップを閉じて、開き直してください。",
        "繰り返す場合は、拡張を再読み込みしてください。",
      ],
    },
  },
  preflight: {
    "body-too-large": {
      text: "選択したテキストが大きすぎます（4 MiB を超えています）。",
      steps: ["選択する字幕の行を減らして、もう一度試してください。"],
    },
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
  startRejected: {
    invalid_body: {
      text: "選択したテキストがこの動画の字幕として受け付けられなかったため、記事の生成を始めませんでした。",
      steps: [
        "文字起こしパネルを開き、字幕の行を選択して、もう一度試してください。",
      ],
    },
    too_many_requests: {
      text: "すでに 2 つの記事を生成しているため、この記事の生成を始めませんでした。",
      steps: [
        "どちらかの生成が終わるまで待ってから、もう一度ボタンを押してください。",
      ],
    },
    internal: {
      text: "拡張の内部で失敗が起きたため、記事の生成を始めませんでした。",
      steps: [pressAgainStep, "繰り返す場合は、拡張を再読み込みしてください。"],
    },
  },
  startUnknown: {
    text: "開始の結果が届かなかったため、記事の生成が始まったかどうか分かりません。",
    steps: [
      "ビューアのタブで記事の状態を確かめてください。",
      "同じ選択範囲でもう一度ボタンを押しても、その記事が処理中か完成している間は、二重に生成しません。それ以外のときは、新しく記事の生成を始めます。",
    ],
  },
  inputUnavailable: {
    text: "ビューアが選択した字幕を受け取れなかったため、記事の生成を始めていません。料金はかかっていません。",
    steps: [pressAgainStep],
  },
  setup: {
    api_key_missing: {
      text: "記事を生成するには、Anthropic の API キーの設定が必要です。",
      steps: [
        `「${setupButton}」を押して、API キーを保存してください。`,
        pressAgainStep,
      ],
    },
    settings_invalid: {
      text: "保存した API キーかワークスペース ID の値が、使える形ではありません。",
      steps: [
        `「${setupButton}」を押して、API キーとワークスペース ID を保存し直してください。`,
        pressAgainStep,
      ],
    },
  },
  keyRejected: {
    text: "Anthropic が API キーを拒みました。キーの期限切れ、失効、誤りか、組織がブラウザからの呼び出しを許していない可能性があります。",
    steps: [
      `「${setupButton}」を押して、新しい API キーを保存してください。`,
      `${pressAgainStep}同じ選択範囲なら、同じジョブのまま記事を生成し直します。`,
    ],
  },
  failure: {
    internal: {
      text: "記事の生成中に、拡張の内部で失敗が起きました。",
      steps: [pressAgainStep],
    },
    transcript_too_long: {
      text: "文字起こしがプロンプトに対して長すぎます。",
      steps: [fewerLinesStep],
    },
    llm_timeout: {
      text: "LLM の呼び出しが時間切れになりました。",
      steps: [pressAgainStep],
    },
    llm_empty: {
      text: "LLM が空の応答を返しました。",
      steps: [pressAgainStep],
    },
    llm_truncated: {
      text: "LLM の応答が途中で切れました。",
      steps: [pressAgainStep],
    },
    llm_failed: {
      text: "LLM の呼び出しが失敗しました。",
      steps: [
        `しばらく待ってから、動画のタブでポップアップを開き、「${generateButton}」をもう一度押してください。`,
        "失敗が続くときは、Claude Console で利用の上限と使用量を確かめてください。",
      ],
    },
    article_invalid: {
      text: "生成した記事が、完了の時点の検査に通りませんでした。",
      steps: [pressAgainStep],
    },
    article_too_large: {
      text: "生成した記事が大きすぎて保持できません。",
      steps: [fewerLinesStep],
    },
    run_timeout: {
      text: "50 分たっても記事が完成しなかったため、生成を止めました。",
      steps: [pressAgainStep],
    },
  },
  stopped: {
    text: "記事の生成がページとともに止まりました。生成していたビューアのタブが閉じられたか読み込み直されたか、拡張がそのタブと連絡を取れなくなりました。",
    steps: [pressAgainStep],
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
    interrupted: "中断",
  },
  viewer: {
    preparing: { text: "記事の生成を準備しています…", steps: [] },
    loading: { text: "記事の状態を読み込んでいます…", steps: [] },
    invalidLink: {
      text: "このビューアのリンクは正しくないため、記事を表示しません。",
      steps: [pressAgainStep],
    },
    running: "記事を生成しています",
    lost: {
      text: "この記事はもう保持されていません。ブラウザが終了したか、拡張が再読み込みか更新されたか、新しい記事のために捨てられました。",
      steps: [
        "上のリンクから動画を開き、字幕の行を選択して、記事を生成し直してください。",
      ],
    },
    unstored: {
      text: "記事は完成しましたが、拡張が保持できませんでした。記事を持っているのはこのページだけで、読み込み直すか閉じると失われます。",
      steps: ["今のうちに保存するかコピーしてください。"],
    },
    existing: {
      text: "このビューアは、同じ動画の既存の記事を表示しています。現在の選択範囲から生成した記事ではありません。",
      steps: [
        `現在の選択範囲で生成するには、動画のタブでポップアップを開き、「${generateButton}」を押してください。`,
        `選択肢が表示されたら、「${generateFromSelectionButton}」を選んでください。`,
      ],
    },
    videoLink: "YouTube で動画を開く",
    elapsed: "経過時間",
    duration: "所要時間",
    cost: "推定の料金",
    costUnknown: "不明",
    save: ".md で保存する",
    saved: "ダウンロードを始めました。",
    copy: "Markdown をコピーする",
    copied: "Markdown をコピーしました。",
    copiedNeutralized:
      "Markdown をコピーしました。メンションにならないよう、@ の後ろに見えない文字を入れました。",
    copyFailed: "Markdown をコピーできませんでした。",
  },
  options: {
    saved: { text: "API キーを設定しました。", steps: [] },
    saved_unverified: {
      text: "API キーを設定しましたが、Anthropic でキーを確かめられませんでした。",
      steps: [
        "後で Anthropic がキーを拒んで記事の生成が失敗したときは、ここで新しいキーを設定してください。",
      ],
    },
    field_rejected: {
      text: "印を付けた欄の値は受け付けられない形です。何も保存していません。",
      steps: [
        "API キーは、Anthropic が示すとおりに、空白を入れずに入力してください。",
        "ワークスペース ID は、空のままにするか、空白を入れずに入力してください。",
      ],
    },
    key_rejected: {
      text: "Anthropic がこの API キーを拒みました。何も保存していません。",
      steps: [
        "キーの期限が切れていないか、失効していないかを確かめるか、Claude Console で新しいキーを作ってください。",
        "組織がブラウザからの呼び出しを許していないときは、このキーをここでは使えません。",
      ],
    },
    check_unavailable: {
      text: "設定を確かめられませんでした。何も保存していません。",
      steps: ["もう一度保存してください。"],
    },
    write_uncertain: {
      text: "設定を保存できなかったかもしれません。",
      steps: [
        "API キーが設定されていると表示されているかを確かめてください。",
        "もう一度保存してください。",
      ],
    },
    deleted: {
      text: "API キーとワークスペース ID を削除しました。",
      steps: [],
    },
    delete_uncertain: {
      text: "設定を削除できなかったかもしれません。",
      steps: [
        "API キーが設定されていると表示されているかを確かめてください。",
        "もう一度削除してください。",
      ],
    },
  },
  viewerTitle: {
    name: { release: "ヨムドウガ", dev: "ヨムドウガ（開発用のビルド）" },
    invalidLink: "正しくないリンク",
    untitled: "タイトルのない動画",
  },
  openOptions: {
    link: "設定",
    setup: setupButton,
  },
  buildMark: "開発用のビルド",
  optionsPage: {
    title: "ヨムドウガの設定",
    heading: "設定",
    apiKey: "Anthropic の API キー",
    apiKeyHint:
      "キーはこのブラウザにだけ保存し、再び表示しません。スクリーンリーダーは、入力した文字を読み上げることがあります。",
    workspaceId: "ワークスペース ID（任意）",
    workspaceIdHint:
      "空のままにすると、API キーが属するワークスペースを使います。",
    save: "保存する",
    remove: "削除する",
    keyState: {
      set: "API キーが設定されています。",
      notSet: "API キーが設定されていません。",
      unknown: "API キーが設定されているかを読み取れませんでした。",
    },
    buildInfo: {
      heading: "ビルド",
      commit: "コミット",
      goVersion: "Go の版",
      llm: "LLM",
      modified: "（変更あり）",
      none: "なし",
      unreadable: "ビルドの情報を読み取れませんでした。",
    },
  },
  popup: {
    primary: {
      generate: generateButton,
      openViewer: "ビューアを開く",
      retry: "やり直す",
      openExisting: openExistingButton,
    },
    secondary: generateFromSelectionButton,
    sending: {
      text: "記事の生成を始めています…",
      steps: [closePopupStep],
    },
    viewerNotice: { text: closePopupStep, steps: [] },
    articleReady: { text: "記事ができました。", steps: [] },
    choice: {
      text: "拡張に、同じ動画の記事があります。",
      steps: [
        `「${openExistingButton}」でその記事を表示します。「${generateFromSelectionButton}」で、選択した字幕から新しく記事を生成します。`,
      ],
    },
    existingStates: {
      loading: "記事の状態を読み込んでいます…",
      running: "記事は生成中です。",
      done: "記事は生成済みです。",
    },
    choiceEnded: {
      text: "同じ動画の記事は使えなくなりました。",
      steps: [`「${generateButton}」を押すと、記事を生成します。`],
    },
  },
};
