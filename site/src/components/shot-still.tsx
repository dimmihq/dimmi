/*
 * A still crop of the real app on a panel: framed like a window, pinned
 * to the top so the part that matters shows and the rest runs off the
 * panel's bottom edge.
 */
import { SHOTS } from "../shots";

export function ShotStill({ name, alt, maxWidth = 520 }: { name: keyof typeof SHOTS; alt: string; maxWidth?: number }) {
  const shot = SHOTS[name];
  return (
    <div
      className="self-start overflow-hidden rounded-t-xl border border-b-0 border-border-button-default bg-background-full p-1 pb-0 shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]"
      style={{ width: Math.min(shot.width, maxWidth), maxWidth: "100%" }}
    >
      <div className="overflow-hidden rounded-t-lg border border-b-0 border-separator-border">
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
      </div>
    </div>
  );
}
