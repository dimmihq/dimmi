// Builds the stylesheet of the dimmi CLI's local connect page from the
// app's own BoardUI theme (resources/js/styles/globals.css), so the page
// uses the same tokens, type scale and component recipes as dimmi. The
// result is committed as cli/connect/page.css and embedded in
// the binary. Run it after changing page.html or the theme:
//
//   node scripts/build-connect-css.mjs
import { readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { compile } from "tailwindcss";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const page = path.join(root, "cli/connect/page.html");
const help = path.join(root, "cli/sources/intercom/setup.go");
const out = path.join(root, "cli/connect/page.css");
const stylesDir = path.join(root, "resources/js/styles");

// Every class the page's templates write.
const candidates = new Set();
for (const file of [page, help]) {
  for (const m of readFileSync(file, "utf8").matchAll(/class="([^"]+)"/g)) {
    for (const c of m[1].split(/\s+/)) {
      if (c && !c.includes("{")) {
        candidates.add(c);
      }
    }
  }
}

// The connect page has no build step of its own: it takes the app's
// stylesheet entry and only the utilities above.
// The app's own accent: app.css copies BoardUI's emerald steps.
const appCSS = readFileSync(path.join(root, "resources/js/app.css"), "utf8");
const accent = [...appCSS.matchAll(/--color-accent-\d+:[^;]+;/g)].map((m) => m[0]).join("\n  ");

const input = `@import "./globals.css";
:root {
  ${accent}
}
@source not "../../..";

/* The Intercom help the source writes is plain HTML. */
.help ol { list-style: decimal; padding-inline-start: 1.25rem; display: flex; flex-direction: column; gap: 0.375rem; margin: 0.5rem 0; }
.help b { color: var(--color-text-primary); font-weight: 500; }
.help code { font-family: var(--font-mono); font-size: 0.8125rem; background: var(--color-background-tertiary-default); border-radius: 6px; padding: 0 4px; }
.help .note { font-size: 0.8125rem; color: var(--color-text-tertiary); }
`;

const require = createRequire(import.meta.url);

// Resolves the stylesheets the entry imports: tailwindcss's own and the
// theme files beside globals.css.
async function loadStylesheet(id, base) {
  const file =
    id === "tailwindcss" || id.startsWith("tailwindcss/")
      ? require.resolve(id === "tailwindcss" ? "tailwindcss/index.css" : `${id}.css`.replace(/\.css\.css$/, ".css"))
      : path.resolve(base, id);

  return { path: file, base: path.dirname(file), content: readFileSync(file, "utf8") };
}

const compiler = await compile(input, { base: stylesDir, loadStylesheet });
let css = compiler.build([...candidates]);

// BoardUI flips its tokens under the .dark class. The page has no script
// to set it, so every top-level .dark rule also applies, on :root, when
// the system is in dark mode.
function darkRules(source) {
  const rules = [];
  const re = /(?:^|\n)(\.dark[^{\n]*)\{/g;
  for (let m = re.exec(source); m !== null; m = re.exec(source)) {
    let depth = 1;
    let i = m.index + m[0].length;

    while (i < source.length && depth > 0) {
      if (source[i] === "{") {
        depth++;
      }
      if (source[i] === "}") {
        depth--;
      }
      i++;
    }

    const selector = m[1].trim().replace(/\.dark\b/g, ":root");
    rules.push(`${selector} {${source.slice(m.index + m[0].length, i - 1)}}`);
    re.lastIndex = i;
  }

  return rules;
}

const dark = darkRules(css);
if (dark.length) {
  css += `\n@media (prefers-color-scheme: dark) {\n${dark.join("\n")}\n}\n`;
}

writeFileSync(out, css);
