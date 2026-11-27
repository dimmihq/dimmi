/*
 * Two-column hero for /vs comparison pages inside the Section rails.
 * The H1 never names the rival; the rival lives in the breadcrumb, eyebrow,
 * title tag, and direct verdict.
 */
import { RiArrowRightLine, RiGithubFill } from "@remixicon/react";
import type { ReactNode } from "react";

import { ButtonLink } from "@/components/base/buttons/button";

import { Section, Tag } from "../components/container";
import type { Rival } from "../data/rivals";
import { APP_URL, GITHUB_URL } from "../links";
import { SourcesStrip } from "./sources-strip";

export function VsHero({
  rival,
  eyebrow,
  title,
  verdict,
  authorId,
  reviewedById,
  published,
  updated,
  primaryCta = { label: "Start free on dimmi Cloud", href: `${APP_URL}/signup` },
  secondaryCta = { label: "Self-host on GitHub", href: GITHUB_URL, github: true },
  visual,
  hubHref = "/pricing#compare",
}: {
  rival: Pick<Rival, "name" | "pricingUrl" | "checkedOn">;
  eyebrow?: string;
  title: string;
  verdict: ReactNode;
  authorId: string;
  reviewedById?: string;
  published: string;
  updated: string;
  primaryCta?: { label: string; href: string };
  secondaryCta?: { label: string; href: string; github?: boolean };
  visual: ReactNode;
  hubHref?: string;
}) {
  return (
    <Section frameClassName="flex flex-col gap-10">
      <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-2 text-[13px] text-text-tertiary">
        <a href="/" className="transition-colors hover:text-text-primary">
          Home
        </a>
        <span aria-hidden>/</span>
        <a href={hubHref} className="transition-colors hover:text-text-primary">
          Compare
        </a>
        <span aria-hidden>/</span>
        <span className="text-text-primary">dimmi vs {rival.name}</span>
      </nav>

      <div className="grid grid-cols-1 items-start gap-10 lg:grid-cols-[minmax(0,1.08fr)_minmax(0,0.92fr)] lg:gap-12">
        <div className="flex min-w-0 flex-col gap-6">
          <Tag>{eyebrow ?? `dimmi vs ${rival.name}`}</Tag>
          <h1 className="text-[44px] leading-[1.08] font-medium tracking-[-0.03em] text-balance text-text-primary max-sm:text-[32px]">
            {title}
          </h1>
          <div className="max-w-xl text-[17px] leading-7 text-text-secondary">{verdict}</div>
          <div className="flex flex-wrap items-center gap-3 pt-1">
            <ButtonLink href={primaryCta.href} trailingIcon={RiArrowRightLine}>
              {primaryCta.label}
            </ButtonLink>
            <ButtonLink
              href={secondaryCta.href}
              variant="secondary"
              leadingIcon={secondaryCta.github ? RiGithubFill : undefined}
            >
              {secondaryCta.label}
            </ButtonLink>
          </div>
        </div>

        <div className="min-w-0">{visual}</div>
      </div>

      <SourcesStrip
        rival={rival}
        authorId={authorId}
        reviewedById={reviewedById}
        published={published}
        updated={updated}
      />
    </Section>
  );
}
