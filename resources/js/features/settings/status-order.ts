/*
 * Where dragged rows land in an ordered list: before or after the row
 * they are dropped on, in the order they had among themselves.
 */
import type { Key } from "react-aria-components";

/*
 * moved returns the order after the dragged ids land before or after the
 * target; a drop on a row counts as before it.
 */
export function moved<T extends string>(
  order: T[],
  dragged: Set<Key>,
  target: Key,
  position: "before" | "after" | "on",
): T[] {
  const rest = order.filter((id) => !dragged.has(id));
  const at = rest.indexOf(target as T);
  if (at < 0) {
    return order;
  }
  const lands = order.filter((id) => dragged.has(id));
  const index = position === "after" ? at + 1 : at;

  return [...rest.slice(0, index), ...lands, ...rest.slice(index)];
}
