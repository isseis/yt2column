/**
 * The developer guide the extension tests read and the locators they find its
 * job record value tables by. The guide is Japanese only, so each locator is
 * one Japanese string that must equal the guide's text.
 */
export const developmentDoc = {
  path: "docs/dev/guides/development.md",
  /** The section that holds the job record's value tables. */
  jobCodesHeading: "### ジョブの記録の値",
  /** Each value table's first header cell; its first column lists the values. */
  jobCodeTables: {
    jobState: "ジョブの状態",
    step: "工程",
    stepState: "工程の状態",
    failure: "失敗",
    internalDetail: "内部の失敗の種類",
    stopReason: "止まった理由",
  },
} as const;
