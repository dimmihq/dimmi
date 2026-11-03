/*
 * The words the collision indicators use: who else is viewing a
 * conversation and who is typing in it.
 */
import type { Listener } from "@/lib/realtime/use-channel";

import type { Typist } from "./typing";

/*
 * viewingLabel says who else is viewing, in words.
 */
export function viewingLabel(others: Listener[]): string {
  const names = others.map((listener) => listener.name || "A teammate");
  if (names.length === 1) {
    return `${names[0]} is also viewing`;
  }
  if (names.length === 2) {
    return `${names[0]} and ${names[1]} are also viewing`;
  }
  return `${names[0]} and ${names.length - 1} others are also viewing`;
}

/*
 * typingLabel says who is typing, naming each typist with the function
 * given.
 */
export function typingLabel(typists: Typist[], nameOf: (typist: Typist) => string): string {
  const names = typists.map(nameOf);
  if (names.length === 1) {
    return `${names[0]} is typing…`;
  }
  return `${names.slice(0, -1).join(", ")} and ${names.at(-1)} are typing…`;
}
