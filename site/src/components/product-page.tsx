/*
 * The pieces every product page is built from: a centred hero over the
 * page's main capture, feature cards that each hold one moving capture on
 * a flat panel with the claim under it, an open grid of smaller features,
 * related products and questions. A page supplies its words and captures.
 */
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { type RemixiconComponentType, RiArrowRightLine } from "@remixicon/react";
import { type ReactNode, useState } from "react";

import { Accordion, AccordionItem } from "@/components/base/accordion/accordion";
import { ButtonLink } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

import { APP_URL, GITHUB_URL } from "../links";
import { type FlowName, ShotFlow } from "../scenes/shot-flow";
import { useScene } from "../scenes/use-scene";
import type { SHOTS } from "../shots";
import { Features, Intro, Panel, Section, Tag } from "./container";
import { ProductShot } from "./product-shot";

type Icon = RemixiconComponentType;
type FeatureIconType = PhosphorIcon;

export function ProductHero({
  eyebrow,
  title,
  body,
  shot,
  shotAlt,
  visual,
}: {
  eyebrow: string;
  title: ReactNode;
  body: ReactNode;
  /* The page's main capture, or a drawn visual where no capture exists yet. */
  shot?: keyof typeof SHOTS;
  shotAlt?: string;
  visual?: ReactNode;
}) {
  return (
    <section className="relative bg-background-full">
      <div className="relative mx-auto w-full max-w-[1200px] overflow-hidden border-x border-separator-border">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 opacity-60 [background-image:linear-gradient(var(--color-separator-border)_1px,transparent_1px),linear-gradient(90deg,var(--color-separator-border)_1px,transparent_1px)] [background-size:64px_64px] [mask-image:radial-gradient(60%_50%_at_50%_30%,black,transparent)]"
        />
        <div className="relative flex flex-col items-center gap-6 px-5 pt-20 pb-14 text-center sm:px-12 md:pt-28">
          <Tag className="self-center">{eyebrow}</Tag>
          <h1 className="max-w-3xl text-[60px] leading-[1.04] font-medium tracking-[-0.035em] text-balance text-text-primary max-md:text-[46px] max-sm:text-[36px]">
            {title}
          </h1>
          <p className="max-w-xl text-[18px] leading-7 text-pretty text-text-secondary">{body}</p>
          <div className="mt-2 flex flex-wrap justify-center gap-3">
            <ButtonLink href={`${APP_URL}/signup`} trailingIcon={RiArrowRightLine}>
              Start free, no card
            </ButtonLink>
            <ButtonLink href={GITHUB_URL} variant="secondary">
              Self-host on GitHub
            </ButtonLink>
          </div>
        </div>
        <div className="site-panel relative flex justify-center border-t border-separator-border px-5 pt-12 sm:px-12 md:pt-16">
          {shot ? <ProductShot name={shot} alt={shotAlt ?? ""} bleed /> : <div className="contents">{visual}</div>}
        </div>
      </div>
    </section>
  );
}

export function ChannelStrip({
  label,
  items,
}: {
  label: string;
  items: { icon: Icon; name: string; href?: string }[];
}) {
  return (
    <Section frameClassName="!py-9 flex flex-wrap items-center justify-between gap-x-6 gap-y-5">
      <span className="font-mono text-[12px] leading-5 font-medium tracking-[0.04em] text-text-tertiary uppercase">
        {label}
      </span>
      {items.map((item) =>
        item.href ? (
          <a
            key={item.name}
            href={item.href}
            className="inline-flex items-center gap-2.5 text-[16px] leading-7 text-text-primary transition-colors hover:text-accent-600 dark:hover:text-accent-400"
          >
            <item.icon className="size-5 text-foreground-icon-secondary" aria-hidden />
            {item.name}
          </a>
        ) : (
          <span key={item.name} className="inline-flex items-center gap-2.5 text-[16px] leading-7 text-text-primary">
            <item.icon className="size-5 text-foreground-icon-secondary" aria-hidden />
            {item.name}
          </span>
        ),
      )}
    </Section>
  );
}

export type Feature = {
  title: string;
  body: string;
  /* The recorded click-through the card plays. */
  flow: FlowName;
  alt: string;
  /* Which part of the capture the card keeps in view. */
  anchor: "bottom" | "top" | "top-right";
};

function FeatureCard({ feature, large }: { feature: Feature; large: boolean }) {
  return (
    <div className="flex flex-col gap-6">
      <Panel className={cx("pt-10", large ? "h-[460px]" : "h-[420px]")}>
        <ShotFlow
          name={feature.flow}
          alt={feature.alt}
          anchor={feature.anchor}
          maxHeight={feature.anchor === "bottom" ? 380 : 2000}
          maxWidth={large ? 2000 : 330}
        />
      </Panel>
      <div className="flex flex-col gap-2">
        <h3
          className={cx(
            "font-medium tracking-[-0.02em] text-text-primary",
            large ? "text-[24px] leading-8" : "text-[20px] leading-7",
          )}
        >
          {feature.title}
        </h3>
        <p className="max-w-[480px] text-[16px] leading-7 text-pretty text-text-secondary">{feature.body}</p>
      </div>
    </div>
  );
}

