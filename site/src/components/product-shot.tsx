/*
 * A product shot: a capture of the real app, taken at twice the pixel
 * density in light and dark mode by site/shots/capture.ts. The page
 * swaps the two with the theme. Until a capture exists the frame shows
 * what will go there.
 */
import { cx } from "@/utils/cx";

import { SHOTS } from "../shots";

export function ProductShot({
  name,
  alt,
  bleed = false,
  className,
}: {
  name: keyof typeof SHOTS;
  alt: string;
  /* Sits on the bottom edge of its panel, open at the bottom like a window rising out of it. */
  bleed?: boolean;
  className?: string;
}) {
  const shot = SHOTS[name];
  return (
    <div
      className={cx(
        "overflow-hidden rounded-2xl border border-border-button-default bg-background-secondary-default p-1.5 shadow-[0_24px_60px_-24px_rgb(0_0_0/0.25)]",
        bleed && "rounded-b-none border-b-0 pb-0",
        className,
      )}
    >
      <div
        className={cx(
          "overflow-hidden rounded-xl border border-separator-border bg-background-full",
          bleed && "rounded-b-none border-b-0",
        )}
      >
        {shot.ready ? (
          <>
            <img
              src={`/shots/${name}-light.webp`}
              alt={alt}
              width={shot.width}
              height={shot.height}
              loading="lazy"
              decoding="async"
              className="block h-auto w-full dark:hidden"
            />
            <img
              src={`/shots/${name}-dark.webp`}
              alt={alt}
              width={shot.width}
              height={shot.height}
              loading="lazy"
              decoding="async"
              className="hidden h-auto w-full dark:block"
            />
          </>
        ) : (
          <div
            className="site-dots flex items-center justify-center text-body-medium text-text-tertiary"
            style={{ aspectRatio: `${shot.width} / ${shot.height}` }}
          >
            {alt}
          </div>
        )}
      </div>
    </div>
  );
}
