"use client";

import type { ComponentType, ReactNode, Ref } from "react";
import { Radio as AriaRadio } from "react-aria-components";
import type { RadioProps as AriaRadioProps } from "react-aria-components";
import { cx } from "@/utils/cx";
import { RadioDot } from "./radio";

/**
 * Selectable radio card — the radio flavor of CheckboxCard (Figma checkbox
 * items, node 3793:2966): title + optional description on the left, the
 * radio dot on the right.
 *   card    radius/2lg (10px), 1px border/button/default, pl 16 pr 20 py 12
 *   hover   bg background/primary/hover (#f7f7f7)
 *   title   Body 1/Medium, text/primary
 *   desc    Body 1/Regular, text/secondary
 *
 * The whole card selects the radio (react-aria Radio renders the label).
 * Must live inside a `RadioGroup`, which provides single-selection and
 * arrow-key navigation across cards.
 */

export interface RadioCardProps extends Omit<AriaRadioProps, "children"> {
  title: ReactNode;
  description?: ReactNode;
  /** Optional leading icon, drawn in the icon tone beside the title. */
  icon?: ComponentType<{ className?: string; "aria-hidden"?: boolean | "true" | "false" }>;
  ref?: Ref<HTMLLabelElement>;
}

export function RadioCard({ className, title, description, icon: Icon, ref, ...props }: RadioCardProps) {
  return (
    <AriaRadio
      ref={ref}
      {...props}
      className={(state) =>
        cx(
          "group flex w-full items-start justify-between gap-3 rounded-2lg border py-3 pe-4 ps-4 select-none",
          state.isSelected ? "border-accent-500" : "border-border-button-default",
          "transition-colors duration-150 ease",
          state.isHovered && !state.isDisabled
            ? "bg-background-primary-hover"
            : "bg-background-primary-default",
          state.isDisabled ? "cursor-not-allowed opacity-50" : "cursor-pointer",
          typeof className === "function" ? className(state) : className,
        )
      }
    >
      {(state) => (
        <>
          <span className="flex min-w-0 items-start gap-2.5">
            {Icon ? <Icon aria-hidden className="mt-0.5 size-4 shrink-0 text-foreground-icon-secondary" /> : null}
            <span className="flex min-w-0 flex-col gap-0.5">
              <span className="text-body-medium text-text-primary">{title}</span>
              {description !== undefined && description !== null && (
                <span className="text-body-2-regular text-pretty text-text-secondary">{description}</span>
              )}
            </span>
          </span>
          <span className="flex shrink-0 items-center py-1">
            <RadioDot size="md" selected={state.isSelected} focusVisible={state.isFocusVisible} />
          </span>
        </>
      )}
    </AriaRadio>
  );
}
