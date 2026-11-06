/*
 * Where the button of the access required screen goes: a sign-in comes
 * back to the page on screen, so nothing is lost by leaving it.
 */
export function destination(actionUrl: string, here: string): string {
  if (!actionUrl.startsWith("/login")) {
    return actionUrl;
  }

  const separator = actionUrl.includes("?") ? "&" : "?";

  return `${actionUrl}${separator}${new URLSearchParams({ redirect: here }).toString()}`;
}
