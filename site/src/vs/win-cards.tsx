/*
 * Side-by-side strengths: where the rival genuinely wins and where dimmi wins.
 */
import { RiCheckLine } from "@remixicon/react";

import { Section, SplitHead } from "../components/container";

export function WinCards({
  tag = "Where each tool wins",
  title,
  body = "",
  rivalName,
  rivalWins,
  dimmiWins,
}: {
  tag?: string;
  title: string;
  body?: string;
  rivalName: string;
  rivalWins: string[];
  dimmiWins: string[];
}) {
  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead tag={tag} title={title} body={body} />
      <div className="grid grid-cols-1 gap-5 md:grid-cols-2 [&>*]:min-w-0">
        <div className="flex flex-col gap-5 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-7">
          <div className="flex items-center gap-2.5">
            <span className="size-2.5 shrink-0 rounded-full" style={{ backgroundColor: "var(--rival, #71717a)" }} />
            <h3 className="text-[19px] leading-7 font-medium tracking-[-0.015em] text-text-primary">
              Where {rivalName} wins
            </h3>
          </div>
          <ul className="flex flex-col gap-3.5 text-[15px] leading-6 text-text-secondary">
            {rivalWins.map((item) => (
              <li key={item} className="flex items-start gap-2.5">
                <RiCheckLine className="mt-1 size-4 shrink-0 text-text-tertiary" aria-hidden />
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className="flex flex-col gap-5 rounded-2xl border border-accent-500/35 bg-accent-500/[0.04] p-6 sm:p-7">
          <div className="flex items-center gap-2.5">
            <span className="size-2.5 shrink-0 rounded-full bg-accent-500" />
            <h3 className="text-[19px] leading-7 font-medium tracking-[-0.015em] text-text-primary">
              Where dimmi wins
            </h3>
          </div>
          <ul className="flex flex-col gap-3.5 text-[15px] leading-6 text-text-primary">
            {dimmiWins.map((item) => (
              <li key={item} className="flex items-start gap-2.5">
                <RiCheckLine className="mt-1 size-4 shrink-0 text-accent-600 dark:text-accent-400" aria-hidden />
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </Section>
  );
}
