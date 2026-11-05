/*
 * Updates a part of the messenger to new markup without redrawing what
 * did not change. Elements carrying a data-key keep their node when the
 * new markup has the same key, so a message stays put while its state
 * moves from sending to sent, and an element whose key is new is marked
 * to enter, which plays the arrive animation. An unkeyed element is kept
 * when one of the same tag and class sits there, and updated in place.
 */

const entering = "enter";

function keyOf(node: Node): string | undefined {
  return node instanceof HTMLElement ? node.dataset.key : undefined;
}

function syncAttributes(from: Element, to: Element) {
  for (const { name } of [...to.attributes]) {
    if (!from.hasAttribute(name) && !(name === "class" && to.classList.contains(entering))) {
      to.removeAttribute(name);
    }
  }
  const keepEntering = to.classList.contains(entering);
  for (const { name, value } of [...from.attributes]) {
    const next = name === "class" && keepEntering ? `${value} ${entering}` : value;
    if (to.getAttribute(name) !== next) {
      to.setAttribute(name, next);
    }
  }
}

function markEntering(node: Node, known: Set<string>) {
  if (!(node instanceof HTMLElement)) {
    return;
  }
  const keyed = [node, ...node.querySelectorAll<HTMLElement>("[data-key]")].filter((el) => el.dataset.key);
  for (const el of keyed) {
    if (!known.has(el.dataset.key ?? "")) {
      el.classList.add(entering);
      el.addEventListener("animationend", () => el.classList.remove(entering), { once: true });
    }
  }
}

/*
 * take removes a reused node from the unkeyed candidates, so it is used
 * once.
 */
function take(candidates: Node[], node: Node) {
  const index = candidates.indexOf(node);
  if (index >= 0) {
    candidates.splice(index, 1);
  }
}

/*
 * sameKind finds an unkeyed element of the same tag and class to update in
 * place, such as the bubbles of a group that gained a message.
 */
function sameKind(candidates: Node[], node: Node): Node | undefined {
  if (!(node instanceof Element)) {
    return undefined;
  }

  return candidates.find(
    (child) => child instanceof Element && child.tagName === node.tagName && child.className === node.className,
  );
}

function patchChildren(parent: Element, fresh: Node, known: Set<string>, animate: boolean) {
  const old = new Map<string, Node>();
  for (const child of parent.childNodes) {
    const key = keyOf(child);
    if (key) {
      old.set(key, child);
    }
  }

  const unkeyed: Node[] = [...parent.childNodes].filter((child) => !keyOf(child));
  const result = [...fresh.childNodes].map((node) => {
    const key = keyOf(node);
    const kept = key ? old.get(key) : (unkeyed.find((child) => child.isEqualNode(node)) ?? sameKind(unkeyed, node));
    if (kept instanceof Element && node instanceof Element && kept.tagName === node.tagName) {
      if (!kept.isEqualNode(node)) {
        syncAttributes(node, kept);
        patchChildren(kept, node, known, animate);
      }
      take(unkeyed, kept);
      return kept;
    }
    if (kept && !key) {
      take(unkeyed, kept);
      return kept;
    }
    if (animate) {
      markEntering(node, known);
    }
    return node;
  });

  let cursor = parent.firstChild;
  for (const node of result) {
    if (node === cursor) {
      cursor = cursor.nextSibling;
      continue;
    }
    parent.insertBefore(node, cursor);
  }
  while (cursor) {
    const next = cursor.nextSibling;
    parent.removeChild(cursor);
    cursor = next;
  }
}

/*
 * patch makes target hold the markup. With animate off, as when a
 * different conversation opens, nothing is marked to enter.
 */
export function patch(target: Element, markup: string, animate: boolean) {
  const template = document.createElement("template");
  template.innerHTML = markup;
  const known = new Set([...target.querySelectorAll<HTMLElement>("[data-key]")].map((el) => el.dataset.key ?? ""));
  patchChildren(target, template.content, known, animate);
}
