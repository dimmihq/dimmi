import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchGitHubStars, GITHUB_REPO_API_URL } from "../../github-stars";
import { formatStarCount, STAR_COUNT_THRESHOLD, shouldShowStarCount, starButtonLabel } from "../github-stars";
import { GITHUB_URL } from "../links";
import { GitHubStars } from "./github-stars";

afterEach(() => {
  cleanup();
  delete window.webTraffic;
});

describe("star count helpers", () => {
  it("defaults the threshold to 100", () => {
    expect(STAR_COUNT_THRESHOLD).toBe(100);
  });

  it("hides counts below the threshold and null", () => {
    expect(shouldShowStarCount(null)).toBe(false);
    expect(shouldShowStarCount(0)).toBe(false);
    expect(shouldShowStarCount(3)).toBe(false);
    expect(shouldShowStarCount(99)).toBe(false);
    expect(shouldShowStarCount(Number.NaN)).toBe(false);
    expect(shouldShowStarCount(100)).toBe(true);
    expect(shouldShowStarCount(1_234)).toBe(true);
    expect(shouldShowStarCount(50, 50)).toBe(true);
  });

  it("formats star counts cleanly", () => {
    expect(formatStarCount(0)).toBe("0");
    expect(formatStarCount(100)).toBe("100");
    expect(formatStarCount(250)).toBe("250");
    expect(formatStarCount(999)).toBe("999");
    expect(formatStarCount(1_000)).toBe("1k");
    expect(formatStarCount(1_049)).toBe("1k");
    expect(formatStarCount(1_050)).toBe("1.1k");
    expect(formatStarCount(1_234)).toBe("1.2k");
    expect(formatStarCount(9_950)).toBe("10k");
    expect(formatStarCount(12_345)).toBe("12.3k");
    expect(formatStarCount(125_400)).toBe("125k");
    expect(formatStarCount(999_500)).toBe("1m");
    expect(formatStarCount(1_234_567)).toBe("1.2m");
  });

  it("builds accessible labels with or without the count", () => {
    expect(starButtonLabel(null)).toBe("Star dimmi on GitHub");
    expect(starButtonLabel(42)).toBe("Star dimmi on GitHub");
    expect(starButtonLabel(1_234)).toBe("Star dimmi on GitHub, 1,234 stars");
  });
});

describe("GitHubStars", () => {
  it("hides the number below threshold and when count is null", () => {
    const { rerender } = render(<GitHubStars location="hero" count={42} />);
    let link = screen.getByRole("link", { name: "Star dimmi on GitHub" });
    expect(link.getAttribute("href")).toBe(GITHUB_URL);
    expect(link.textContent).toBe("Star on GitHub");

    rerender(<GitHubStars location="hero" count={null} />);
    link = screen.getByRole("link", { name: "Star dimmi on GitHub" });
    expect(link.textContent).toBe("Star on GitHub");
  });

  it("shows Star and the formatted count at or above threshold across variants", () => {
    for (const variant of ["button", "nav", "mobile"] as const) {
      cleanup();
      render(<GitHubStars location={variant} variant={variant} count={1_234} />);
      const link = screen.getByRole("link", { name: "Star dimmi on GitHub, 1,234 stars" });
      expect(link.getAttribute("href")).toBe(GITHUB_URL);
      expect(link.textContent).toContain("Star");
      expect(link.textContent).toContain("1.2k");
      expect(link.textContent).not.toContain("Star on GitHub");
    }
  });

  it("tracks clicks with location when webTraffic is present and stays safe without it", () => {
    const onClick = vi.fn((event: { preventDefault: () => void }) => event.preventDefault());
    render(<GitHubStars location="pricing" count={250} onClick={onClick} />);
    const link = screen.getByRole("link", { name: "Star dimmi on GitHub, 250 stars" });

    fireEvent.click(link);
    expect(onClick).toHaveBeenCalledTimes(1);

    const track = vi.fn();
    window.webTraffic = { track };
    fireEvent.click(link);
    expect(track).toHaveBeenCalledWith("github_click", { location: "pricing" });
    expect(onClick).toHaveBeenCalledTimes(2);
  });
});

describe("fetchGitHubStars", () => {
  it("fetches stargazers_count and forwards GITHUB_TOKEN when set", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ stargazers_count: 1_234 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const count = await fetchGitHubStars({
      env: { GITHUB_TOKEN: "ghs_test_token" },
      fetchImpl: fetchMock,
    });

    expect(count).toBe(1_234);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(GITHUB_REPO_API_URL);
    expect(init?.headers).toEqual({
      Accept: "application/vnd.github+json",
      "User-Agent": "dimmi-site-build",
      Authorization: "Bearer ghs_test_token",
    });
  });

  it("returns null on HTTP errors or network failures", async () => {
    const rateLimited = vi.fn<typeof fetch>().mockResolvedValue(new Response("rate limit", { status: 403 }));
    expect(await fetchGitHubStars({ env: {}, fetchImpl: rateLimited })).toBeNull();

    const offline = vi.fn<typeof fetch>().mockRejectedValue(new Error("ENOTFOUND"));
    expect(await fetchGitHubStars({ env: {}, fetchImpl: offline })).toBeNull();
  });

  it("respects GITHUB_STARS_OFFLINE and GITHUB_STARS_COUNT without calling fetch", async () => {
    const fetchMock = vi.fn<typeof fetch>();

    expect(await fetchGitHubStars({ env: { GITHUB_STARS_OFFLINE: "1" }, fetchImpl: fetchMock })).toBeNull();
    expect(await fetchGitHubStars({ env: { GITHUB_STARS_COUNT: "1234" }, fetchImpl: fetchMock })).toBe(1_234);
    expect(await fetchGitHubStars({ env: { GITHUB_STARS_COUNT: "invalid" }, fetchImpl: fetchMock })).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("aborts and returns null when the API times out", async () => {
    const slowFetch = vi.fn<typeof fetch>(
      (_url, init) =>
        new Promise((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")));
        }),
    );

    const count = await fetchGitHubStars({ env: {}, fetchImpl: slowFetch, timeoutMs: 25 });
    expect(count).toBeNull();
  });
});
