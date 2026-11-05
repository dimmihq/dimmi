import { loadTurnstile } from "./turnstile";

describe("loadTurnstile", () => {
  it("adds the provider's script once, and resolves when it has loaded", async () => {
    const first = loadTurnstile();
    const second = loadTurnstile();
    const scripts = document.head.querySelectorAll("script[src^='https://challenges.cloudflare.com/']");
    expect(scripts).toHaveLength(1);

    const api = { render: vi.fn(), remove: vi.fn() };
    window.turnstile = api;
    scripts[0]?.dispatchEvent(new Event("load"));

    await expect(first).resolves.toBe(api);
    await expect(second).resolves.toBe(api);
    await expect(loadTurnstile()).resolves.toBe(api);
  });
});
