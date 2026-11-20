"use client";

/**
 * Slider — a single-value range control on React Aria's Slider: a track
 * that fills up to the value and a thumb to drag, keyboard and pointer
 * driven. Media players use it as their scrub bar. `tone` picks the
 * colours for the surface it sits on: the default for app surfaces,
 * `onDark` for controls laid over a video.
 */

import type { ReactNode, Ref } from "react";
import {
  Slider as AriaSlider,
  type SliderProps as AriaSliderProps,
  SliderThumb as AriaSliderThumb,
  SliderTrack as AriaSliderTrack,
} from "react-aria-components";

import { cx, sortCx } from "@/utils/cx";

export type SliderTone = "default" | "onDark";

const styles = sortCx({
  root: "group/slider flex w-full touch-none items-center",
  track: {
    default: "relative h-5 w-full cursor-pointer",
    tall: "relative h-8 w-full cursor-pointer",
  },
  rail: {
    default: "absolute inset-x-0 top-1/2 h-1 -translate-y-1/2 rounded-full bg-background-tertiary-default",
    onDark: "absolute inset-x-0 top-1/2 h-1 -translate-y-1/2 rounded-full bg-text-white/30",
  },
  fill: {
    default: "absolute top-1/2 left-0 h-1 -translate-y-1/2 rounded-full bg-button-primary",
    onDark: "absolute top-1/2 left-0 h-1 -translate-y-1/2 rounded-full bg-text-white",
  },
  thumb: {
    default: [
      "top-1/2 size-3 rounded-full bg-button-primary shadow-xs",
      "opacity-0 transition-opacity duration-150 group-hover/slider:opacity-100",
      "data-[dragging]:opacity-100 data-[focus-visible]:opacity-100",
      "outline-none data-[focus-visible]:ring-2 data-[focus-visible]:ring-border-focus-ring",
    ].join(" "),
    onDark: [
      "top-1/2 size-3 rounded-full bg-text-white shadow-xs",
      "opacity-0 transition-opacity duration-150 group-hover/slider:opacity-100",
      "data-[dragging]:opacity-100 data-[focus-visible]:opacity-100",
      "outline-none data-[focus-visible]:ring-2 data-[focus-visible]:ring-border-focus-ring",
    ].join(" "),
  },
});

export type SliderSize = "default" | "tall";

export interface SliderProps extends Omit<AriaSliderProps<number>, "children" | "className"> {
  tone?: SliderTone;
  /** tall gives a backdrop such as a waveform the height of a small button. */
  size?: SliderSize;
  className?: string;
  /** Drawn under the fill, such as a voice note's waveform; replaces the rail. */
  backdrop?: ReactNode;
  ref?: Ref<HTMLDivElement>;
}

export function Slider({ tone = "default", size = "default", className, backdrop, ref, ...props }: SliderProps) {
  return (
    <AriaSlider ref={ref} className={cx(styles.root, className)} {...props}>
      <AriaSliderTrack className={styles.track[size]}>
        {({ state }) => {
          const percent = state.getThumbPercent(0) * 100;

          return (
            <>
              {backdrop ?? <span aria-hidden className={styles.rail[tone]} />}
              {backdrop ? null : <span aria-hidden className={styles.fill[tone]} style={{ width: `${percent}%` }} />}
              <AriaSliderThumb className={styles.thumb[tone]} />
            </>
          );
        }}
      </AriaSliderTrack>
    </AriaSlider>
  );
}
