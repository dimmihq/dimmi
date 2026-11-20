/*
 * A labelled slider with the value typed beside it. The slider moves on a
 * curve when `curve` is set, so small volumes get as much travel as large
 * ones; the number field always holds the exact value.
 */
import type { ReactNode } from "react";
import { Label, Slider, SliderThumb, SliderTrack } from "react-aria-components";

import { NumberField } from "@/components/base/number-field/number-field";
import { cx } from "@/utils/cx";

const STEPS = 1000;

export function Range({
  label,
  hint,
  value,
  onChange,
  max,
  step = 1,
  curve = false,
  freeUpTo,
  format,
}: {
  label: string;
  hint?: ReactNode;
  value: number;
  onChange: (value: number) => void;
  max: number;
  step?: number;
  curve?: boolean;
  /* Marks the start of the track as free, up to this value. */
  freeUpTo?: number;
  format?: Intl.NumberFormatOptions;
}) {
  const toSlider = (v: number) => (curve ? Math.round(Math.sqrt(v / max) * STEPS) : v);
  const fromSlider = (s: number) => {
    if (!curve) {
      return s;
    }
    const raw = (s / STEPS) ** 2 * max;
    const grain = raw < 1000 ? 10 : raw < 5000 ? 50 : 100;
    return Math.min(max, Math.round(raw / grain) * grain);
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-end justify-between gap-4">
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="text-[15px] leading-6 font-medium text-text-primary">{label}</span>
          {hint ? <span className="text-[14px] leading-5 text-text-secondary">{hint}</span> : null}
        </div>
        <NumberField
          aria-label={label}
          value={value}
          onChange={(v) => onChange(Number.isNaN(v) ? 0 : v)}
          minValue={0}
          maxValue={max * 10}
          step={step}
          formatOptions={format}
          size="small"
          hideSteppers
          className="w-28 shrink-0"
          fieldClassName="text-end tabular-nums"
        />
      </div>
      <Slider
        aria-label={label}
        value={toSlider(Math.min(value, max))}
        onChange={(s) => onChange(fromSlider(s as number))}
        minValue={0}
        maxValue={curve ? STEPS : max}
        step={curve ? 1 : step}
        className="w-full"
      >
        <Label className="sr-only">{label}</Label>
        <SliderTrack className="relative h-6 w-full">
          {({ state }) => (
            <>
              <div className="absolute top-1/2 h-1.5 w-full -translate-y-1/2 rounded-full bg-background-tertiary-default" />
              {freeUpTo ? (
                <div
                  className="absolute top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-accent-200 dark:bg-accent-800"
                  style={{ width: `${(toSlider(freeUpTo) / (curve ? STEPS : max)) * 100}%` }}
                />
              ) : null}
              <div
                className="absolute top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-accent-500"
                style={{ width: `${state.getThumbPercent(0) * 100}%` }}
              />
              <SliderThumb
                className={({ isFocusVisible, isDragging }) =>
                  cx(
                    "top-1/2 size-5 rounded-full border border-border-button-default bg-white shadow-[0_1px_3px_rgb(0_0_0/0.25)] outline-none",
                    isDragging && "scale-110",
                    isFocusVisible && "ring-2 ring-border-focus-ring",
                  )
                }
              />
            </>
          )}
        </SliderTrack>
      </Slider>
      {freeUpTo ? (
        <div className="relative -mt-1 h-4 text-[12px] leading-4 text-text-tertiary">
          <span
            className="absolute -translate-x-1/2 whitespace-nowrap"
            style={{ left: `${(toSlider(freeUpTo) / (curve ? STEPS : max)) * 100}%` }}
          >
            {freeUpTo} free
          </span>
        </div>
      ) : null}
    </div>
  );
}
