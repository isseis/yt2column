// Redraws a container only when what it shows changed: the job state and the
// viewer's article body both use it, so a poll that brings the same value
// again keeps the focus and the selection inside the container.

/** The key last drawn into each container, and the nodes that drawing left. */
const drawn = new WeakMap<
  HTMLElement,
  { readonly key: string; readonly nodes: readonly Node[] }
>();

/**
 * Calls draw, which replaces container's children, unless key equals the key
 * last drawn into container and its children are still the nodes drawn then.
 * Returns whether it drew.
 *
 * Contract: the key is kept in this module's memory only, never written to
 * the DOM, so a key made from untrusted text never becomes an attribute; and
 * a container whose children something else replaced is always drawn again,
 * even with the same key.
 */
export function drawOnKeyChange(
  container: HTMLElement,
  key: string,
  draw: () => void,
): boolean {
  const last = drawn.get(container);
  const current = Array.from(container.childNodes);
  if (
    last !== undefined &&
    last.key === key &&
    last.nodes.length === current.length &&
    last.nodes.every((node, i) => node === current[i])
  ) {
    return false;
  }
  draw();
  drawn.set(container, { key, nodes: Array.from(container.childNodes) });
  return true;
}
