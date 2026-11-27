/*
 * Monitoring for the marketing site and the docs, which share one project
 * in the error tracker. Both report their crashes and time page loads for
 * web vitals: every one on staging, a fifth in production. Production also
 * counts visits with the tracker's Web Traffic script, which sets no
 * cookies. Anywhere else, such as a local build, nothing is sent.
 */

const tracker = "https://errors.trydimmi.com";
const project = "3";
const key = "ee0d3c1646fa87790f5f180d1d4ba547";

const environments: Record<string, string> = {
  "trydimmi.com": "production",
  "staging.trydimmi.com": "staging",
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

  void import("@sentry/browser").then((Sentry) => {
    Sentry.init({
      dsn: `https://${key}@${new URL(tracker).host}/${project}`,
      environment,
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
