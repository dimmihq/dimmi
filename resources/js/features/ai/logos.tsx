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

/*
 * DeepSeekMark is the DeepSeek mark, the maker of the DeepSeek models.
 */
function DeepSeekMark({ className, style }: { className?: string; style?: CSSProperties }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden className={className} style={style}>
      <path d="M23.748 4.482c-.254-.124-.364.113-.512.234-.051.039-.094.09-.137.136-.372.397-.806.657-1.373.626-.829-.046-1.537.214-2.163.848-.133-.782-.575-1.248-1.247-1.548-.352-.156-.708-.311-.955-.65-.172-.241-.219-.51-.305-.774-.055-.16-.11-.323-.293-.35-.2-.031-.278.136-.356.276-.313.572-.434 1.202-.422 1.84.027 1.436.633 2.58 1.838 3.393.137.093.172.187.129.323-.082.28-.18.552-.266.833-.055.179-.137.217-.329.14a5.526 5.526 0 0 1-1.736-1.18c-.857-.828-1.631-1.742-2.597-2.458a11.365 11.365 0 0 0-.689-.471c-.985-.957.13-1.743.388-1.836.27-.098.093-.432-.779-.428-.872.004-1.67.295-2.687.684a3.055 3.055 0 0 1-.465.137 9.597 9.597 0 0 0-2.883-.102c-1.885.21-3.39 1.102-4.497 2.623C.082 8.606-.231 10.684.152 12.85c.403 2.284 1.569 4.175 3.36 5.653 1.858 1.533 3.997 2.284 6.438 2.14 1.482-.085 3.133-.284 4.994-1.86.47.234.962.327 1.78.397.63.059 1.236-.03 1.705-.128.735-.156.684-.837.419-.961-2.155-1.004-1.682-.595-2.113-.926 1.096-1.296 2.746-2.642 3.392-7.003.05-.347.007-.565 0-.845-.004-.17.035-.237.23-.256a4.173 4.173 0 0 0 1.545-.475c1.396-.763 1.96-2.015 2.093-3.517.02-.23-.004-.467-.247-.588zM11.581 18c-2.089-1.642-3.102-2.183-3.52-2.16-.392.024-.321.471-.235.763.09.288.207.486.371.739.114.167.192.416-.113.603-.673.416-1.842-.14-1.897-.167-1.361-.802-2.5-1.86-3.301-3.307-.774-1.393-1.224-2.887-1.298-4.482-.02-.386.093-.522.477-.592a4.696 4.696 0 0 1 1.529-.039c2.132.312 3.946 1.265 5.468 2.774.868.86 1.525 1.887 2.202 2.891.72 1.066 1.494 2.082 2.48 2.914.348.292.625.514.891.677-.802.09-2.14.11-3.054-.614zm1-6.44a.306.306 0 0 1 .415-.287.302.302 0 0 1 .2.288.306.306 0 0 1-.31.307.303.303 0 0 1-.304-.308zm3.11 1.596c-.2.081-.399.151-.59.16a1.245 1.245 0 0 1-.798-.254c-.274-.23-.47-.358-.552-.758a1.73 1.73 0 0 1 .016-.588c.07-.327-.008-.537-.239-.727-.187-.156-.426-.199-.688-.199a.559.559 0 0 1-.254-.078.253.253 0 0 1-.114-.358c.028-.054.16-.186.192-.21.356-.202.767-.136 1.146.016.352.144.618.408 1.001.782.391.451.462.576.685.914.176.265.336.537.445.848.067.195-.019.354-.25.452z" />
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
  if (id.startsWith("deepseek")) {
    return { mark: DeepSeekMark, color: "#4D6BFE" };
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
 * claude-*, OpenAI for gpt-*, Z.ai for glm-*, DeepSeek for deepseek-*.
 */
export function ModelLogo({ model, size = "sm", tile = false }: { model: string; size?: LogoSize; tile?: boolean }) {
  return <Logo look={modelLook(model)} size={size} tile={tile} />;
}
