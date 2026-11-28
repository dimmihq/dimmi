/*
 * A product shot: a capture of the real app, taken at twice the pixel
 * density in light and dark mode by site/shots/capture.ts. The page
 * swaps the two with the theme. Until a capture exists the frame shows
 * what will go there.
 */
import { cx } from "@/utils/cx";
import { sectionWidth, shotSrcSet } from "../shot-src";
import { SHOTS } from "../shots";

export function ProductShot({
  name,
  alt,
  bleed = false,
  sizes = sectionWidth(16),
  className,
}: {
  name: keyof typeof SHOTS;
  alt: string;
  /* Sits on the bottom edge of its panel, open at the bottom like a window rising out of it. */
  bleed?: boolean;
  /* The img sizes value; the default fits a shot spanning a section, inside its 16px frame. */
  sizes?: string;
  className?: string;
}) {
  const shot = SHOTS[name];
  return (
    <div
      className={cx(
        "w-full overflow-hidden rounded-2xl border border-border-button-default bg-background-secondary-default p-1.5 shadow-[0_24px_60px_-24px_rgb(0_0_0/0.25)]",
        bleed && "rounded-b-none border-b-0 pb-0",
        className,
      )}
    >
      <div
        className={cx(
          "overflow-hidden rounded-xl border border-separator-border bg-background-full",
          bleed && "rounded-b-none border-b-0",
        )}
        style={{ aspectRatio: `${shot.width} / ${shot.height}` }}
      >
        {shot.ready ? (
          <>
            <img
              src={`/shots/${name}-light.webp`}
              srcSet={shotSrcSet(`${name}-light`)}
              sizes={sizes}
              alt={alt}
              width={shot.width}
              height={shot.height}
              loading="lazy"
              decoding="async"
              className="block h-full w-full object-cover dark:hidden"
            />
            <img
              src={`/shots/${name}-dark.webp`}
              srcSet={shotSrcSet(`${name}-dark`)}
              sizes={sizes}
              alt={alt}
              width={shot.width}
              height={shot.height}
              loading="lazy"
              decoding="async"
              className="hidden h-full w-full object-cover dark:block"
            />
          </>
        ) : (
          <div className="site-dots flex h-full w-full flex-col items-center justify-center gap-2 px-8 py-12 text-center">
            <span className="rounded-md border border-dashed border-border-button-default bg-background-full/90 px-2.5 py-1 font-mono text-[11px] tracking-[0.06em] text-text-tertiary uppercase">
              Screenshot · {name}
            </span>
            <p className="max-w-md text-[14px] leading-6 text-pretty text-text-secondary">{alt}</p>
          </div>
        )}
      </div>
    </div>
  );
}
