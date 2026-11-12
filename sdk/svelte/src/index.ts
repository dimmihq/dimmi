/*
 * @dimmi/svelte puts the dimmi messenger in a Svelte or SvelteKit app.
 * Call initDimmi({ appId, host }) in your root layout's onMount (it does
 * nothing on the server), read $unreadCount anywhere, and add
 * use:dimmiAction to any element that should open the messenger.
 */

import { type DimmiClient, type DimmiOptions, getDimmi, isBrowser, loadDimmi } from "@dimmi/browser";
import { type Readable, readable } from "svelte/store";

export type { DimmiClient, DimmiOptions, IdentitySource, MessengerSettings } from "@dimmi/browser";

/*
 * initDimmi loads the messenger in the browser and answers its client.
 */
export function initDimmi(options: DimmiOptions): DimmiClient {
  return isBrowser() ? loadDimmi(options) : getDimmi();
}

/*
 * useDimmi answers the messenger's client.
 */
export function useDimmi(): DimmiClient {
  return getDimmi();
}

/*
 * unreadCount is a store of the visitor's unread count.
 */
export const unreadCount: Readable<number> = readable(0, (set) => getDimmi().onUnreadCountChange(set));

export type DimmiAction = "open" | "toggle" | "close" | { message: string };

/*
 * dimmiAction opens, toggles or closes the messenger, or starts a new
 * message, when the element is clicked: <button use:dimmiAction={"open"}>.
 */
export function dimmiAction(node: HTMLElement, action: DimmiAction = "open") {
  let current = action;
  const click = () => {
    const dimmi = getDimmi();
    if (typeof current === "object") {
      dimmi.showNewMessage(current.message);
    } else if (current === "toggle") {
      dimmi.toggle();
    } else if (current === "close") {
      dimmi.hide();
    } else {
      dimmi.show();
    }
  };
  node.addEventListener("click", click);
  return {
    update(next: DimmiAction) {
      current = next;
    },
    destroy() {
      node.removeEventListener("click", click);
    },
  };
}
