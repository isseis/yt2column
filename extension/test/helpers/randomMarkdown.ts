// Seeded pseudo-random display Markdown for the property tests of the parse
// and the build. The generator is fixed here (mulberry32), so a seed and an
// index name one input for good, and a failure can be rerun from its report.
//
// Text that must be Japanese, full-width, or a bidi control is written with
// \u escapes so the source stays ASCII; each value says what it is.

/** The seeds the property tests run, each for inputsPerSeed inputs. */
export const randomSeeds = [0x0063, 0x5eed, 0xc0ffee] as const;
export const inputsPerSeed = 500;
export const maxInputLength = 300;

/** mulberry32: a 32-bit state, each call returning a number in [0, 1). */
function mulberry32(seed: number): () => number {
  let state = seed >>> 0;
  return () => {
    state = (state + 0x6d2b79f5) >>> 0;
    let t = state;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// The pieces an input is made of: the marks of the rules, the characters a
// URL is made of, whole autolinks, and the characters the parse must keep as
// ordinary text.
const pieces: readonly string[] = [
  "*",
  "**",
  "`",
  "```",
  "~~~",
  "<",
  ">",
  "\\",
  "#",
  "# ",
  "## ",
  "-",
  "- ",
  "+ ",
  "* ",
  "1",
  "9",
  "0",
  ".",
  ". ",
  ")",
  "~",
  "&",
  "[",
  "]",
  " ",
  "   ",
  "\t",
  "\n",
  "\n\n",
  "\r",
  "/",
  "a",
  "x",
  "@",
  ":",
  "%",
  "?",
  "https://",
  "<https://",
  "xn--",
  "<https://a.example/>",
  "<https://xn--nxasmq6b.com/>",
  "<https://user@a.example/>",
  "<https://a.example/?q=1#f>",
  "記事", // Japanese "article"
  "。", // ideographic full stop
  "　", // ideographic space
  "ｅ", // full-width small letter e
  "＊", // full-width asterisk
  "א", // Hebrew letter alef
  "‮", // right-to-left override
  "⁧", // right-to-left isolate
  "\u0007", // bell, a control character
];

/** The inputsPerSeed inputs of seed, each of length 0 to maxInputLength. */
export function randomMarkdownInputs(seed: number): string[] {
  const next = mulberry32(seed);
  const pick = (count: number): number => Math.floor(next() * count);
  const inputs: string[] = [];
  for (let index = 0; index < inputsPerSeed; index++) {
    const target = pick(maxInputLength + 1);
    let input = "";
    while (input.length < target) {
      input += pieces[pick(pieces.length)] ?? "";
    }
    inputs.push(input.slice(0, target));
  }
  return inputs;
}
