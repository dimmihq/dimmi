/*
 * What the loader hands the core: the app the snippet names, the API base
 * every request goes to, and where the visitor's identity comes from. The
 * loader reads these off its own script tag, so a page that pastes the
 * snippet twice or behind a subpath still points at the right install.
 * A preview, such as the one in the dashboard, shows sample content and
 * never calls the server.
 */

export interface Settings {
  appId: string;
  apiBase: string;
  identityUrl?: string;
  latency?: number;
  preview?: boolean;
}

export type Command = [name: string, ...args: unknown[]];

export interface Queue {
  (...command: Command): void;
  q?: Command[];
}

export interface DimmiWindow {
  Dimmi?: Queue;
  __dimmi?: Settings;
}
