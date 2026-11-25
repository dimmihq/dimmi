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
  { value: "$0", label: "Self-hosted on your own server, every feature included" },
  { value: "100 free", label: "Conversations every month on dimmi Cloud, no card needed" },
  { value: "Per thread", label: "Rates drop as you grow, and seats are always free" },
];

const title = "text-[32px] leading-[1.14] font-medium tracking-[-0.025em] text-text-primary max-sm:text-[26px]";

export function Trust() {
  return (
    <Section frameClassName="grid grid-cols-1 gap-y-16 lg:grid-cols-2 [&>*]:min-w-0">
      <div className="flex flex-col gap-4 lg:pe-16">
        <h3 className={title}>Pricing you can do in 10 seconds.</h3>
        <p className="text-[16px] leading-7 text-text-secondary">
          No plans to compare and no seat fees. Every workspace gets the full product on day one.
        </p>
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
          <MonoLink href="/pricing">Calculate your bill</MonoLink>
        </span>
      </div>
      <div className="flex flex-col gap-4 lg:border-s lg:border-separator-border lg:ps-16">
        <h3 className={title}>Security without the enterprise tax.</h3>
        <p className="text-[16px] leading-7 text-text-secondary">
          SAML SSO, passkeys, and audit logs come standard on Cloud and self-hosted. No sales call needed.
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
          <MonoLink href="/privacy#security-overview">Read about security</MonoLink>
        </span>
      </div>
    </Section>
  );
}
