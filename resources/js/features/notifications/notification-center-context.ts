/*
 * Opens the notification center from anywhere in the app, such as the
 * address that emails link to, which used to be a page of its own.
 */
import { createContext, useContext } from "react";

export const OpenNotificationsContext = createContext<() => void>(() => undefined);

/*
 * useOpenNotifications returns what opens the notification center beside
 * the sidebar.
 */
export function useOpenNotifications(): () => void {
  return useContext(OpenNotificationsContext);
}
