import { RiArrowRightLine, RiCheckLine, RiGithubFill } from "@remixicon/react";

import { ButtonLink } from "@/components/base/buttons/button";

import { APP_URL, GITHUB_URL } from "../links";
import { HeroProduct } from "../scenes/hero-product";

const highlights = [
  "No per-seat pricing",
  "Unlimited support agents",
  "Bring your own AI keys",
  "Fully open source (MIT)",
];

export function Hero() {
  return (
    <section className="relative overflow-hidden bg-background-full">
      <div className="relative mx-auto grid w-full max-w-[1200px] grid-cols-1 border-x border-separator-border lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)]">
        <div className="relative flex min-w-0 flex-col items-start justify-center gap-7 px-5 pt-16 pb-14 sm:px-12 lg:py-24">
          <a
            href="/agents"
            className="group inline-flex max-w-full items-center gap-2.5 rounded-full border border-border-button-default bg-background-secondary-default py-1 ps-1 pe-3 transition-colors hover:border-border-button-hover"
          >
            <span className="shrink-0 rounded-full bg-accent-500/15 px-2.5 py-1 font-mono text-[11px] leading-4 font-medium tracking-[0.08em] text-accent-600 uppercase">
              New
            </span>
            <span className="min-w-0 truncate text-caption-1-medium text-text-secondary">
              <span className="sm:hidden">Agents for every queue and brand</span>
              <span className="max-sm:hidden">Dedicated support agents for every queue and brand</span>
            </span>
            <RiArrowRightLine
              className="size-3.5 shrink-0 text-foreground-icon-tertiary transition-transform group-hover:translate-x-0.5"
              aria-hidden
            />
          </a>
          <h1 className="text-[56px] leading-[1.02] font-medium tracking-[-0.035em] text-balance text-text-primary max-md:text-[46px] max-sm:text-[38px]">
            The open customer support platform.
          </h1>
          <p className="max-w-[30rem] text-[17px] leading-7 text-pretty text-text-secondary">
            Your shared inbox, specialized support agents, tickets, workflows, and help center, all open source under
            MIT. Run on our cloud with free seats, or host it yourself for $0.
          </p>
          <div className="flex flex-wrap items-center gap-3">
            <ButtonLink href={`${APP_URL}/signup`} trailingIcon={RiArrowRightLine}>
              Start free, no card
            </ButtonLink>
            <ButtonLink href={GITHUB_URL} variant="secondary" leadingIcon={RiGithubFill}>
              Star on GitHub
            </ButtonLink>
          </div>
          <ul className="grid w-full max-w-[30rem] grid-cols-1 gap-x-6 gap-y-2.5 border-t border-separator-border pt-6 sm:grid-cols-2">
            {highlights.map((item) => (
              <li key={item} className="flex items-center gap-2 text-[14px] leading-5 text-text-secondary">
                <RiCheckLine className="size-4 shrink-0 text-accent-600" aria-hidden />
                {item}
              </li>
            ))}
          </ul>
        </div>
        <HeroProduct className="border-t border-separator-border lg:border-t-0 lg:border-s" />
      </div>
    </section>
  );
}
