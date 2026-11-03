import { THEME_CHANGE_EVENT, THEME_STORAGE_KEY } from "@/components/application/theme/theme-toggle";

import { syncTheme } from "./theme-sync";

describe("syncTheme", () => {
  afterEach(() => {
    document.documentElement.className = "";
    localStorage.clear();
    vi.unstubAllGlobals();
  });

  it("seeds the toggle from the theme the page shows when the browser never chose", () => {
    document.documentElement.className = "dark";

    syncTheme();

    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark");
  });

  it("copies every change of the toggle to the cookie the server reads", () => {
    const set = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("cookieStore", { set });

    syncTheme();
    window.dispatchEvent(new CustomEvent(THEME_CHANGE_EVENT, { detail: "dark" }));

    expect(set).toHaveBeenCalledWith(expect.objectContaining({ name: "dimmi_theme", value: "dark", path: "/" }));
  });
});
