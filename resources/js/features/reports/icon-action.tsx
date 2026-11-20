/*
 * A quiet action that is only its icon, for the reports' exports and
 * digest: no border or fill until hovered, named for screen readers and
 * in a tooltip. tip is left off when the action opens something of its
 * own, such as a popover, since one press cannot do both.
 */
import type { ComponentType, Ref } from "react";
import { Button as AriaButton, type ButtonProps as AriaButtonProps } from "react-aria-components";

import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";
import { cx } from "@/utils/cx";

interface IconActionProps extends Omit<AriaButtonProps, "children" | "className"> {
  icon: ComponentType<{ className?: string }>;
  label: string;
  tip?: boolean;
  className?: string;
  ref?: Ref<HTMLButtonElement>;
}

export function IconAction({ icon: Icon, label, tip = true, className, ...props }: IconActionProps) {
  const button = (
    <AriaButton
      {...props}
      aria-label={label}
      className={cx(
        "grid size-7 shrink-0 cursor-pointer place-items-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150",
        "hover:bg-background-tertiary-default hover:text-foreground-icon-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        "disabled:cursor-not-allowed disabled:text-foreground-icon-quaternary disabled:hover:bg-transparent",
        className,
      )}
    >
      <Icon aria-hidden className="size-4" />
    </AriaButton>
  );

  if (!tip) {
    return button;
  }

  return (
    <TooltipTrigger delay={200}>
      {button}
      <Tooltip>{label}</Tooltip>
    </TooltipTrigger>
  );
}
