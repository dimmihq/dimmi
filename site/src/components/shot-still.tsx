/*
 * A still crop of the real app on a panel: framed like a window, pinned
 * to the top so the part that matters shows and the rest runs off the
 * panel's bottom edge.
 */
import { cx } from "@/utils/cx";

import { SHOTS } from "../shots";

export function ShotStill({
  name,
  alt,
  maxWidth = 520,
  anchor = "top",
}: {
  name: keyof typeof SHOTS;
  alt: string;
  maxWidth?: number;
  /* Which edge stays in view when the shot is taller than its panel. */
  anchor?: "top" | "bottom";
}) {
  const shot = SHOTS[name];
  const bottom = anchor === "bottom";
  return (
    <div
      className={cx(
        "overflow-hidden border border-border-button-default bg-background-full shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]",
        bottom ? "mb-5 self-end rounded-b-xl border-t-0 p-1 pt-0" : "self-start rounded-t-xl border-b-0 p-1 pb-0",
      )}
      style={{ width: Math.min(shot.width, maxWidth), maxWidth: "100%" }}
    >
      <div
        className={cx(
          "overflow-hidden border border-separator-border",
          bottom ? "rounded-b-lg border-t-0" : "rounded-t-lg border-b-0",
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
            className="site-dots flex flex-col items-center justify-center gap-2 px-6 py-12 text-center"
            style={{ aspectRatio: `${shot.width} / ${shot.height}` }}
          >
            <span className="rounded-md border border-dashed border-border-button-default bg-background-full/90 px-2.5 py-1 font-mono text-[11px] tracking-[0.06em] text-text-tertiary uppercase">
              Screenshot · {name}
            </span>
            <p className="max-w-xs text-[14px] leading-6 text-pretty text-text-secondary">{alt}</p>
          </div>
        )}
      </div>
    </div>
  );
}
