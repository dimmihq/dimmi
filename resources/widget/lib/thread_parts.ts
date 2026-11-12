/*
 * What the thread chunk carries: the components that draw a conversation
 * and the stylesheet of its bubbles and action cards, adopted once into
 * each messenger root that shows one.
 */
import actions from "../styles/actions.css?inline";
import bubbles from "../styles/bubbles.css?inline";

export { ActionBlock, AiActions, Delivery, Said, Thread, Typing } from "../components/thread";

const styled = new WeakSet<ShadowRoot>();

export function adopt(root: ShadowRoot) {
  if (styled.has(root)) {
    return;
  }
  const sheet = new CSSStyleSheet();
  sheet.replaceSync(bubbles + actions);
  root.adoptedStyleSheets = [...root.adoptedStyleSheets, sheet];
  styled.add(root);
}
