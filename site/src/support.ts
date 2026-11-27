/*
 * dimmi's own messenger on the marketing site and the docs, so visitors
 * can ask the team, and the agent, a question from any page. The team
 * answers them in dimmi itself. Only production has a messenger: on
 * staging, a preview or a local build nothing loads. It comes through
 * @dimmi/browser, the package customers install, from the host that
 * serves the site.
 */
import { loadDimmi } from "@dimmi/browser";

type Messenger = { appId: string; host: string };

const messengers: Record<string, Messenger> = {
  "trydimmi.com": { appId: "wgt_01m89n8mc6e64trfb4yt4xfh1p", host: "https://trydimmi.com" },
};

/** The messenger a page served from the host shows, or none. */
export function messengerOf(host: string): Messenger | undefined {
  return messengers[host];
}

/** Loads the messenger when the page is served from a host that has one. */
export function startSupport(): void {
  const messenger = messengerOf(window.location.hostname);
  if (messenger) {
    loadDimmi(messenger);
  }
}
