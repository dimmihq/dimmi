/*
 * The frame of the customer pages: one panel with the section's title,
 * tabs between people, companies and the blocked, and the page's own
 * actions on the right.
 */
import { Link, useLocation } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { cx } from "@/utils/cx";

const tabs = [
  { to: "/d/customers", label: "People" },
  { to: "/d/customers/companies", label: "Companies" },
  { to: "/d/customers/blocked", label: "Blocked" },
] as const;

function Tabs() {
  const { pathname } = useLocation();

  return (
    <nav aria-label="Customers" className="flex items-center gap-1 px-4 pb-3">
      {tabs.map((tab) => {
        const current = pathname === tab.to;

        return (
          <Link
            key={tab.to}
            to={tab.to}
            aria-current={current ? "page" : undefined}
            className={cx(
              "rounded-full px-3 py-1 text-body-2-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-border-focus-ring",
              current
                ? "bg-background-tertiary-default text-text-primary"
                : "text-text-secondary hover:bg-background-secondary-hover",
            )}
          >
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}

export function CustomersFrame({
  title = "Customers",
  actions,
  tabs = true,
  children,
}: {
  title?: ReactNode;
  actions?: ReactNode;
  tabs?: boolean;
  children: ReactNode;
}) {
  return (
    <Panel label="Customers" className="h-full flex-1">
      <PageHeader title={title}>{actions}</PageHeader>
      {tabs ? <Tabs /> : null}
      <ScrollArea>{children}</ScrollArea>
    </Panel>
  );
}
