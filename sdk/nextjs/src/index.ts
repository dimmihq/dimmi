"use client";
/*
 * <Dimmi /> puts the messenger on every page of your app. Pass identity,
 * a server action that returns identityToken(config), and the messenger
 * signs your user in, asks again when it needs a fresh token, and signs
 * them out when it answers null. Render it once, in your root layout, or
 * wrap your app in <DimmiProvider> from here to use useDimmi() below it.
 * Everything @dimmi/react exports is exported here too.
 */
import { getDimmi } from "@dimmi/browser";
import { Dimmi as ReactDimmi } from "@dimmi/react";
import { createElement, type ReactNode } from "react";

export type { DimmiClient, MessengerSettings } from "@dimmi/browser";
export {
  DimmiButton,
  type DimmiButtonProps,
  DimmiProvider,
  type DimmiProviderProps,
  useDimmi,
  useTrackPage,
  useUnreadCount,
} from "@dimmi/react";

export interface DimmiProps {
  /* The messenger's app id, wgt_..., from Settings > Messenger. */
  appId: string;
  /* Where dimmi runs, such as https://dimmi.acme.com. */
  host: string;
  /* A server action answering the signed-in user's token, or null. */
  identity?: () => Promise<string | null>;
  /* A nonce for the loader script under a strict CSP. */
  nonce?: string;
}

export function Dimmi(props: DimmiProps): ReactNode {
  return createElement(ReactDimmi, props);
}

/*
 * refreshDimmiIdentity asks the messenger to check who is signed in, for
 * right after your app signed someone in or out without a page load.
 */
export function refreshDimmiIdentity(): void {
  getDimmi().identify();
}

/*
 * dimmiLogout ends the visitor's messenger session, for your sign-out
 * button.
 */
export function dimmiLogout(): void {
  getDimmi().shutdown();
}
