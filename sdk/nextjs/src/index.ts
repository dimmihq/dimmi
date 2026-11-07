"use client";
/*
 * <Dimmi /> puts the messenger on every page of your app. Pass identity,
 * a server action that returns identityToken(config), and the messenger
 * signs your user in, asks again when it needs a fresh token, and signs
 * them out when it answers null. Render it once, in your root layout.
 */
import { useEffect } from "react";

type Command = [string, unknown?];

interface Queue {
  (...command: Command): void;
  q?: Command[];
}

type DimmiWindow = Window & { Dimmi?: Queue };

export interface DimmiProps {
  /* The messenger's app id, wgt_..., from Settings > Messenger. */
  appId: string;
  /* Where dimmi runs, such as https://dimmi.acme.com. */
  host: string;
  /* A server action answering the signed-in user's token, or null. */
  identity?: () => Promise<string | null>;
}

export function Dimmi({ appId, host, identity }: DimmiProps): null {
  useEffect(() => {
    const dimmi = queue();
    if (identity) {
      dimmi("identify", () => identity());
    }
    if (document.querySelector("script[data-dimmi-loader]")) {
      return;
    }
    const script = document.createElement("script");
    script.src = `${host.replace(/\/$/, "")}/widget/v1/loader.js`;
    script.async = true;
    script.dataset.appId = appId;
    script.dataset.dimmiLoader = "";
    document.body.append(script);
  }, [appId, host, identity]);

  return null;
}

/*
 * refreshDimmiIdentity asks the messenger to check who is signed in, for
 * right after your app signed someone in or out without a page load.
 */
export function refreshDimmiIdentity(): void {
  queue()("identify");
}

/*
 * dimmiLogout ends the visitor's messenger session, for your sign-out
 * button.
 */
export function dimmiLogout(): void {
  queue()("logout");
}

function queue(): Queue {
  const win = window as DimmiWindow;
  if (!win.Dimmi) {
    const stub: Queue = (...command: Command) => {
      stub.q = stub.q ?? [];
      stub.q.push(command);
    };
    win.Dimmi = stub;
  }
  return win.Dimmi;
}
