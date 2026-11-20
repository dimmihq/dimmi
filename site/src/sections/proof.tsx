import { cx } from "@/utils/cx";

import { Section } from "../components/container";

const facts = [
  { value: "MIT licensed", label: "Every feature. No enterprise edition." },
  { value: "One command", label: "Puts it on your server, with HTTPS" },
  { value: "Your model key", label: "AI at the human price" },
  { value: "11 SDKs", label: "React, Vue, Go, PHP, Python and more" },
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
