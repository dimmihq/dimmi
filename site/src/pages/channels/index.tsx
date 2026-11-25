import { RiArrowRightLine } from "@remixicon/react";

import { ButtonLink } from "@/components/base/buttons/button";

import { CHANNELS } from "../../channels";
import { Intro, MonoLink, Section, Tag } from "../../components/container";
import { Questions } from "../../components/product-page";
import { APP_URL, GITHUB_URL } from "../../links";
import { FinalCta } from "../../sections/final-cta";

const questions = [
  {
    question: "Do all channels land in the same inbox?",
    answer:
      "Yes. Every channel goes into one shared inbox with the same customer history, reply timers, workflows, and AI agents. If a customer emails after chatting, you see both threads side by side.",
  },
  {
    question: "Can we send different channels or brands to separate teams?",
    answer:
      "Connect multiple chat windows, email addresses, WhatsApp numbers, Instagram accounts, and Facebook Pages, and point each one at its own inbox and team.",
  },
  {
    question: "Do any channels cost extra?",
    answer:
      "No. All 7 channels are included on dimmi Cloud and in the free self-hosted MIT build. For WhatsApp, you connect through Meta and pay Meta directly with $0 markup from dimmi.",
  },
  {
    question: "What if we want to connect a channel not listed here?",
    answer:
      "Every inbox has a built-in API channel. Use our API and webhooks to send messages from your own app or custom chat bridge right into dimmi.",
  },
];

export function ChannelsIndexPage() {
  return (
    <>
      <section className="relative bg-background-full">
        <div className="relative mx-auto w-full max-w-[1200px] overflow-hidden border-x border-separator-border">
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0 opacity-60 [background-image:linear-gradient(var(--color-separator-border)_1px,transparent_1px),linear-gradient(90deg,var(--color-separator-border)_1px,transparent_1px)] [background-size:64px_64px] [mask-image:radial-gradient(60%_50%_at_50%_30%,black,transparent)]"
          />
          <div className="relative flex flex-col items-center gap-6 px-5 pt-20 pb-16 text-center sm:px-12 md:pt-28 md:pb-20">
            <Tag className="self-center">Channels</Tag>
            <h1 className="max-w-3xl text-[60px] leading-[1.04] font-medium tracking-[-0.035em] text-balance text-text-primary max-md:text-[46px] max-sm:text-[36px]">
              Every customer channel in one inbox.
            </h1>
            <p className="max-w-xl text-[18px] leading-7 text-pretty text-text-secondary">
              Connect live chat, email, Slack, WhatsApp, Instagram, Facebook Messenger, and your own app to one shared
              support inbox.
            </p>
            <div className="mt-2 flex flex-wrap justify-center gap-3">
              <ButtonLink href={`${APP_URL}/signup`} trailingIcon={RiArrowRightLine}>
                Start free, no card
              </ButtonLink>
              <ButtonLink href={GITHUB_URL} variant="secondary">
                Self-host on GitHub
              </ButtonLink>
            </div>
          </div>
        </div>
      </section>

      <Section frameClassName="flex flex-col gap-12">
        <Intro
          title="Pick a channel to see how it works."
          body="Every channel includes full customer history, routing workflows, reply timers, and AI support agents."
        />
        <div className="grid grid-cols-1 gap-5 md:grid-cols-2 lg:grid-cols-3 [&>*]:min-w-0">
          {CHANNELS.map((channel) => (
            <a
              key={channel.href}
              href={channel.href}
              className="group flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-full p-6 transition-colors hover:border-border-button-hover hover:bg-background-secondary-default/40"
            >
              <span className="flex size-11 items-center justify-center rounded-xl border border-border-button-default bg-background-full shadow-2xs">
                <channel.logo className="size-6" />
              </span>
              <div className="flex flex-col gap-1.5">
                <h2 className="text-[18px] leading-6 font-medium tracking-[-0.015em] text-text-primary">
                  {channel.name}
                </h2>
                <p className="text-[15px] leading-6 text-pretty text-text-secondary">{channel.blurb}</p>
              </div>
              <span className="mt-auto inline-flex items-center gap-1.5 pt-2 font-mono text-[12px] leading-5 font-medium tracking-[0.04em] text-text-primary uppercase">
                Explore {channel.name}
                <RiArrowRightLine className="size-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden />
              </span>
            </a>
          ))}
        </div>
        <div className="flex flex-wrap items-center justify-center gap-x-8 gap-y-3 border-t border-separator-border pt-10">
          <MonoLink href="/inbox">Explore the shared inbox</MonoLink>
          <MonoLink href="/agents">See AI support agents</MonoLink>
          <MonoLink href="/workflows">Automate routing with workflows</MonoLink>
          <MonoLink href="/pricing">Check transparent pricing</MonoLink>
        </div>
      </Section>

      <Questions body="What teams ask before connecting their support channels to dimmi." items={questions} />
      <FinalCta />
    </>
  );
}
