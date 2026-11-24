/*
 * An A/B split as the builder edits it. Each variant is one of the
 * step's paths and shares its id; the variant keeps its label and its
 * share of conversations, out of 100, in the step's settings, and the
 * path carries the same label so the canvas reads it. Adding or removing
 * a variant shares conversations evenly again. A winner, once picked,
 * sends everyone down one variant from the next publish on.
 */
import { newId, type Path, type Step } from "./workflows";

export interface Variant {
  id: string;
  label: string;
  weight: number;
}

export const fewestVariants = 2;
export const mostVariants = 4;

/*
 * variantsOf reads a split's variants, empty when unset.
 */
export function variantsOf(step: Step): Variant[] {
  const value = step.settings?.variants;
  if (!Array.isArray(value)) {
    return [];
  }

  return value.flatMap((item): Variant[] =>
    item && typeof item === "object" && typeof item.id === "string"
      ? [{ id: item.id, label: String(item.label ?? ""), weight: Number(item.weight) || 0 }]
      : [],
  );
}

/*
 * winnerOf is the variant everyone takes, empty when none is picked.
 */
export function winnerOf(step: Step): string {
  const value = step.settings?.winner;

  return typeof value === "string" ? value : "";
}

/*
 * evenShares shares 100 between count variants, the remainder going to
 * the first ones.
 */
export function evenShares(count: number): number[] {
  const each = Math.floor(100 / count);

  return Array.from({ length: count }, (_, i) => each + (i < 100 - each * count ? 1 : 0));
}

/*
 * sharesTotal is what a split's shares add up to.
 */
export function sharesTotal(step: Step): number {
  return variantsOf(step).reduce((sum, v) => sum + v.weight, 0);
}

function letter(index: number): string {
  return String.fromCharCode(65 + index);
}

/*
 * freshSplit is a new split: variants A and B, half each.
 */
export function freshSplit(step: Step): Step {
  const variants = [0, 1].map((i) => ({ id: newId("p_"), label: letter(i), weight: 50 }));

  return {
    ...step,
    settings: { variants },
    paths: variants.map((v): Path => ({ id: v.id, label: v.label, steps: [] })),
  };
}

function withVariants(step: Step, variants: Variant[]): Step {
  return { ...step, settings: { ...step.settings, variants } };
}

/*
 * withVariantLabel renames a variant and its path.
 */
export function withVariantLabel(step: Step, id: string, label: string): Step {
  const renamed = withVariants(
    step,
    variantsOf(step).map((v) => (v.id === id ? { ...v, label } : v)),
  );

  return { ...renamed, paths: renamed.paths?.map((p) => (p.id === id ? { ...p, label } : p)) };
}

/*
 * withVariantWeight changes one variant's share.
 */
export function withVariantWeight(step: Step, id: string, weight: number): Step {
  const share = Math.max(0, Math.min(100, Math.trunc(weight)));

  return withVariants(
    step,
    variantsOf(step).map((v) => (v.id === id ? { ...v, weight: share } : v)),
  );
}

function rebalanced(variants: Variant[]): Variant[] {
  const shares = evenShares(variants.length);

  return variants.map((v, i) => ({ ...v, weight: shares[i] ?? 0 }));
}

/*
 * withNewVariant adds a variant and its path, and shares evenly again.
 */
export function withNewVariant(step: Step): Step {
  const current = variantsOf(step);
  const fresh = { id: newId("p_"), label: letter(current.length), weight: 0 };
  const next = withVariants(step, rebalanced([...current, fresh]));

  return { ...next, paths: [...(next.paths ?? []), { id: fresh.id, label: fresh.label, steps: [] }] };
}

/*
 * withoutVariant removes a variant, its path and every step down it,
 * shares evenly again, and drops the winner when it was this one.
 */
export function withoutVariant(step: Step, id: string): Step {
  let next = withVariants(step, rebalanced(variantsOf(step).filter((v) => v.id !== id)));
  if (winnerOf(step) === id) {
    next = withWinner(next, "");
  }

  return { ...next, paths: next.paths?.filter((p) => p.id !== id) };
}

/*
 * withWinner sends everyone down one variant, or back to the shares
 * with an empty id.
 */
export function withWinner(step: Step, id: string): Step {
  const { winner: _, ...rest } = step.settings ?? {};

  return { ...step, settings: id ? { ...rest, winner: id } : rest };
}

/*
 * splitSentence is what a split's card says.
 */
export function splitSentence(step: Step): string {
  const variants = variantsOf(step);
  const winner = variants.find((v) => v.id === winnerOf(step));
  if (winner) {
    return `Send everyone down ${winner.label || "the winner"}`;
  }
  if (variants.length === 0) {
    return "Not set up yet";
  }

  return `Split ${variants.map((v) => `${v.label || "?"} ${v.weight}%`).join(" · ")}`;
}
