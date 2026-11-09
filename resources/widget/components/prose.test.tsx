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
