/*
 * Every person keeps one of BoardUI's avatar tints wherever they show, so
 * the same customer or teammate is recognisable at a glance across the
 * list, the thread and the details.
 */
import type { AvatarColor } from "@/components/base/avatar/avatar";

const tints: AvatarColor[] = ["blue", "lime", "pink", "neutral"];

/*
 * avatarColor picks the tint for a name. The same name always gets the
 * same tint.
 */
export function avatarColor(name: string): AvatarColor {
  let sum = 0;
  for (const letter of name) {
    sum = (sum * 31 + (letter.codePointAt(0) ?? 0)) % 997;
  }

  return tints[sum % tints.length] ?? "neutral";
}
