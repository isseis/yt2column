import type { StepState } from "../core/response.ts";
import type { CheckState } from "../core/types.ts";

/** The SVG namespace; createElementNS builds the symbol without parsing a string. */
const svgNamespace = "http://www.w3.org/2000/svg";

/** Sets the outline a shape shares: a stroke in the colour the CSS gives the symbol. */
function outline(shape: SVGElement): void {
  shape.setAttribute("fill", "none");
  shape.setAttribute("stroke", "currentColor");
  shape.setAttribute("stroke-width", "2");
  shape.setAttribute("stroke-linecap", "round");
  shape.setAttribute("stroke-linejoin", "round");
}

/** Fills a shape with the colour the CSS gives the symbol. */
function fill(shape: SVGElement): void {
  shape.setAttribute("fill", "currentColor");
}

/** Adds one state's shapes to the symbol. */
type SymbolShapes = (doc: Document, svg: SVGSVGElement) => void;

// A record keyed by every CheckState, so a new state fails the type check here
// until it has its own shapes; a state never falls back to another's symbol.
const symbols: Record<CheckState, SymbolShapes> = {
  ok(doc, svg) {
    const circle = doc.createElementNS(svgNamespace, "circle");
    circle.setAttribute("cx", "10");
    circle.setAttribute("cy", "10");
    circle.setAttribute("r", "8");
    const check = doc.createElementNS(svgNamespace, "polyline");
    check.setAttribute("points", "6 10.5 9 13.5 14 7");
    outline(circle);
    outline(check);
    svg.append(circle, check);
  },
  warning(doc, svg) {
    const triangle = doc.createElementNS(svgNamespace, "polygon");
    triangle.setAttribute("points", "10 2.5 18.5 17 1.5 17");
    outline(triangle);
    const bar = doc.createElementNS(svgNamespace, "rect");
    bar.setAttribute("x", "9");
    bar.setAttribute("y", "7.5");
    bar.setAttribute("width", "2");
    bar.setAttribute("height", "4.5");
    bar.setAttribute("rx", "1");
    fill(bar);
    const dot = doc.createElementNS(svgNamespace, "circle");
    dot.setAttribute("cx", "10");
    dot.setAttribute("cy", "14.5");
    dot.setAttribute("r", "1");
    fill(dot);
    svg.append(triangle, bar, dot);
  },
};

/**
 * The check row's tick or warning symbol, as an SVG built with
 * createElementNS so no string is ever parsed as markup. The tick is a check
 * inside a circle and the warning an exclamation mark inside a triangle, so
 * the two states differ in shape and not only in colour. The symbol itself
 * carries no colour value and no character: the CSS rule its class selects
 * comes from the colour variables, and the row's state word carries the
 * meaning for assistive technology, so the symbol is aria-hidden.
 */
export function checkIcon(doc: Document, state: CheckState): SVGSVGElement {
  const svg = emptySymbol(doc, 14, ["check-icon", `check-icon-${state}`]);
  symbols[state](doc, svg);
  return svg;
}

/** An aria-hidden, unfocusable SVG of the given size on a 20 by 20 grid. */
function emptySymbol(
  doc: Document,
  size: number,
  classes: readonly string[],
): SVGSVGElement {
  const svg = doc.createElementNS(svgNamespace, "svg");
  svg.setAttribute("viewBox", "0 0 20 20");
  svg.setAttribute("width", String(size));
  svg.setAttribute("height", String(size));
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("focusable", "false");
  svg.classList.add(...classes);
  return svg;
}

/** A stroked line from (x1, y1) to (x2, y2). */
function line(
  doc: Document,
  x1: number,
  y1: number,
  x2: number,
  y2: number,
): SVGLineElement {
  const shape = doc.createElementNS(svgNamespace, "line");
  shape.setAttribute("x1", String(x1));
  shape.setAttribute("y1", String(y1));
  shape.setAttribute("x2", String(x2));
  shape.setAttribute("y2", String(y2));
  outline(shape);
  return shape;
}

/** A three-quarter arc; the CSS turns it while it shows a running wait. */
function arc(doc: Document): SVGPathElement {
  const shape = doc.createElementNS(svgNamespace, "path");
  shape.setAttribute("d", "M10 3 A7 7 0 1 1 3 10");
  outline(shape);
  return shape;
}

// A record keyed by every StepState, so a new state fails the type check here
// until it has its own shapes. Every state has a different shape: an open
// circle, a turning arc, a check, a dash and a cross.
const stepSymbols: Record<StepState, SymbolShapes> = {
  pending(doc, svg) {
    const circle = doc.createElementNS(svgNamespace, "circle");
    circle.setAttribute("cx", "10");
    circle.setAttribute("cy", "10");
    circle.setAttribute("r", "7");
    outline(circle);
    svg.append(circle);
  },
  running(doc, svg) {
    svg.append(arc(doc));
  },
  done(doc, svg) {
    const check = doc.createElementNS(svgNamespace, "polyline");
    check.setAttribute("points", "4 10.5 8 14.5 16 6");
    outline(check);
    svg.append(check);
  },
  skipped(doc, svg) {
    svg.append(line(doc, 4, 10, 16, 10));
  },
  failed(doc, svg) {
    svg.append(line(doc, 5, 5, 15, 15), line(doc, 15, 5, 5, 15));
  },
};

/**
 * A generation step's symbol, built like checkIcon: its shape tells the state
 * without colour, and the step's state word carries the meaning for assistive
 * technology, so the symbol is aria-hidden.
 */
export function stepIcon(doc: Document, state: StepState): SVGSVGElement {
  const svg = emptySymbol(doc, 14, ["step-icon", `step-icon-${state}`]);
  stepSymbols[state](doc, svg);
  return svg;
}

/**
 * The loading mark: a turning arc beside the sentence that says which wait it
 * is. The sentence carries the meaning, so the mark is aria-hidden.
 */
export function loadingIcon(doc: Document): SVGSVGElement {
  const svg = emptySymbol(doc, 20, ["loading-icon"]);
  svg.append(arc(doc));
  return svg;
}
