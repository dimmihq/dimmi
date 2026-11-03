import { raw } from "./jsx-runtime";

describe("jsx runtime", () => {
  it("writes elements with escaped text and attributes", () => {
    const title = '<b>"hi"</b>';

    expect(String(<a href={`/x?q=${title}`}>{title}</a>)).toBe(
      '<a href="/x?q=&lt;b&gt;&quot;hi&quot;&lt;/b&gt;">&lt;b&gt;&quot;hi&quot;&lt;/b&gt;</a>',
    );
  });

  it("spells attributes the HTML way and drops the ones switched off", () => {
    expect(
      String(
        <label className="field" htmlFor="q" hidden data-open={undefined} aria-busy={false}>
          <svg viewBox="0 0 24 24" strokeWidth="2" strokeLinecap="round" aria-hidden="true" />
        </label>,
      ),
    ).toBe(
      '<label class="field" for="q" hidden><svg viewBox="0 0 24 24" stroke-width="2" stroke-linecap="round" aria-hidden="true"></svg></label>',
    );
  });

  it("closes void elements without children and joins lists and fragments", () => {
    const items = ["a", "b"];

    expect(
      String(
        <>
          <input name="q" />
          {items.map((item) => (
            <i key={item}>{item}</i>
          ))}
          {null}
          {0}
        </>,
      ),
    ).toBe('<input name="q"><i>a</i><i>b</i>0');
  });

  it("passes the widget's own markup through only when marked raw", () => {
    expect(String(<span>{raw("<em>ok</em>")}</span>)).toBe("<span><em>ok</em></span>");
  });
});
