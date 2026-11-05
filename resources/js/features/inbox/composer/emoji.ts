/*
 * The emoji a teammate can add, with the words that find them. The list
 * is large, so it loads in its own chunk from this server the first time
 * someone looks for an emoji; skin tone and component variants are left
 * out, and an emoji goes into the message as plain text.
 */

export interface Emoji {
  unicode: string;
  label: string;
  tags: string[];
}

interface Entry {
  unicode: string;
  label: string;
  group?: number;
  tags?: string[];
}

const componentGroup = 2;

let loading: Promise<Emoji[]> | null = null;

export function loadEmoji(): Promise<Emoji[]> {
  loading ??= import("emojibase-data/en/compact.json").then((data) =>
    (data.default as Entry[])
      .filter((entry) => entry.group !== undefined && entry.group !== componentGroup)
      .map((entry) => ({ unicode: entry.unicode, label: entry.label, tags: entry.tags ?? [] })),
  );

  return loading;
}

/*
 * findEmoji returns the emoji whose name or tags start with the words
 * typed, those whose name starts with them first.
 */
export function findEmoji(list: Emoji[], query: string, limit: number): Emoji[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") {
    return list.slice(0, limit);
  }
  const named: Emoji[] = [];
  const tagged: Emoji[] = [];
  for (const emoji of list) {
    const label = emoji.label.toLowerCase();
    if (label.startsWith(needle)) {
      named.push(emoji);
    } else if (label.includes(` ${needle}`) || emoji.tags.some((tag) => tag.startsWith(needle))) {
      tagged.push(emoji);
    }
    if (named.length >= limit) {
      break;
    }
  }

  return [...named, ...tagged].slice(0, limit);
}
