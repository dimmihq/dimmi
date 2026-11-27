/*
 * The choice between dimmi's included AI and the workspace's own model,
 * as two cards. The own-model card stacks the marks of the providers a
 * workspace can connect.
 */
import { Radio, RadioGroup } from "react-aria-components";

import { RadioDot } from "@/components/base/radio/radio";
import { ProviderLogo } from "@/features/ai/logos";
import { cx } from "@/utils/cx";

const INCLUDED = "included";
const OWN = "own";

/* One round tile in the stack, ringed in the card's colour so the overlap reads. */
function Disc({ children, label }: { children: React.ReactNode; label: string }) {
  return (
    <span
      title={label}
      className="relative flex size-10 items-center justify-center rounded-full border border-border-button-default bg-background-primary-default shadow-[0_0_0_3px_var(--color-background-full)]"
    >
      {children}
    </span>
  );
}

/*
 * The providers' own marks, from theSVG, in site/public/logos. A mark drawn
 * in black has a white twin for dark mode. OpenAI's mark stands for both
 * API keys and ChatGPT plans; DeepSeek, Mistral and OpenRouter connect as
 * OpenAI-compatible servers.
 */
const providers = [
  { name: "Anthropic", light: "anthropic-light", dark: "anthropic-dark" },
  { name: "OpenAI and ChatGPT", light: "openai-light", dark: "openai-dark" },
  { name: "DeepSeek", light: "deepseek-default" },
  { name: "Mistral", light: "mistral-ai-default" },
  { name: "OpenRouter", light: "openrouter-light", dark: "openrouter-dark" },
];

function Stack() {
  return (
    <span className="flex items-center -space-x-2.5">
      {providers.map((p) => (
        <Disc key={p.name} label={p.name}>
          <img
            src={`/logos/${p.light}.svg`}
            alt={p.name}
            width={20}
            height={20}
            className={cx("size-5", p.dark && "dark:hidden")}
          />
          {p.dark ? (
            <img src={`/logos/${p.dark}.svg`} alt="" width={20} height={20} className="hidden size-5 dark:block" />
          ) : null}
        </Disc>
      ))}
    </span>
  );
}

function Card({ value, title, visual }: { value: string; title: string; visual: React.ReactNode }) {
  return (
    <Radio
      value={value}
      className={({ isSelected, isHovered }) =>
        cx(
          "group relative flex cursor-pointer flex-col gap-4 rounded-xl border bg-background-full p-5 outline-none transition-[border-color,box-shadow] duration-150",
          isSelected
            ? "border-accent-500 shadow-[0_0_0_1px_var(--color-accent-500)]"
            : isHovered
              ? "border-border-button-hover"
              : "border-border-button-default",
        )
      }
    >
      {({ isSelected, isFocusVisible }) => (
        <>
          <div className="flex items-start justify-between gap-3">
            {visual}
            <RadioDot selected={isSelected} focusVisible={isFocusVisible} size="md" />
          </div>
          <span className="text-[15px] leading-6 font-medium text-text-primary">{title}</span>
        </>
      )}
    </Radio>
  );
}

export function ModelChoice({ own, onChange }: { own: boolean; onChange: (own: boolean) => void }) {
  return (
    <RadioGroup
      aria-label="AI model"
      value={own ? OWN : INCLUDED}
      onChange={(value) => onChange(value === OWN)}
      className="grid grid-cols-1 gap-3 sm:grid-cols-2"
    >
      <Card
        value={INCLUDED}
        title="AI included"
        visual={
          <Disc label="dimmi">
            <ProviderLogo provider="dimmi" size="md" tile={false} />
          </Disc>
        }
      />
      <Card value={OWN} title="Your own key or ChatGPT plan" visual={<Stack />} />
    </RadioGroup>
  );
}
