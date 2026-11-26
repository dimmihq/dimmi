import { Prose } from "./prose";

describe("prose", () => {
  it("colors code that comes with tokens and escapes every token", () => {
    const html = String(
      <Prose
        blocks={[
          {
            kind: "code",
            text: 'const a = "<b>"',
            tokens: [{ class: "keyword", text: "const" }, { text: " a = " }, { class: "string", text: '"<b>"' }],
          },
        ]}
      />,
    );

    expect(html).toContain('<span class="keyword">const</span>');
    expect(html).toContain("<span> a = </span>");
    expect(html).toContain('<span class="string">&quot;&lt;b&gt;&quot;</span>');
  });

  it("shows code without tokens as plain text", () => {
    const html = String(<Prose blocks={[{ kind: "code", text: "plain <code>" }]} />);

    expect(html).toContain("<pre><code><span>plain &lt;code&gt;</span></code></pre>");
  });
});

describe("article blocks", () => {
  it("opens an embed on its own site and a button as a link", () => {
    const html = String(
      <Prose
        blocks={[
          { kind: "embed", text: "Setup tour", url: "https://www.youtube.com/watch?v=dQw4w9WgXcQ" },
          { kind: "button", text: "Start", url: "https://example.com/start" },
        ]}
      />,
    );

    expect(html).toContain(
      'href="https://www.youtube.com/watch?v=dQw4w9WgXcQ" target="_blank" rel="noopener noreferrer"',
    );
    expect(html).toContain("<small>www.youtube.com</small>");
    expect(html).toContain('<a class="button primary" href="https://example.com/start"');
  });

  it("keeps a link that is not a web address inert", () => {
    const html = String(
      <Prose
        blocks={[
          { kind: "embed", text: "x", url: "javascript:alert(1)" },
          { kind: "button", text: "y", url: "data:text/html,hi" },
        ]}
      />,
    );

    expect(html).not.toContain("javascript:");
    expect(html).not.toContain("data:");
    expect(html).toContain("<p>x</p>");
  });

  it("draws a details block with its own blocks inside", () => {
    const html = String(
      <Prose blocks={[{ kind: "details", text: "Refunds?", blocks: [{ kind: "p", text: "Five <days>" }] }]} />,
    );

    expect(html).toContain('<details class="details"><summary>');
    expect(html).toContain("<span>Refunds?</span></summary><p>Five &lt;days&gt;</p></details>");
  });
});
