import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { markdownSection } from "./helpers/repoFile.ts";

describe("markdownSection", () => {
  const doc = [
    "## Chapter",
    "### Target",
    "inside",
    "#### Sub",
    "still inside",
    "### Next",
    "after",
  ].join("\n");

  const cases: {
    name: string;
    markdown: string;
    heading: string;
    want: string;
  }[] = [
    {
      name: "stops at the next level-3 heading and keeps a level-4 one",
      markdown: doc,
      heading: "### Target",
      want: "inside\n#### Sub\nstill inside\n",
    },
    {
      name: "stops at a level-2 heading",
      markdown: "### Target\ninside\n## Chapter two\nafter",
      heading: "### Target",
      want: "inside\n",
    },
    {
      name: "runs to the end without a following heading",
      markdown: "### Target\ninside",
      heading: "### Target",
      want: "inside",
    },
    {
      name: "does not match a heading that only starts with the given one",
      markdown: "### Target and more\ninside",
      heading: "### Target",
      want: "",
    },
    {
      name: "returns empty for a missing heading",
      markdown: doc,
      heading: "### Missing",
      want: "",
    },
  ];
  for (const { name, markdown, heading, want } of cases) {
    it(name, () => {
      assert.equal(markdownSection(markdown, heading), want);
    });
  }
});
