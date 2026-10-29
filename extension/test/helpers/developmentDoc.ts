/**
 * The developer guide the extension tests read and the locators they find its
 * rejection-reason table by. The guide is Japanese only, so each locator is
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
} as const;
