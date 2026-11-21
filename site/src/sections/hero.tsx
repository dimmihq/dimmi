import { RiArrowRightLine, RiCheckLine, RiFileCopyLine, RiGithubFill } from "@remixicon/react";
import { useState } from "react";

import { ButtonLink } from "@/components/base/buttons/button";

import { Eyebrow } from "../components/container";
import { APP_URL, GITHUB_URL } from "../links";
import { HeroProduct } from "../scenes/hero-product";

const INSTALL = "curl -fsSL https://github.com/dimmihq/dimmi/releases/latest/download/install.sh | bash";

function InstallCommand() {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      onClick={() => {
        navigator.clipboard?.writeText(INSTALL).then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1600);
        });
      }}
      className="group flex w-full max-w-[520px] items-center gap-3 rounded-xl border border-border-button-default bg-background-secondary-default px-3.5 py-2.5 text-start font-mono text-caption-1-regular text-text-secondary transition-colors hover:border-border-button-hover"
      aria-label="Copy the self-host install command"
    >
      <span className="text-text-tertiary select-none">$</span>
      <span className="min-w-0 flex-1 truncate">{INSTALL}</span>
      {copied ? (
        <RiCheckLine className="size-4 shrink-0 text-accent-600" aria-hidden />
      ) : (
        <RiFileCopyLine
          className="size-4 shrink-0 text-foreground-icon-tertiary group-hover:text-foreground-icon-primary"
          aria-hidden
        />
      )}
    </button>
  );
}

export function Hero() {
  return (
    <section className="relative overflow-hidden bg-background-full">
      <div className="relative mx-auto grid w-full max-w-[1200px] grid-cols-1 border-x border-separator-border lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)]">
        <div className="relative flex min-w-0 flex-col items-start justify-center gap-7 px-5 pt-16 pb-14 sm:px-12 lg:py-24">
          <a href="#agent" className="group inline-flex items-center gap-2">
            <Eyebrow>New · Procedures for the AI agent</Eyebrow>
            <RiArrowRightLine
              className="size-3.5 text-text-tertiary transition-transform group-hover:translate-x-0.5"
              aria-hidden
            />
          </a>
          <h1 className="text-[56px] leading-[1.02] font-medium tracking-[-0.035em] text-text-primary max-md:text-[46px] max-sm:text-[38px]">
            Customer support <br className="max-sm:hidden" />
            you own.
          </h1>
          <p className="max-w-[30rem] text-[17px] leading-7 text-pretty text-text-secondary">
            Your inbox, AI agent, tickets, workflows, and help center, all open source under MIT. Use our cloud with
            free seats, or host it yourself for $0.
          </p>
          <div className="flex flex-wrap items-center gap-3">
            <ButtonLink href={`${APP_URL}/signup`} trailingIcon={RiArrowRightLine}>
              Start free, no card
            </ButtonLink>
            <ButtonLink href={GITHUB_URL} variant="secondary" leadingIcon={RiGithubFill}>
              Star on GitHub
            </ButtonLink>
          </div>
          <div className="flex w-full flex-col gap-2">
            <span className="font-mono text-[12px] leading-5 font-medium tracking-[0.04em] text-text-tertiary uppercase">
              Or skip our cloud and self-host in 3 minutes
            </span>
            <InstallCommand />
          </div>
        </div>
        <HeroProduct className="border-t border-separator-border lg:border-t-0 lg:border-s" />
      </div>
    </section>
  );
}
