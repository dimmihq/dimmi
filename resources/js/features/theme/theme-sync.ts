/*
 * Keeps BoardUI's theme toggle and the server in step. The toggle keeps
 * its choice in local storage; the server renders each page in the theme
 * named by a cookie, so pages start in the right theme without a flash.
 * Every change the toggle makes is copied to the cookie, and at start
 * the toggle's storage is seeded from the cookie, or from the theme the
 * page already shows, which follows the system for a browser that never
 * chose.
 */
import { THEME_CHANGE_EVENT, THEME_STORAGE_KEY, type ThemeMode } from "@/components/application/theme/theme-toggle";

export const themeCookie = "dimmi_theme";

const year = 365 * 24 * 60 * 60 * 1000;

function chosenTheme(): ThemeMode | null {
  const match = document.cookie.match(/(?:^|; )dimmi_theme=(light|dark)/);

  return (match?.[1] as ThemeMode | undefined) ?? null;
}

/*
 * remember writes the cookie through the Cookie Store, which browsers
 * only offer on HTTPS and localhost. An install served over plain HTTP
 * keeps the choice in local storage alone, and its pages start light
 * until the app applies it.
 */
function remember(theme: ThemeMode) {
  if (!("cookieStore" in window)) {
    return;
  }

  void window.cookieStore.set({
    name: themeCookie,
    value: theme,
    path: "/",
    expires: Date.now() + year,
    sameSite: "lax",
  });
}

/*
 * syncTheme seeds the toggle from the page and copies its changes to the
 * cookie. Call it once, before the app renders.
 */
export function syncTheme() {
  const shown: ThemeMode = document.documentElement.classList.contains("dark") ? "dark" : "light";

  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, chosenTheme() ?? shown);
  } catch {
    // Blocked storage leaves the toggle on its own default; the page keeps the theme it shows.
  }

  window.addEventListener(THEME_CHANGE_EVENT, (event) => {
    remember((event as CustomEvent<ThemeMode>).detail);
  });
}
