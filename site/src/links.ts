/* Where the site sends people: the app and the source. */
export const GITHUB_URL = "https://github.com/dimmihq/dimmi";
// The built site shares its host with the app (the edge sends the site's
// pages to it and everything else to the app), so its links to the app are
// relative and work on every environment. The dev server has no app behind
// it, so there they go to the production one.
export const APP_URL = import.meta.env.DEV ? "https://trydimmi.com" : "";
