/*
 * A code sample with quiet colouring: keywords, strings, comments and
 * calls, on the theme's own tokens. A small tokenizer is enough for the
 * few JavaScript samples the site shows, and keeps a highlighter library
 * out of the page.
 */
const TOKEN =
  /(\/\/[^\n]*)|("(?:[^"\\]|\\.)*"|`(?:[^`\\]|\\.)*`)|\b(import|from|export|function|return|const|await|async)\b|([A-Za-z_$][\w$]*)(?=\()|([{}()[\];,.<>/=]+)/g;

const kinds = ["c", "s", "k", "f", "p"];

function tokens(code: string) {
  const out: { id: string; text: string; kind?: string }[] = [];
  let last = 0;
  for (const match of code.matchAll(TOKEN)) {
    const at = match.index ?? 0;
    if (at > last) {
      out.push({ id: `t${last}`, text: code.slice(last, at) });
    }
    const group = match.slice(1).findIndex((part) => part !== undefined);
    out.push({ id: `t${at}`, text: match[0], kind: kinds[group] });
    last = at + match[0].length;
  }
  if (last < code.length) {
    out.push({ id: `t${last}`, text: code.slice(last) });
  }

  return out;
}

export function Code({ code }: { code: string }) {
  const lines = code.split("\n").length;
  return (
    <div className="site-code flex overflow-x-auto py-5 font-mono text-[13px] leading-6">
      <div className="shrink-0 px-4 text-end text-text-tertiary/60 select-none" aria-hidden>
        {Array.from({ length: lines }, (_, i) => i + 1).map((n) => (
          <div key={n}>{n}</div>
        ))}
      </div>
      <pre className="pe-6 text-text-primary">
        <code>
          {tokens(code).map((token) =>
            token.kind ? (
              <span key={token.id} className={token.kind}>
                {token.text}
              </span>
            ) : (
              token.text
            ),
          )}
        </code>
      </pre>
    </div>
  );
}
