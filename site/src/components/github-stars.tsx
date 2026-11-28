/*
 * Star on GitHub button with an optional build-time star count. The count
 * stays hidden until the repository reaches STAR_COUNT_THRESHOLD, so a tiny
 * number never undermines social proof and offline builds degrade cleanly.
 */
import { RiGithubFill } from "@remixicon/react";
import type { MouseEvent } from "react";

import { ButtonLink } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

import {
  BUILD_GITHUB_STARS,
  formatStarCount,
  STAR_COUNT_THRESHOLD,
  shouldShowStarCount,
  starButtonLabel,
} from "../github-stars";
import { GITHUB_URL } from "../links";
import { track } from "../track";

export interface GitHubStarsProps {
  /** Spot on the site where this button sits, sent with github_click events. */
  location: string;
  variant?: "button" | "nav" | "mobile";
  size?: "medium" | "small";
  count?: number | null;
  threshold?: number;
  className?: string;
  onClick?: (event: MouseEvent<HTMLAnchorElement>) => void;
}

export function GitHubStars({
  location,
  variant = "button",
  size = "medium",
  count = BUILD_GITHUB_STARS,
  threshold = STAR_COUNT_THRESHOLD,
  className,
  onClick,
}: GitHubStarsProps) {
  const showCount = shouldShowStarCount(count, threshold);
  const formattedCount = showCount ? formatStarCount(count) : null;
  const ariaLabel = starButtonLabel(count, threshold);

  const handleClick = (event: MouseEvent<HTMLAnchorElement>) => {
    track("github_click", { location });
    onClick?.(event);
  };

  if (variant === "nav") {
    return (
      <a
        href={GITHUB_URL}
        aria-label={ariaLabel}
        onClick={handleClick}
        className={cx(
          "inline-flex h-8 items-center gap-1.5 rounded-lg px-2.5 text-body-medium whitespace-nowrap text-text-secondary outline-none transition-colors hover:bg-background-secondary-default hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          className,
        )}
      >
        <RiGithubFill className="size-[18px] shrink-0" aria-hidden />
        <span>{showCount ? "Star" : "Star on GitHub"}</span>
        {formattedCount ? (
          <span className="rounded-md border border-border-button-default bg-background-secondary-default px-1.5 py-0.5 font-mono text-[12px] leading-4 font-medium text-text-primary tabular-nums">
            {formattedCount}
          </span>
        ) : null}
      </a>
    );
  }

  if (variant === "mobile") {
    return (
      <a
        href={GITHUB_URL}
        aria-label={ariaLabel}
        onClick={handleClick}
        className={cx(
          "flex items-center gap-3 rounded-lg px-3 py-2 text-body-medium text-text-primary outline-none transition-colors hover:bg-background-secondary-default focus-visible:bg-background-secondary-default",
          className,
        )}
      >
        <RiGithubFill className="size-5 shrink-0" aria-hidden />
        <span>{showCount ? "Star" : "Star on GitHub"}</span>
        {formattedCount ? (
          <span className="ms-auto rounded-md border border-border-button-default bg-background-secondary-default px-1.5 py-0.5 font-mono text-[12px] leading-4 font-medium text-text-secondary tabular-nums">
            {formattedCount}
          </span>
        ) : null}
      </a>
    );
  }

  return (
    <ButtonLink
      href={GITHUB_URL}
      variant="secondary"
      size={size}
      leadingIcon={RiGithubFill}
      aria-label={ariaLabel}
      onClick={handleClick}
      className={className}
    >
      {formattedCount ? (
        <span className="inline-flex items-center gap-2">
          <span>Star</span>
          <span className="border-s border-separator-border ps-2 font-mono text-[12px] font-medium text-text-secondary tabular-nums">
            {formattedCount}
          </span>
        </span>
      ) : (
        "Star on GitHub"
      )}
    </ButtonLink>
  );
}
