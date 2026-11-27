/*
 * Comparison hub (`/vs`): side-by-side directory of every published dimmi vs
 * rival breakdown, pricing check dates, and direct links to live bill tools.
 */
import { RiArrowRightLine } from "@remixicon/react";

import { ButtonLink } from "@/components/base/buttons/button";

import { MonoLink, Section, SplitHead, Tag } from "../../components/container";
import { CHECKED_ON, CHECKED_ON_ISO } from "../../data/rivals";
import { APP_URL, GITHUB_URL } from "../../links";
import { FREE_CONVERSATIONS } from "../../pricing/prices";
import { FinalCta } from "../../sections/final-cta";
import { formatIsoDate } from "../../vs/rival-tone";

export type ComparisonCard = {
  slug: string;
  href: string;
  rival: string;
  headline: string;
  summary: string;
  keyDiff: string;
  tone: string;
};

const COMPARISON_CARDS: ComparisonCard[] = [
  {
    slug: "intercom",
    href: "/vs/intercom",
    rival: "Intercom",
    headline: "dimmi vs Intercom",
    summary:
      "Compare $29 to $132 seats and $0.99 Fin outcomes against unlimited free seats, BYO model keys, and MIT self-hosting.",
    keyDiff: "$0 seats · no $0.99 outcome fee",
    tone: "#286efa",
  },
  {
    slug: "chatwoot",
    href: "/vs/chatwoot",
    rival: "Chatwoot",
    headline: "dimmi vs Chatwoot",
    summary:
      "Compare pure MIT licensing (no enterprise folder), one Go binary vs Rails and Sidekiq, and BYO AI keys vs Captain credit caps.",
    keyDiff: "100% MIT · single Go binary",
    tone: "#1f93ff",
  },
  {
    slug: "front",
    href: "/vs/front",
    rival: "Front",
    headline: "dimmi vs Front",
    summary:
      "Compare Front's 1-channel Starter limit and $65 omnichannel seats against every support channel unlocked from conversation one.",
    keyDiff: "All channels unlocked · 0% WhatsApp fee",
    tone: "#635bff",
  },
  {
    slug: "freshdesk",
    href: "/vs/freshdesk",
    rival: "Freshdesk",
    headline: "dimmi vs Freshdesk",
    summary:
      "Compare expiring $49 per 100-session Freddy AI packs and Pro/Enterprise SLA locks against transparent conversation billing.",
    keyDiff: "No expiring session packs · MIT code",
    tone: "#25c16f",
  },
];

export function VsHubPage() {
  return (
    <>
      <Section frameClassName="flex flex-col gap-8">
        <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-2 text-[13px] text-text-tertiary">
          <a href="/" className="transition-colors hover:text-text-primary">
            Home
          </a>
          <span aria-hidden>/</span>
          <span className="text-text-primary">Compare</span>
        </nav>

        <div className="flex max-w-3xl flex-col gap-5">
          <Tag>Compare dimmi</Tag>
          <h1 className="text-[48px] leading-[1.06] font-medium tracking-[-0.03em] text-balance text-text-primary max-sm:text-[34px]">
            Honest side-by-side comparisons, checked against public pricing pages.
          </h1>
          <p className="text-[17px] leading-7 text-text-secondary">
            Every comparison below checks each vendor's live pricing page, names where that vendor beats dimmi today,
            and shows real monthly bills on dimmi Cloud ({FREE_CONVERSATIONS} free conversations a month, unlimited free
            seats) and MIT self-hosting ($0 software).
          </p>
          <div className="flex flex-wrap items-center gap-3 pt-1">
            <ButtonLink href={`${APP_URL}/signup`} trailingIcon={RiArrowRightLine}>
              Start free on dimmi Cloud
            </ButtonLink>
            <ButtonLink href={GITHUB_URL} variant="secondary">
              Self-host on GitHub
            </ButtonLink>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-separator-border pt-4 font-mono text-[12px] text-text-tertiary">
          <span>
            Updated <time dateTime={CHECKED_ON_ISO}>{formatIsoDate(CHECKED_ON_ISO)}</time>
          </span>
          <span>All vendor pricing pages verified {CHECKED_ON}</span>
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-10">
        <SplitHead
          tag="Head-to-head guides"
          title="Pick the platform you are evaluating today."
          body="Each guide includes an interactive monthly bill duel, real dimmi screenshots, honest trade-offs, and a team profile picker."
        />

        <div className="grid grid-cols-1 gap-5 md:grid-cols-2 [&>*]:min-w-0">
          {COMPARISON_CARDS.map((card) => (
            <a
              key={card.slug}
              href={card.href}
              className="group flex flex-col justify-between gap-5 rounded-2xl border border-border-button-default bg-background-full p-6 transition-colors hover:border-accent-500/45"
            >
              <div className="flex flex-col gap-3">
                <div className="flex items-center justify-between gap-2">
                  <span className="inline-flex items-center gap-2 font-mono text-[12px] text-text-tertiary uppercase">
                    <span className="size-2 rounded-full" style={{ backgroundColor: card.tone }} />
                    {card.rival}
                  </span>
                  <span className="rounded-full bg-accent-500/10 px-2.5 py-0.5 font-mono text-[11px] text-accent-700 dark:text-accent-300">
                    {card.keyDiff}
                  </span>
                </div>
                <h2 className="text-[22px] leading-7 font-medium tracking-[-0.02em] text-text-primary group-hover:text-accent-600 dark:group-hover:text-accent-400">
                  {card.headline}
                </h2>
                <p className="text-[15px] leading-6 text-text-secondary">{card.summary}</p>
              </div>
              <span className="inline-flex items-center gap-1.5 font-mono text-[12px] font-medium text-text-primary uppercase">
                Read full comparison
                <RiArrowRightLine className="size-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden />
              </span>
            </a>
          ))}
        </div>

        <div className="flex flex-wrap gap-6 border-t border-separator-border pt-6">
          <MonoLink href="/pricing">Open the full dimmi pricing page</MonoLink>
          <MonoLink href="/agents">Explore dimmi AI support agents</MonoLink>
          <MonoLink href="/inbox">Explore the shared inbox</MonoLink>
        </div>
      </Section>

      <FinalCta />
    </>
  );
}
