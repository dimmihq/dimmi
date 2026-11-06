/*
 * The marks of the AI providers and models, drawn inline so they never
 * load from elsewhere. Every page that names a provider or a model shows
 * it through ProviderLogo or ModelLogo, so a connection or a model reads
 * the same in the providers list, the pickers, the overview and usage.
 * Monochrome marks take the text colour and follow light and dark mode;
 * a mark with a brand colour of its own keeps it, on the same neutral
 * tile.
 */
import { RiAnthropicFill, RiClaudeFill, RiOpenaiFill, RiPlug2Line, RiSparkling2Line } from "@remixicon/react";
import type { ComponentType, CSSProperties } from "react";

import { DimmiIconSmall } from "@/features/brand/logo";
import { cx } from "@/utils/cx";

import { providerName } from "./provider-info";

type Mark = ComponentType<{ className?: string; style?: CSSProperties }>;

/*
 * ZaiMark is the Z.ai mark, the maker of the GLM models.
 */
function ZaiMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" fillRule="evenodd" aria-hidden className={className}>
      <path d="M12.105 2L9.927 4.953H.653L2.83 2h9.276zM23.254 19.048L21.078 22h-9.242l2.174-2.952h9.244zM24 2L9.264 22H0L14.736 2H24z" />
    </svg>
  );
}

function DimmiMark({ className }: { className?: string }) {
  return <DimmiIconSmall title="" aria-hidden className={className} />;
}

/*
 * LogoSize is how large a logo draws: xs and sm sit inline beside text,
 * such as in a picker; md and lg are the tiles of list rows and cards.
 */
export type LogoSize = "xs" | "sm" | "md" | "lg";

const tiles: Record<LogoSize, { box: string; mark: string; text: string }> = {
  xs: { box: "size-5 rounded-md", mark: "size-3", text: "text-caption-1-semibold" },
  sm: { box: "size-6 rounded-md", mark: "size-3.5", text: "text-caption-1-semibold" },
  md: { box: "size-8 rounded-lg", mark: "size-[18px]", text: "text-body-2-medium" },
  lg: { box: "size-10 rounded-xl", mark: "size-[22px]", text: "text-body-medium" },
};

const bare: Record<LogoSize, string> = {
  xs: "size-3.5",
  sm: "size-4",
  md: "size-5",
  lg: "size-6",
};

/*
 * Look is one mark and the colour it keeps, if it has one.
 */
interface Look {
  mark: Mark | null;
  color?: string;
  letter?: string;
}

const providers: Record<string, Look> = {
  anthropic: { mark: RiAnthropicFill },
  openai: { mark: RiOpenaiFill },
  chatgpt: { mark: RiOpenaiFill, color: "#10A37F" },
  openai_compatible: { mark: RiPlug2Line },
  dimmi: { mark: DimmiMark },
};

function modelLook(model: string): Look {
  const id = model.toLowerCase();
  if (id.startsWith("claude")) {
    return { mark: RiClaudeFill, color: "#D97757" };
  }
  if (id.startsWith("gpt") || /^o\d/.test(id)) {
    return { mark: RiOpenaiFill };
  }
  if (id.startsWith("glm")) {
    return { mark: ZaiMark };
  }

  return { mark: RiSparkling2Line };
}

/*
 * Logo draws a look on a neutral tile, or bare beside text.
 */
function Logo({ look, size, tile }: { look: Look; size: LogoSize; tile: boolean }) {
  const s = tiles[size];
  const Mark = look.mark;
  const markClass = cx(tile ? s.mark : bare[size], "shrink-0", !look.color && "text-text-primary");
  const glyph = Mark ? (
    <Mark className={markClass} style={look.color ? { color: look.color } : undefined} />
  ) : (
    <span className={cx(s.text, "text-text-secondary")}>{look.letter}</span>
  );

  if (!tile) {
    return (
      <span aria-hidden className="inline-flex shrink-0 items-center justify-center">
        {glyph}
      </span>
    );
  }

  return (
    <span
      aria-hidden
      className={cx(
        "flex shrink-0 items-center justify-center bg-background-primary-default ring-1 ring-border-button-default ring-inset",
        s.box,
      )}
    >
      {glyph}
    </span>
  );
}

/*
 * ProviderLogo is a provider's mark: Anthropic, OpenAI, a ChatGPT plan,
 * an OpenAI-compatible server, dimmi's own models, or a monogram for a
 * provider without a mark of its own, such as Engy.
 */
export function ProviderLogo({
  provider,
  size = "md",
  tile = true,
}: {
  provider: string;
  size?: LogoSize;
  tile?: boolean;
}) {
  const look = providers[provider] ?? { mark: null, letter: providerName(provider).charAt(0).toUpperCase() };

  return <Logo look={look} size={size} tile={tile} />;
}

/*
 * ModelLogo is a model's maker's mark, read from its id: Claude for
 * claude-*, OpenAI for gpt-*, Z.ai for glm-*.
 */
export function ModelLogo({ model, size = "sm", tile = false }: { model: string; size?: LogoSize; tile?: boolean }) {
  return <Logo look={modelLook(model)} size={size} tile={tile} />;
}
