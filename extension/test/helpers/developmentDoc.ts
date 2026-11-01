/**
 * The developer guide the extension tests read and the locators they find its
 * rejection-reason table and its job API code tables by. The guide is Japanese only, so each locator is
 * one Japanese string that must equal the guide's text.
 */
export const developmentDoc = {
  path: "docs/dev/guides/development.md",
  /** The rejection-reason table's header cells, in order. */
  rejectionColumns: ["reason", "ステータスコード", "拡張の表示"],
  /** How the table's last column writes each category of display. */
  display: {
    fixedMessage: "固定のメッセージ",
    unexpectedReply: "想定外の応答",
    fixedMessageUnlessNoRequestId:
      "固定のメッセージ（`requestId` が空のときは想定外の応答）",
  },
  /** The section that holds the job API's code tables. */
  jobCodesHeading: "### ジョブの API のコード",
  /** Each code table's first header cell; its first column lists the codes. */
  jobCodeTables: {
    jobState: "ジョブの状態",
    step: "工程",
    stepState: "工程の状態",
    failure: "失敗",
    publishState: "投稿の状態",
    publishReason: "投稿の理由",
  },
} as const;
