/*
 * Each provider's mark and the one line that says what connecting it
 * means, so the providers page, the add-provider modal and the model
 * picker all show a provider the same way. Brands with a logo of their
 * own wear it on their colour; a provider without one gets a monogram,
 * and dimmi's own models wear the dimmi mark.
 */
import { RiAnthropicFill, RiOpenaiFill, RiPlug2Line } from "@remixicon/react";

import { DimmiIconSmall } from "@/features/brand/logo";
import { cx } from "@/utils/cx";

import { providerName } from "./provider-info";

const box = "flex shrink-0 items-center justify-center rounded-lg ring-1 ring-border-button-default ring-inset";

const sizes = {
  sm: { box: "size-6 rounded-md", icon: "size-3.5", text: "text-caption-1-semibold" },
  md: { box: "size-8", icon: "size-[18px]", text: "text-body-2-medium" },
  lg: { box: "size-10 rounded-xl", icon: "size-[22px]", text: "text-body-medium" },
} as const;

/*
 * brands are the colours a provider's logo sits on.
 */
const brands: Record<string, string> = {
  anthropic: "#D97757",
  openai: "#0D0D0D",
  chatgpt: "#10A37F",
};

/*
 * ProviderMark is a provider's logo tile, at the size of a list row's
 * mark by default.
 */
export function ProviderMark({ provider, size = "md" }: { provider: string; size?: keyof typeof sizes }) {
  const s = sizes[size];

  if (provider === "anthropic" || provider === "openai" || provider === "chatgpt") {
    const Icon = provider === "anthropic" ? RiAnthropicFill : RiOpenaiFill;

    return (
      <span aria-hidden className={cx(box, s.box)} style={{ backgroundColor: brands[provider] }}>
        <Icon className={cx(s.icon, "text-text-white")} />
      </span>
    );
  }

  if (provider === "dimmi") {
    return (
      <span aria-hidden className={cx(box, s.box, "bg-background-primary-default")}>
        <DimmiIconSmall title="" className={cx(s.icon, "text-text-primary")} />
      </span>
    );
  }

  if (provider === "openai_compatible") {
    return (
      <span aria-hidden className={cx(box, s.box, "bg-background-tertiary-default")}>
        <RiPlug2Line className={cx(s.icon, "text-foreground-icon-secondary")} />
      </span>
    );
  }

  return (
    <span aria-hidden className={cx(box, s.box, "bg-background-tertiary-default")}>
      <span className={cx(s.text, "text-text-secondary")}>{providerName(provider).charAt(0).toUpperCase()}</span>
    </span>
  );
}