/* Two large cards, then a row of three smaller ones. */
export function FeatureCards({ title, body, features }: { title: string; body: string; features: Feature[] }) {
  return (
    <Section frameClassName="flex flex-col gap-16">
      <Intro title={title} body={body} />
      <div className="flex flex-col gap-14">
        <div className="grid grid-cols-1 gap-x-6 gap-y-14 lg:grid-cols-2 [&>*]:min-w-0">
          {features.slice(0, 2).map((feature) => (
            <FeatureCard key={feature.title} feature={feature} large />
          ))}
        </div>
        <div className="grid grid-cols-1 gap-x-6 gap-y-14 md:grid-cols-3 [&>*]:min-w-0">
          {features.slice(2).map((feature) => (
            <FeatureCard key={feature.title} feature={feature} large={false} />
          ))}
        </div>
      </div>
    </Section>
  );
}

export function FeatureGrid({
  title,
  body,
  items,
}: {
  title: string;
  body?: string;
  items: { icon: FeatureIconType; title: string; body: string; link?: { href: string; label: string } }[];
}) {
  return (
    <Section frameClassName="flex flex-col gap-16">
      <Intro title={title} body={body} />
      <Features items={items} />
    </Section>
  );
}

/* Three steps side by side: a product fragment over a title and a line. */
export function Steps({
  title,
  body,
  items,
}: {
  title: string;
  body?: string;
  items: { title: string; body: string; visual: ReactNode }[];
}) {
  return (
    <Section frameClassName="flex flex-col gap-16">
      <Intro title={title} body={body} />
      <div className="grid grid-cols-1 gap-6 md:grid-cols-3 [&>*]:min-w-0">
        {items.map((item) => (
          <Panel key={item.title} className="flex flex-col">
            <div className="flex h-[250px] items-center justify-center overflow-hidden px-6 pt-8" aria-hidden>
              {item.visual}
            </div>
            <div className="flex flex-col gap-1 px-7 pt-6 pb-8">
              <h3 className="text-[16px] leading-7 font-medium text-text-primary">{item.title}</h3>
              <p className="text-[15px] leading-6 text-pretty text-text-secondary">{item.body}</p>
            </div>
          </Panel>
        ))}
      </div>
    </Section>
  );
}

/* Rows that alternate sides: a short claim beside a large product fragment. */
export function Rows({
  title,
  rows,
}: {
  title: string;
  rows: { label: string; title: string; body: string; visual: ReactNode }[];
}) {
  return (
    <Section frameClassName="flex flex-col gap-16">
      <Intro title={title} />
      <div className="flex flex-col gap-6">
        {rows.map((row, i) => (
          <div key={row.title} className="grid grid-cols-1 items-center gap-x-6 gap-y-8 lg:grid-cols-2 [&>*]:min-w-0">
            <div className={cx("flex flex-col gap-3 lg:px-14", i % 2 === 0 ? "lg:order-1" : "lg:order-2")}>
              <span className="font-mono text-[12px] leading-5 font-medium tracking-[0.04em] text-text-tertiary uppercase">
                {row.label}
              </span>
              <h3 className="max-w-[360px] text-[24px] leading-8 font-medium tracking-[-0.02em] text-balance text-text-primary">
                {row.title}
              </h3>
              <p className="max-w-[380px] text-[16px] leading-7 text-pretty text-text-secondary">{row.body}</p>
            </div>
            <Panel
              className={cx(
                "flex h-[480px] items-end justify-center px-6 pt-12 sm:px-10",
                i % 2 === 0 ? "lg:order-2" : "lg:order-1",
              )}
            >
              {row.visual}
            </Panel>
          </div>
        ))}
      </div>
    </Section>
  );
}

/*
 * How something works, in numbered steps: the steps advance on their own
 * and the visual follows the one in play. Clicking a step picks it.
 * Motion follows BoardUI's Accordion and Reveal curves: height eases over
 * 320ms on cubic-bezier(0.22, 1, 0.36, 1) while copy softens in from a
 * 4px blur and short lift. Reduced motion switches transitions off.
 */
const BOARDUI_EASE = "ease-[cubic-bezier(0.22,1,0.36,1)]";

