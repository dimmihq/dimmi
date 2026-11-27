/*
 * Error reports from the browser app. The server's page names the tracker
 * (a Sentry-compatible DSN) with its environment and release; a page that
 * names none reports nothing, as in development and tests. Reports carry
 * no personal data: no addresses, cookies or request bodies. Session
 * recordings go to their own recorder (./sessions), never with a report.
 * A failed API call is the server's to report, so only the app's own
 * crashes are sent. Page loads and route changes are timed too, for web
 * vitals: every one outside production, a fifth in it.
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
    tracesSampleRate: tracesRate(reporting.environment),
    // The API doesn't continue browser traces, so its calls carry no trace headers.
    tracePropagationTargets: [],
    beforeSend: (event, hint) => (reportable(hint.originalException) ? event : null),
  });
}

/** The share of page loads and route changes that are timed. */
export function tracesRate(environment: string): number {
  return environment === "production" ? 0.2 : 1;
}

/** Times page loads and route changes by route once reporting started. */
export function startTracing(router: unknown): void {
  if (Sentry.getClient()) {
    Sentry.addIntegration(Sentry.tanstackRouterBrowserTracingIntegration(router));
  }
}

/** Reports an error a page caught and showed instead of crashing. */
export function reportError(error: unknown): void {
  if (reportable(error)) {
    Sentry.captureException(error);
  }
}
