/*
 * How the tag picker reads what is typed: which tags it finds, and
 * whether it names a tag nobody has yet. Names match without regard to
 * case, as they do on the server.
 */
import type { Tag } from "@/lib/api/tags";

const limitShown = 8;

/*
 * matching narrows the tags to those containing the text, the first few
 * of them.
 */
export function matching(tags: Tag[], text: string): Tag[] {
  const wanted = text.trim().toLowerCase();
  const found = wanted ? tags.filter((tag) => tag.name.toLowerCase().includes(wanted)) : tags;

  return found.slice(0, limitShown);
}

/*
 * isNewName reports whether the text names a tag nobody has yet.
 */
export function isNewName(tags: Tag[], text: string): boolean {
  const wanted = text.trim().toLowerCase();

  return wanted !== "" && !tags.some((tag) => tag.name.toLowerCase() === wanted);
}
