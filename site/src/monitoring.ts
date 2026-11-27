/*
 * Monitoring for the marketing site and the docs, which share one project
 * in the error tracker. Both report their crashes and time page loads for
 * web vitals: every one on staging, a fifth in production. Production also
 * counts visits with the tracker's Web Traffic script, which sets no
 * cookies. Anywhere else, such as a local build, nothing is sent. Reports
 * name the release the build was given (site/vite.config.ts), the commit's
 * sha, as the app's do.
 *
 * Both also load the session recorder (kimmi), with the public key of the
 * project the dashboard on the same host records to, so a visit that ends
 * in a sign-up stays one session. The recorder names no one here: the
 * dashboard does that once someone signs in. Its project's rules mask
 * every character of text and every input in the browser before anything
 * is sent, and decide whether visits are counted as well.
 */

const tracker = "https://errors.trydimmi.com";
const project = "3";
const key = "ee0d3c1646fa87790f5f180d1d4ba547";

const environments: Record<string, string> = {
  "trydimmi.com": "production",
  "staging.trydimmi.com": "staging",
};

const recorder = "https://sessions.trydimmi.com";

/** The recorder's public key for each environment's project. */
const recorderKeys: Record<string, string> = {
  production: "km_pub_58f104442330e6b9",
  staging: "km_pub_78e91f486fbaa2a4",
};

/** The environment a page served from the host belongs to, or none. */
export function environmentOf(host: string): string | undefined {
  return environments[host];
}

/** Starts monitoring when the page is served from a known host. */
export function startMonitoring(): void {
  const environment = environmentOf(window.location.hostname);
  if (!environment) {
    return;
  }

  if (environment === "production") {
    countVisits();
  }
  recordSessions(environment);

  void import("@sentry/browser").then((Sentry) => {
    Sentry.init({
      dsn: `https://${key}@${new URL(tracker).host}/${project}`,
      environment,
      release: import.meta.env.DIMMI_RELEASE || undefined,
      sendDefaultPii: false,
      integrations: [Sentry.browserTracingIntegration()],
      tracesSampleRate: environment === "production" ? 0.2 : 1,
      tracePropagationTargets: [],
    });
  });
}

/** Adds the Web Traffic script, which counts this page and the next ones. */
function countVisits(): void {
  const script = document.createElement("script");
  script.defer = true;
  script.src = `${tracker}/public/web-traffic/script.js`;
  script.dataset.project = project;
  script.dataset.key = key;
  document.head.append(script);
}

/** Adds the session recorder, once, for the environment's project. */
function recordSessions(environment: string): void {
  const key = recorderKeys[environment];
  if (!key || document.querySelector("script[data-dimmi-sessions]")) {
    return;
  }

  const script = document.createElement("script");
  script.async = true;
  script.src = `${recorder}/kimmi.js`;
  script.dataset.key = key;
  script.dataset.host = recorder;
  script.dataset.dimmiSessions = "";
  document.head.append(script);
}
