import { cx } from "@/utils/cx";

import { Section } from "../components/container";
import { SDK_COUNT } from "../sdks";

const facts = [
  { value: "MIT licensed", label: "All the code is open source, with no enterprise paywalls." },
  { value: "Unlimited seats", label: "Add engineers and founders to your inbox for $0." },
  { value: "BYO model keys", label: "Use your own AI keys and save up to 83%." },
  { value: `${SDK_COUNT} typed SDKs`, label: "React, Next.js, Vue, Svelte, Go, PHP, Laravel, and Python." },
];

export function Proof() {
  return (
    <Section flush>
      <dl className="grid grid-cols-2 md:grid-cols-4 [&>*]:min-w-0">
        {facts.map((fact, i) => (
          <div
            key={fact.value}
            className={cx(
              "flex flex-col gap-1 border-separator-border px-5 py-8 sm:px-8 md:py-10",
              i % 2 === 0 && "border-e",
              i === 1 && "md:border-e",
              i < 2 && "max-md:border-b",
            )}
          >
            <dt className="text-[20px] leading-7 font-medium tracking-[-0.015em] text-text-primary">{fact.value}</dt>
            <dd className="text-[15px] leading-6 text-text-secondary">{fact.label}</dd>
          </div>
        ))}
      </dl>
    </Section>
  );
}
