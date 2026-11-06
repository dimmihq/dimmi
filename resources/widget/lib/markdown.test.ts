import { markdown } from "./markdown";

describe("markdown", () => {
  it("renders bold, italic and inline code inside a paragraph", () => {
    expect(markdown("**Forgot password?** Use *Reset* or `reset --now`")).toBe(
      "<p><strong>Forgot password?</strong> Use <em>Reset</em> or <code>reset --now</code></p>",
    );
  });

  it("keeps snake_case words and lone stars as written", () => {
    expect(markdown("Set max_ai_replies to 5 * 2")).toBe("<p>Set max_ai_replies to 5 * 2</p>");
    expect(markdown("an _emphasised_ word")).toBe("<p>an <em>emphasised</em> word</p>");
  });

  it("splits paragraphs on blank lines and keeps single line breaks", () => {
    expect(markdown("First line\nsecond line\n\nNew paragraph")).toBe(
      "<p>First line<br>second line</p><p>New paragraph</p>",
    );
  });

  it("renders bulleted and numbered lists", () => {
    expect(markdown("Steps:\n1. Open **Settings**\n2. Choose Billing\n\n- one\n- two")).toBe(
      "<p>Steps:</p><ol><li>Open <strong>Settings</strong></li><li>Choose Billing</li></ol><ul><li>one</li><li>two</li></ul>",
    );
  });

  it("renders a code block without touching what is inside", () => {
    expect(markdown("```js\nconst a = 1 < 2 && **b**;\n```")).toBe(
      "<pre><code>const a = 1 &#60; 2 &#38;&#38; **b**;</code></pre>",
    );
  });

  it("opens http and https links in a new tab and drops every other scheme", () => {
    expect(markdown("[Billing guide](https://help.acme.com/billing)")).toBe(
      '<p><a href="https://help.acme.com/billing" target="_blank" rel="noopener noreferrer">Billing guide</a></p>',
    );
    expect(markdown("[click](javascript:alert(1))")).not.toContain("href");
    expect(markdown("See https://acme.com/status.")).toBe(
      '<p>See <a href="https://acme.com/status" target="_blank" rel="noopener noreferrer">https://acme.com/status</a>.</p>',
    );
  });

  it("never renders raw HTML or images", () => {
    const out = markdown('<img src=x onerror="alert(1)"> ![logo](https://acme.com/logo.png)');
    expect(out).not.toContain("<img");
    expect(out).toContain("&#60;img");
    expect(out).toContain("logo");
  });

  it("reads headings and quotes as plain bold lines and a quote", () => {
    expect(markdown("## Reset your password\n> Note this")).toBe(
      "<p><strong>Reset your password</strong></p><blockquote>Note this</blockquote>",
    );
  });
});
