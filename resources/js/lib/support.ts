/*
 * The messenger the dashboard shows its own teammates. The server's page
 * names a messenger of this same server by its app id when the team that
 * runs the server supports the people who use it, with dimmi itself; a
 * page that names none shows no messenger, as in development, tests and
 * any server whose operator named none. It loads through @dimmi/react,
 * the package customers install, once a teammate is signed in. It has no
 * launcher of its own, which would sit over the corner of the pages: the
 * teammate opens it from the sidebar, below Settings. Signing out forgets the
 * visitor, so the next person at the browser starts a conversation of
 * their own.
 */
import { getDimmi, type MessengerSettings } from "@dimmi/browser";

/** The messenger the page names, and the server it lives on. */
export type SupportMessenger = { appId: string; host: string };

/** The messenger the page names, or none. */
export function readSupportMessenger(doc: Document = document, win: Window = window): SupportMessenger | null {
  const appId = doc.querySelector<HTMLMetaElement>('meta[name="dimmi-support-messenger"]')?.content ?? "";
  if (!appId.startsWith("wgt_")) {
    return null;
  }

  return { appId, host: win.location.origin };
}

/** The messenger's settings in the dashboard: no launcher. One object, so it is sent once. */
export const supportSettings: MessengerSettings = { appearance: { launcher: { style: "hidden" } } };

/** Opens the messenger, as from the sidebar. */
export function openSupport(): void {
  getDimmi().show();
}

/** Forgets the visitor in the messenger, as when the teammate signs out. */
export function forgetSupportVisitor(doc: Document = document, win: Window = window): void {
  if (readSupportMessenger(doc, win)) {
    getDimmi().shutdown();
  }
}
