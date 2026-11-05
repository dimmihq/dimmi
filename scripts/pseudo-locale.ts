/*
 * Writes the pseudo locale en-XA from the English messages, for the
 * browser app (resources/js/lang) and the server (lang). Every letter is
 * swapped for an accented one, every message grows by about a third and
 * sits in brackets, and placeholders, tags and nested keys stay as they
 * are. Reading the app in en-XA shows at a glance any text that skipped
 * translation, since it stays plain English, and any layout that breaks
 * on longer words or clips a message's end.
 *
 * The output is generated, never checked in, and only ever served where
 * app.locales lists en-XA: in development and in the end-to-end tests,
 * never in a release.
 *
 *   node scripts/pseudo-locale.ts
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

type Messages = { [key: string]: string | Messages };

const root = fileURLToPath(new URL("..", import.meta.url));
const source = "en";
const target = "en-XA";
const directories = ["resources/js/lang", "lang"];

const accents: Record<string, string> = {
  a: "å",
  b: "ƀ",
  c: "ç",
  d: "đ",
  e: "é",
  f: "ƒ",
  g: "ĝ",
  h: "ĥ",
  i: "î",
  j: "ĵ",
  k: "ķ",
  l: "ļ",
  m: "ɱ",
  n: "ñ",
  o: "ö",
  p: "þ",
  r: "ŕ",
  s: "š",
  t: "ţ",
  u: "û",
  w: "ŵ",
  y: "ý",
  z: "ž",
  A: "Å",
  C: "Ç",
  D: "Đ",
  E: "É",
  G: "Ĝ",
  I: "Î",
  N: "Ñ",
  O: "Ö",
  S: "Š",
  T: "Ŧ",
  U: "Û",
  Y: "Ý",
};

/*
 * kept matches what must reach the translation untouched: placeholders
 * such as {{name}} or {{.field}}, nested keys such as $t(key), and tags.
 */
const kept = /(\{\{[^}]*\}\}|\$t\([^)]*\)|<[^>]*>)/;

const filler = " ŀöŕéɱ îþšûɱ đöļöŕ šîţ åɱéţ çöñšéçţéţûŕ";

/*
 * pseudo writes one message in the pseudo locale.
 */
function pseudo(message: string): string {
  const parts = message.split(kept).map((part, index) => {
    if (index % 2 === 1) {
      return part;
    }

    return [...part].map((letter) => accents[letter] ?? letter).join("");
  });
  const padding = filler.slice(0, Math.ceil(message.length * 0.3));

  return `[${parts.join("")}${padding}]`;
}

function translate(messages: Messages): Messages {
  return Object.fromEntries(
    Object.entries(messages).map(([key, value]) => [key, typeof value === "string" ? pseudo(value) : translate(value)]),
  );
}

function write(directory: string) {
  const from = path.join(root, directory, source);
  const to = path.join(root, directory, target);
  fs.rmSync(to, { recursive: true, force: true });
  fs.mkdirSync(to, { recursive: true });

  for (const file of fs.readdirSync(from).filter((name) => name.endsWith(".json"))) {
    const messages = JSON.parse(fs.readFileSync(path.join(from, file), "utf8")) as Messages;
    fs.writeFileSync(path.join(to, file), `${JSON.stringify(translate(messages), null, 2)}\n`);
  }
}

for (const directory of directories) {
  write(directory);
}
