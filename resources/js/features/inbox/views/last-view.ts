/*
 * The view the person last looked at, kept in this browser so the
 * sidebar's Inbox link takes them back to it. Storage can be missing or
 * refuse, so every read and write gives up quietly and the inbox opens
 * on its first view instead.
 */

const key = "dimmi:inbox:last-view";

/*
 * rememberView keeps the view the person is looking at.
 */
export function rememberView(view: string): void {
  try {
    window.localStorage.setItem(key, view);
  } catch {
    return;
  }
}

/*
 * lastView returns the view the person last looked at, or the given one
 * when there is none.
 */
export function lastView(fallback: string): string {
  try {
    return window.localStorage.getItem(key) || fallback;
  } catch {
    return fallback;
  }
}
