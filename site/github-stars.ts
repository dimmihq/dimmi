/*
 * Build-time GitHub star count loader for the marketing site. Reads
 * stargazers_count from the GitHub API once per Vite run, using GITHUB_TOKEN
 * when CI provides one. Any failure or timeout returns null so builds always
 * stay green and fall back to a count-free "Star on GitHub" button.
 */
import process from "node:process";

export const GITHUB_REPO_API_URL = "https://api.github.com/repos/dimmihq/dimmi";
export const GITHUB_STARS_TIMEOUT_MS = 5_000;

export interface FetchGitHubStarsOptions {
  env?: Record<string, string | undefined>;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
  url?: string;
}

function parseCountOverride(raw: string): number | null {
  const parsed = Number(raw);
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : null;
}

function extractStargazersCount(payload: unknown): number | null {
  if (typeof payload !== "object" || payload === null || !("stargazers_count" in payload)) {
    return null;
  }
  const count = (payload as { stargazers_count: unknown }).stargazers_count;
  return typeof count === "number" && Number.isFinite(count) && count >= 0 ? Math.floor(count) : null;
}

function requestHeaders(token: string | undefined): Record<string, string> {
  const headers: Record<string, string> = {
    Accept: "application/vnd.github+json",
    "User-Agent": "dimmi-site-build",
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

export async function fetchGitHubStars(options: FetchGitHubStarsOptions = {}): Promise<number | null> {
  const env = options.env ?? process.env;

  if (env.GITHUB_STARS_COUNT !== undefined && env.GITHUB_STARS_COUNT.trim() !== "") {
    return parseCountOverride(env.GITHUB_STARS_COUNT);
  }

  if (env.GITHUB_STARS_OFFLINE === "1" || env.GITHUB_STARS_OFFLINE === "true") {
    return null;
  }

  const fetchImpl = options.fetchImpl ?? fetch;
  const timeoutMs = options.timeoutMs ?? GITHUB_STARS_TIMEOUT_MS;
  const url = options.url ?? GITHUB_REPO_API_URL;

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await fetchImpl(url, {
      headers: requestHeaders(env.GITHUB_TOKEN),
      signal: controller.signal,
    });
    if (!response.ok) {
      return null;
    }
    return extractStargazersCount(await response.json());
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}
