import { RiCheckLine } from "@remixicon/react";

import { MonoLink, Section } from "../components/container";

const security = [
  "SAML single sign-on",
  "Passkeys and two-factor",
  "Roles and permissions",
  "Audit log",
  "Retention and redaction",
  "Erasure requests",
];

const pricing = [
  { value: "Free", label: "Self-hosted, every feature" },
  { value: "Per conversation", label: "On dimmi Cloud. Seats are free." },
  { value: "100 free", label: "Conversations, every month" },
];

const title = "text-[32px] leading-[1.14] font-medium tracking-[-0.025em] text-text-primary max-sm:text-[26px]";

export function Trust() {
  return (
    <Section frameClassName="grid grid-cols-1 gap-y-16 lg:grid-cols-2 [&>*]:min-w-0">
      <div className="flex flex-col gap-4 lg:pe-16">
        <h3 className={title}>Pricing you can do in your head.</h3>
        <p className="text-[16px] leading-7 text-text-secondary">There are no plans. Everyone gets every feature.</p>
        <dl className="mt-4 flex flex-col">
          {pricing.map((row) => (
            <div key={row.value} className="flex items-baseline gap-6 border-t border-separator-border py-5">
              <dt className="w-52 shrink-0 max-sm:w-44 text-[24px] leading-8 font-medium tracking-[-0.02em] text-text-primary">
                {row.value}
              </dt>
              <dd className="text-[15px] leading-6 text-text-secondary">{row.label}</dd>
            </div>
          ))}
        </dl>
        <span className="mt-2">
          <MonoLink href="/pricing">See pricing</MonoLink>
        </span>
      </div>
      <div className="flex flex-col gap-4 lg:border-s lg:border-separator-border lg:ps-16">
        <h3 className={title}>Security without the enterprise tier.</h3>
        <p className="text-[16px] leading-7 text-text-secondary">
          Included on dimmi Cloud and on your own servers. No upsell.
        </p>
        <ul className="mt-4 grid gap-x-6 gap-y-3 sm:grid-cols-2">
          {security.map((item) => (
            <li key={item} className="flex items-center gap-2.5 text-[15px] leading-6 text-text-primary">
              <RiCheckLine className="size-4 text-accent-600" aria-hidden />
              {item}
            </li>
          ))}
        </ul>
        <span className="mt-4">
          <MonoLink href="/security">Read about security</MonoLink>
        </span>
      </div>
    </Section>
  );
}
