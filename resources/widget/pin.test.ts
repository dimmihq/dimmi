import { pinStrip } from "./lib/ticket_pin";

/*
 * thread builds a scrolling thread of the given size after a bar slot.
 */
function thread(height: number, client: number) {
  document.body.innerHTML = '<div class="view"><div class="thread"></div></div>';
  const el = document.querySelector<HTMLElement>(".thread") as HTMLElement;
  Object.defineProperty(el, "scrollHeight", { configurable: true, get: () => height });
  Object.defineProperty(el, "clientHeight", { configurable: true, get: () => client });

  return el;
}

function scrollTo(el: HTMLElement, top: number) {
  el.scrollTop = top;
  el.dispatchEvent(new Event("scroll"));
}

describe("the pinned request bar", () => {
  it("sits before the thread and goes away without a request", () => {
    const el = thread(1000, 400);
    pinStrip(el, '<div class="ticket-strip">T-1</div>');
    const bar = el.previousElementSibling as HTMLElement;
    expect(bar.classList.contains("ticket-pin")).toBe(true);
    expect(bar.hidden).toBe(false);

    pinStrip(el, "");
    expect(bar.hidden).toBe(true);
  });

  it("folds well into a long thread and unfolds only back at the top", () => {
    const el = thread(1000, 400);
    pinStrip(el, '<div class="ticket-strip">T-1</div>');
    const bar = el.previousElementSibling as HTMLElement;

    scrollTo(el, 30);
    expect(bar.hasAttribute("data-folded")).toBe(false);
    scrollTo(el, 80);
    expect(bar.hasAttribute("data-folded")).toBe(true);
    scrollTo(el, 30);
    expect(bar.hasAttribute("data-folded")).toBe(true);
    scrollTo(el, 0);
    expect(bar.hasAttribute("data-folded")).toBe(false);
  });

  it("never folds a thread too short to stay scrollable once folded", () => {
    const el = thread(520, 400);
    pinStrip(el, '<div class="ticket-strip">T-1</div>');
    const bar = el.previousElementSibling as HTMLElement;

    scrollTo(el, 100);
    expect(bar.hasAttribute("data-folded")).toBe(false);
  });
});
