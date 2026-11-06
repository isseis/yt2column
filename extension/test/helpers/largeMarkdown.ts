// Inputs as long as the largest response the extension accepts, shared by
// the time test of the parse and the size test of the build.

/** Repeats unit until the string is length characters long. */
export function repeatTo(unit: string, length: number): string {
  return unit.repeat(Math.ceil(length / unit.length)).slice(0, length);
}

/** One line of backtick runs that grow by one: `a``a```a and so on. */
function growingBacktickRuns(length: number): string {
  let line = "";
  for (let run = 1; line.length < length; run++) {
    line += `${"`".repeat(run)}a`;
  }
  return line.slice(0, length);
}

const articleParagraph =
  "# Title\n\n## Section\n\n**Speaker**: a *word* and `code` <https://a.example/x>.\n- item\n1. item\n\n```\ncode\n```\n\n";

/**
 * The inputs of size characters that would take a parse searching again for
 * each opening mark or line far longer than the time limit, and a line and
 * lines of article text. Each is built only when its build is called.
 */
export function largeInputs(
  size: number,
): readonly { name: string; build: () => string }[] {
  return [
    { name: "a line of stars", build: () => "*".repeat(size) },
    { name: "a line of backticks", build: () => "`".repeat(size) },
    {
      name: "a line of autolink openings",
      build: () => repeatTo("<https://", size),
    },
    { name: "a line of backslashes", build: () => "\\".repeat(size) },
    {
      name: "a line of unclosed strong emphasis",
      build: () => repeatTo("**a", size),
    },
    { name: "a line of unclosed emphasis", build: () => repeatTo("*a", size) },
    {
      name: "a line of strong emphasis closings after spaces",
      build: () => repeatTo("**a ", size),
    },
    {
      name: "a line of emphasis closings after spaces",
      build: () => repeatTo("*a ", size),
    },
    {
      name: "a line of code spans of one length",
      build: () => repeatTo("`a", size),
    },
    {
      name: "a line of growing backtick runs",
      build: () => growingBacktickRuns(size),
    },
    {
      name: "lines of unclosed fences",
      // The last line is padded with letters, so no line closes a fence.
      build: () => {
        const lines = "```x\n".repeat(Math.floor((size - 1) / 5));
        return lines + "a".repeat(size - lines.length);
      },
    },
    {
      name: "a line mixing the marks",
      build: () => repeatTo("**a *b `c <https://d \\", size),
    },
    {
      name: "lines of article paragraphs",
      build: () => repeatTo(articleParagraph, size),
    },
  ];
}
