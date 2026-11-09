import { Prose } from "./prose";

describe("prose", () => {
  it("colors code that comes with tokens and escapes every token", () => {
    const html = String(
      <Prose
        blocks={[
          {
            kind: "code",
            text: 'const a = "<b>"',
            tokens: [{ class: "kd", text: "const" }, { text: " a = " }, { class: "s2", text: '"<b>"' }],
          },
        ]}
      />,
    );

    expect(html).toContain('<span class="chroma-kd">const</span>');
    expect(html).toContain("<span> a = </span>");
    expect(html).toContain('<span class="chroma-s2">&quot;&lt;b&gt;&quot;</span>');
  });

  it("shows code without tokens as plain text", () => {
    const html = String(<Prose blocks={[{ kind: "code", text: "plain <code>" }]} />);

    expect(html).toContain("<pre><code>plain &lt;code&gt;</code></pre>");
  });
});