export function HowItWorks({
  label,
  title,
  body,
  steps,
  visual,
}: {
  label: string;
  title: string;
  body: string;
  steps: { title: string; body: string }[];
  visual: (step: number) => ReactNode;
}) {
  const { ref, step } = useScene(steps.length, { interval: 3600, hold: 3600 });
  const [picked, setPicked] = useState<number | null>(null);
  const active = picked ?? step;

  return (
    <Section frameClassName="grid grid-cols-1 gap-x-16 gap-y-12 lg:grid-cols-[minmax(0,440px)_1fr] lg:items-start [&>*]:min-w-0">
      <div className="flex flex-col gap-5 lg:min-h-[628px]">
        <span className="font-mono text-[12px] leading-5 font-medium tracking-[0.04em] text-text-tertiary uppercase">
          {label}
        </span>
        <h2 className="text-[40px] leading-[1.12] font-medium tracking-[-0.025em] text-balance text-text-primary max-sm:text-[30px]">
          {title}
        </h2>
        <p className="text-[16px] leading-7 text-pretty text-text-secondary">{body}</p>
        <ol className="flex flex-col pt-6">
          {steps.map((item, i) => {
            const open = active === i;
            return (
              <li key={item.title} className="border-t border-separator-border">
                <button
                  type="button"
                  onClick={() => setPicked(i)}
                  aria-expanded={open}
                  className={cx(
                    "group flex w-full cursor-pointer items-start gap-4 rounded-lg py-4 text-start outline-none",
                    "transition-colors duration-200 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                    BOARDUI_EASE,
                  )}
                >
                  <span
                    className={cx(
                      "mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-md font-mono text-[12px] font-medium",
                      "transition-[background-color,color,transform] duration-300 motion-reduce:transition-none",
                      BOARDUI_EASE,
                      open
                        ? "bg-background-secondary-default text-text-primary shadow-[inset_0_0_0_1px_var(--color-border-button-default)]"
                        : "text-text-tertiary group-hover:text-text-secondary",
                    )}
                  >
                    {i + 1}
                  </span>
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span
                      className={cx(
                        "text-[16px] leading-6 font-medium transition-colors duration-300 motion-reduce:transition-none",
                        BOARDUI_EASE,
                        open ? "text-text-primary" : "text-text-tertiary group-hover:text-text-secondary",
                      )}
                    >
                      {item.title}
                    </span>
                    <span
                      className={cx(
                        "grid transition-[grid-template-rows] duration-300 motion-reduce:transition-none",
                        BOARDUI_EASE,
                        open ? "grid-rows-[1fr]" : "grid-rows-[0fr]",
                      )}
                    >
                      <span className="overflow-hidden">
                        <span
                          className={cx(
                            "block pt-1.5 text-[15px] leading-6 text-pretty text-text-secondary",
                            "transition-[opacity,transform,filter] duration-300 motion-reduce:transition-none",
                            BOARDUI_EASE,
                            open
                              ? "translate-y-0 opacity-100 blur-none"
                              : "motion-safe:-translate-y-1 opacity-0 motion-safe:blur-[4px]",
                          )}
                        >
                          {item.body}
                        </span>
                      </span>
                    </span>
                  </span>
                </button>
              </li>
            );
          })}
        </ol>
      </div>
      <Panel className="site-dots flex h-[580px] items-center justify-center px-6 py-8">
        <div ref={ref}>{visual(active)}</div>
      </Panel>
    </Section>
  );
}

/* A plain list of things to start from, each a row with an arrow. */
export function LinkList({ title, body, items }: { title: string; body?: string; items: string[] }) {
  return (
    <Section frameClassName="flex flex-col gap-14">
      <Intro title={title} body={body} />
      <ul className="mx-auto grid w-full max-w-[920px] grid-cols-1 gap-x-10 md:grid-cols-2">
        {items.map((item) => (
          <li
            key={item}
            className="flex items-center justify-between gap-4 border-t border-separator-border py-4 text-[16px] leading-7 text-text-primary"
          >
            {item}
            <RiArrowRightLine className="size-4 shrink-0 text-foreground-icon-tertiary" aria-hidden />
          </li>
        ))}
      </ul>
    </Section>
  );
}

export function Questions({ body, items }: { body?: string; items: { question: string; answer: string }[] }) {
  return (
    <Section frameClassName="grid grid-cols-1 gap-10 lg:grid-cols-[0.8fr_1.2fr] [&>*]:min-w-0">
      <div className="flex flex-col gap-4">
        <h2 className="text-[40px] leading-[1.12] font-medium tracking-[-0.025em] text-text-primary max-sm:text-[30px]">
          Questions and answers.
        </h2>
        {body ? <p className="max-w-[300px] text-[16px] leading-7 text-text-secondary">{body}</p> : null}
      </div>
      <Accordion variant="divided">
        {items.map((item) => (
          <AccordionItem key={item.question} id={item.question} title={item.question}>
            <p className="text-[16px] leading-7 text-text-secondary">{item.answer}</p>
          </AccordionItem>
        ))}
      </Accordion>
    </Section>
  );
}
