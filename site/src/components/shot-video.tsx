/*
 * A high-res product video captured from the real app in light and dark
 * mode. Renders a crisp looping MP4 over its 4K WebP poster, matching
 * ProductShot (bleed hero) or ShotStill (row panel). Users who prefer
 * reduced motion see the still WebP poster instead.
 */
import { useEffect, useRef } from "react";

import { cx } from "@/utils/cx";
import { sectionWidth, shotSrcSet, upTo } from "../shot-src";
import { SHOTS } from "../shots";

function syncPlayback(container: HTMLElement, visible: boolean) {
  for (const video of container.querySelectorAll("video")) {
    if (!visible) {
      video.pause();
      continue;
    }
    if (video.offsetParent !== null) {
      void video.play().catch(() => undefined);
    }
  }
}

export function ShotVideo({
  name,
  poster,
  alt,
  bleed = false,
  maxWidth,
  className,
}: {
  /* File stem under site/public/shots/{name}-{light,dark}.mp4 */
  name: string;
  /* Matching WebP poster key in SHOTS */
  poster: keyof typeof SHOTS;
  alt: string;
  /* Sits on the bottom edge of a hero panel, open at the bottom. */
  bleed?: boolean;
  maxWidth?: number;
  className?: string;
}) {
  const shot = SHOTS[poster];
  const root = useRef<HTMLDivElement>(null);
  // A bleed hero spans its section inside a 16px frame; a row panel's still sits in a 12px one.
  const sizes = bleed ? sectionWidth(16) : upTo(Math.min(shot.width, maxWidth ?? shot.width) - 12);

  useEffect(() => {
    const el = root.current;
    if (!el || !shot.ready || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          syncPlayback(el, entry.isIntersecting);
        }
      },
      { threshold: 0.15 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [shot.ready]);

  const frame = bleed
    ? "w-full overflow-hidden rounded-2xl rounded-b-none border border-b-0 border-border-button-default bg-background-secondary-default p-1.5 pb-0 shadow-[0_24px_60px_-24px_rgb(0_0_0/0.25)]"
    : "self-start overflow-hidden rounded-t-xl border border-b-0 border-border-button-default bg-background-full p-1 pb-0 shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]";
  const inner = bleed
    ? "overflow-hidden rounded-xl rounded-b-none border border-b-0 border-separator-border bg-background-full"
    : "overflow-hidden rounded-t-lg border border-b-0 border-separator-border bg-background-full";

  return (
    <div
      ref={root}
      className={cx(frame, className)}
      style={maxWidth ? { width: Math.min(shot.width, maxWidth), maxWidth: "100%" } : undefined}
    >
      <div className={inner} style={{ aspectRatio: `${shot.width} / ${shot.height}` }}>
        {shot.ready ? (
          <>
            <video
              aria-label={alt}
              autoPlay
              loop
              muted
              playsInline
              preload="metadata"
              width={shot.width}
              height={shot.height}
              poster={`/shots/${poster}-light.webp`}
              className="block h-full w-full object-cover motion-reduce:hidden dark:hidden"
            >
              <source src={`/shots/${name}-light.mp4`} type="video/mp4" />
            </video>
            <video
              aria-label={alt}
              autoPlay
              loop
              muted
              playsInline
              preload="metadata"
              width={shot.width}
              height={shot.height}
              poster={`/shots/${poster}-dark.webp`}
              className="hidden h-full w-full object-cover motion-reduce:hidden dark:block"
            >
              <source src={`/shots/${name}-dark.mp4`} type="video/mp4" />
            </video>
            <img
              src={`/shots/${poster}-light.webp`}
              srcSet={shotSrcSet(`${poster}-light`)}
              sizes={sizes}
              alt={alt}
              width={shot.width}
              height={shot.height}
              loading="lazy"
              decoding="async"
              className="hidden h-full w-full object-cover motion-reduce:block dark:motion-reduce:hidden"
            />
            <img
              src={`/shots/${poster}-dark.webp`}
              srcSet={shotSrcSet(`${poster}-dark`)}
              sizes={sizes}
              alt={alt}
              width={shot.width}
              height={shot.height}
              loading="lazy"
              decoding="async"
              className="hidden h-full w-full object-cover dark:motion-reduce:block"
            />
          </>
        ) : (
          <div className="site-dots flex h-full w-full flex-col items-center justify-center gap-2 px-8 py-12 text-center">
            <span className="rounded-md border border-dashed border-border-button-default bg-background-full/90 px-2.5 py-1 font-mono text-[11px] tracking-[0.06em] text-text-tertiary uppercase">
              Video · {name}
            </span>
            <p className="max-w-md text-[14px] leading-6 text-pretty text-text-secondary">{alt}</p>
          </div>
        )}
      </div>
    </div>
  );
}
