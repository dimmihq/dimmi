/*
 * Concrete switching triggers: why teams leave the rival and how dimmi handles
 * the same constraint, each backed by a source link.
 */
import { RiExternalLinkLine } from "@remixicon/react";

import { Section, SplitHead } from "../components/container";
import type { RivalSwitchTrigger } from "../data/rivals";

export function SwitchTriggers({
  tag = "Why teams switch",
  title,
  body = "",
  rivalName,
  triggers,
}: {
  tag?: string;
  title: string;
  body?: string;
  rivalName: string;
  triggers: RivalSwitchTrigger[];
}) {
  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead tag={tag} title={title} body={body} />
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
        {triggers.map((item) => (
          <article
            key={item.trigger}
            className="flex flex-col justify-between gap-4 rounded-2xl border border-border-button-default bg-background-full p-6"
          >
            <div className="flex flex-col gap-3">
              <h3 className="text-[18px] leading-6 font-medium tracking-[-0.015em] text-text-primary">
                {item.trigger}
              </h3>
              <dl className="flex flex-col gap-2.5 text-[14px] leading-6">
                <div>
                  <dt className="font-mono text-[11px] tracking-[0.06em] text-text-tertiary uppercase">{rivalName}</dt>
                  <dd className="text-text-secondary">{item.rivalReality}</dd>
                </div>
                <div>
                  <dt className="font-mono text-[11px] tracking-[0.06em] text-accent-600 uppercase dark:text-accent-400">
                    dimmi
                  </dt>
                  <dd className="text-text-primary">{item.dimmiReality}</dd>
                </div>
              </dl>
            </div>
            <a
              href={item.sourceUrl}
              rel="noopener noreferrer"
              target="_blank"
              className="inline-flex items-center gap-1 self-start font-mono text-[12px] text-text-tertiary underline-offset-4 hover:text-text-primary hover:underline"
            >
              {item.sourceLabel}
              <RiExternalLinkLine className="size-3.5 shrink-0" aria-hidden />
            </a>
          </article>
        ))}
      </div>
    </Section>
  );
}
