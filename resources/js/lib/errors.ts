/*
 * Error reports from the browser app. The server's page names the tracker
 * (a Sentry-compatible DSN) with its environment and release; a page that
 * names none reports nothing, as in development and tests. Reports carry
 * no personal data: no addresses, cookies or request bodies, and no
 * session replays. A failed API call is the server's to report, so only
 * the app's own crashes are sent.
 */
import * as Sentry from "@sentry/react";

import { ApiError } from "./api/client";

export type Reporting = { dsn: string; environment: string; release: string };

/** The tracker the page names, or none. */
export function readReporting(doc: Document = document): Reporting | null {
  const meta = (name: string) => doc.querySelector<HTMLMetaElement>(`meta[name="dimmi-errors-${name}"]`)?.content ?? "";
  const dsn = meta("dsn");

  return dsn ? { dsn, environment: meta("environment"), release: meta("release") } : null;
}

/** Whether a report about the error is worth sending. */
export function reportable(error: unknown): boolean {
  return !(error instanceof ApiError);
}

/** Starts reporting when the page names a tracker. */
export function startReporting(doc: Document = document): void {
  const reporting = readReporting(doc);
  if (!reporting) {
    return;
  }

  Sentry.init({
    dsn: reporting.dsn,
    environment: reporting.environment || undefined,
    release: reporting.release || undefined,
    sendDefaultPii: false,
    beforeSend: (event, hint) => (reportable(hint.originalException) ? event : null),
  });
}

/** Reports an error a page caught and showed instead of crashing. */
export function reportError(error: unknown): void {
  if (reportable(error)) {
    Sentry.captureException(error);
  }
}
