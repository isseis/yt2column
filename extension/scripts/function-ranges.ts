// Reports the line ranges of the functions and instance field groups in
// extension product files, so the Go side can attach lcov's FN records, which
// carry no end line, to a range of source lines. The ranges come from
// TypeScript's syntax tree; the caller names the files relative to --root.
import { readFileSync } from "node:fs";
import path from "node:path";
import ts from "typescript";

/** One line range, 1-based and inclusive. */
interface LineRange {
  readonly start: number;
  readonly end: number;
}

/** The range helper's answer for one requested file. */
interface FileRanges {
  readonly path: string;
  readonly functions: LineRange[];
  readonly fieldGroups: LineRange[];
}

/** The output: one entry per requested file, in the requested order. */
export interface FunctionRanges {
  readonly files: FileRanges[];
}

/** What the command line named: the tree to read and the files in it. */
interface RangeArguments {
  readonly root: string;
  readonly files: string[];
}

/** Parses "--root <dir> -- <path>..."; anything else is undefined. */
function parseArguments(argv: string[]): RangeArguments | undefined {
  if (argv.length < 4 || argv[0] !== "--root" || argv[2] !== "--") {
    return undefined;
  }
  const root = argv[1];
  if (root === undefined || root === "") {
    return undefined;
  }
  return { root, files: argv.slice(3) };
}

/** Reads the function ranges of every requested file under root. */
export function collectFunctionRanges(
  root: string,
  files: string[],
): FunctionRanges {
  const resolvedRoot = path.resolve(root);
  return { files: files.map((file) => readFileRanges(resolvedRoot, file)) };
}

/** Reads one file under root and reports its ranges. */
function readFileRanges(root: string, file: string): FileRanges {
  const full = path.resolve(root, file);
  const relative = path.relative(root, full);
  if (
    relative === "" ||
    relative === ".." ||
    relative.startsWith(`..${path.sep}`) ||
    path.isAbsolute(relative)
  ) {
    throw new Error(`path is outside --root: ${file}`);
  }
  const text = readFileSync(full, "utf8");
  const source = ts.createSourceFile(
    file,
    text,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS,
  );
  const functions: LineRange[] = [];
  const fieldGroups: LineRange[] = [];
  const visit = (node: ts.Node): void => {
    if (isFunctionWithBody(node)) {
      functions.push(lineRange(source, node));
    }
    if (ts.isClassDeclaration(node) || ts.isClassExpression(node)) {
      const group = instanceFieldGroup(source, node);
      if (group !== undefined) {
        fieldGroups.push(group);
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return {
    path: file,
    functions: sortRanges(functions),
    fieldGroups: sortRanges(fieldGroups),
  };
}

/** The function-like nodes, which all have a body except a signature. */
type FunctionNode =
  | ts.FunctionDeclaration
  | ts.FunctionExpression
  | ts.ArrowFunction
  | ts.MethodDeclaration
  | ts.ConstructorDeclaration
  | ts.GetAccessorDeclaration
  | ts.SetAccessorDeclaration;

/** Reports whether a node is a function-like node with a body. */
function isFunctionWithBody(node: ts.Node): node is FunctionNode {
  if (!(
    ts.isFunctionDeclaration(node) ||
    ts.isFunctionExpression(node) ||
    ts.isArrowFunction(node) ||
    ts.isMethodDeclaration(node) ||
    ts.isConstructorDeclaration(node) ||
    ts.isGetAccessorDeclaration(node) ||
    ts.isSetAccessorDeclaration(node)
  )) {
    return false;
  }
  return node.body !== undefined;
}

/** The lines of a node, from its first token to its last, without leading comments. */
function lineRange(source: ts.SourceFile, node: ts.Node): LineRange {
  const start = source.getLineAndCharacterOfPosition(
    node.getStart(source),
  ).line;
  const end = source.getLineAndCharacterOfPosition(node.getEnd() - 1).line;
  return { start: start + 1, end: end + 1 };
}

/** The range of a class's instance field declarations, first to last. */
function instanceFieldGroup(
  source: ts.SourceFile,
  node: ts.ClassDeclaration | ts.ClassExpression,
): LineRange | undefined {
  const fields = node.members.filter(
    (member): member is ts.PropertyDeclaration =>
      ts.isPropertyDeclaration(member) && !isStatic(member),
  );
  const first = fields[0];
  const last = fields[fields.length - 1];
  if (first === undefined || last === undefined) {
    return undefined;
  }
  return {
    start: lineRange(source, first).start,
    end: lineRange(source, last).end,
  };
}

/** Reports whether a class member has the static modifier. */
function isStatic(member: ts.ClassElement): boolean {
  const modifiers = ts.canHaveModifiers(member)
    ? (ts.getModifiers(member) ?? [])
    : [];
  return modifiers.some(
    (modifier) => modifier.kind === ts.SyntaxKind.StaticKeyword,
  );
}

/** Sorts ranges by their first line, then their last. */
function sortRanges(ranges: LineRange[]): LineRange[] {
  return ranges.sort((a, b) => a.start - b.start || a.end - b.end);
}

if (import.meta.main) {
  const args = parseArguments(process.argv.slice(2));
  if (args === undefined) {
    console.error(
      "function-ranges: usage: function-ranges.ts --root <dir> -- <path>...",
    );
    process.exit(1);
  }
  try {
    process.stdout.write(
      `${JSON.stringify(collectFunctionRanges(args.root, args.files))}\n`,
    );
  } catch (error) {
    console.error(
      `function-ranges: ${error instanceof Error ? error.message : String(error)}`,
    );
    process.exit(1);
  }
}
