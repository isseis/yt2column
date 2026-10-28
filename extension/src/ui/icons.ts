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
  const svg = doc.createElementNS(svgNamespace, "svg");
  svg.setAttribute("viewBox", "0 0 20 20");
  svg.setAttribute("width", "14");
  svg.setAttribute("height", "14");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("focusable", "false");
  svg.classList.add("check-icon", `check-icon-${state}`);
  symbols[state](doc, svg);
  return svg;
}
