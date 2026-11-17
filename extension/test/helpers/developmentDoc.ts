/**
 * The developer guide the extension tests read and the locators they find its
 * job API code tables by. The guide is Japanese only, so each locator is one
 * Japanese string that must equal the guide's text.
 */
export const developmentDoc = {
  path: "docs/dev/guides/development.md",
  /** The section that holds the job API's code tables. */
  jobCodesHeading: "### ジョブの API のコード",
  /** Each code table's first header cell; its first column lists the codes. */
  jobCodeTables: {
    jobState: "ジョブの状態",
    step: "工程",
    stepState: "工程の状態",
    failure: "失敗",
  },
} as const;
