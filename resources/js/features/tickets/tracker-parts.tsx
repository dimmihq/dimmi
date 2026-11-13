/*
 * The parts a tracker's page and an incident's page share: the way back,
 * who it reaches as a row of totals, the issue behind it with its
 * status, the AI's summary when there is one, the states its type moves
 * through, and moving it from one to the next.
 */
import { RiArrowLeftLine, RiExternalLinkLine, RiSparkling2Line } from "@remixicon/react";
import { useRouter } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { StatCards } from "@/components/application/dashboard/stat-cards";
import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { Row } from "@/features/inbox/customer/rows";
import type { Impact, TrackerSummary } from "@/lib/api/ticket-trackers";
import type { Ticket, TicketIssue } from "@/lib/api/tickets";
import { formatNumber, formatRelative } from "@/lib/i18n/format";
import { StatePill } from "./state-pill";

import { money } from "./tracker-helpers";

/*
 * Back goes to where the teammate came from: the inbox, the tickets
 * list, or an incident pin.
 */
export function Back() {
  const { t } = useTranslation("trackers");
  const router = useRouter();

  return (
    <button
      type="button"
      aria-label={t("common.back")}
      onClick={() => router.history.back()}
      className="rounded-md p-1 text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <RiArrowLeftLine aria-hidden className="size-4" />
    </button>
  );
}

/*
 * Title is the page header's line: the way back, the reference, the
 * title and the state.
 */
export function Title({ ticket, fallback }: { ticket: Ticket | undefined; fallback: string }) {
  return (
    <span className="flex min-w-0 items-center gap-2">
      <Back />
      <span className="text-text-tertiary">{ticket?.reference}</span>
      <span className="truncate">{ticket?.title ?? fallback}</span>
      {ticket ? <StatePill state={ticket.state} /> : null}
    </span>
  );
}

/*
 * ImpactTotals is who the ticket reaches: customers, companies, revenue
 * at stake and conversations.
 */
export function ImpactTotals({ impact }: { impact: Impact | undefined }) {
  const { t } = useTranslation("trackers");
  if (!impact) {
    return (
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {["a", "b", "c", "d"].map((id) => (
          <Skeleton key={id} className="h-[100px] rounded-2xl" />
        ))}
      </div>
    );
  }

  return (
    <StatCards
      variant="compact"
      className="gap-2"
      stats={[
        { label: t("impact.customers"), hint: t("impact.customersHint"), value: formatNumber(impact.customers) },
        { label: t("impact.companies"), hint: t("impact.companiesHint"), value: formatNumber(impact.companies) },
        { label: t("impact.revenue"), hint: t("impact.revenueHint"), value: money(impact.revenue) },
        {
          label: t("impact.conversations"),
          hint: t("impact.conversationsHint"),
          value: formatNumber(impact.conversations),
        },
      ]}
    />
  );
}

const issueTint: Record<string, ChipColor> = {
  backlog: "gray",
  unstarted: "gray",
  started: "blue",
  done: "lime",
  canceled: "neutral",
};

/*
 * IssueChips names the issues behind the ticket, such as Linear ENG-412,
 * with their status, each opening in its tracker.
 */
export function IssueChips({ issues }: { issues: TicketIssue[] }) {
  const { t } = useTranslation("trackers");
  if (issues.length === 0) {
    return null;
  }

  return (
    <ul aria-label={t("issues.label")} className="flex flex-wrap items-center gap-2">
      {issues.map((issue) => (
        <li key={issue.id}>
          <a
            href={issue.url}
            target="_blank"
            rel="noopener noreferrer"
            title={issue.title}
            className="flex items-center gap-1.5 rounded-lg bg-background-secondary-default px-2 py-1 text-body-2-medium text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <span className="capitalize">{issue.provider}</span>
            <span className="text-text-primary">{issue.key}</span>
            <Chip variant="caption" color={issueTint[issue.status_group] ?? "gray"}>
              {issue.status}
            </Chip>
            <RiExternalLinkLine aria-hidden className="size-3.5 text-foreground-icon-tertiary" />
          </a>
        </li>
      ))}
    </ul>
  );
}

/*
 * Summary is the AI's read of what customers report, how many and on
 * which plans. Nothing shows until the server sends one.
 */
export function Summary({ ticket }: { ticket: Ticket }) {
  const { t } = useTranslation("trackers");
  const summary = (ticket as Ticket & { summary?: TrackerSummary | null }).summary;
  if (!summary?.text) {
    return null;
  }

  return (
    <section
      aria-label={t("summary.title")}
      className="flex flex-col gap-2 rounded-2xl bg-background-secondary-default p-4"
    >
      <h2 className="flex items-center gap-1.5 text-body-2-medium text-text-secondary">
        <RiSparkling2Line aria-hidden className="size-4 text-foreground-icon-tertiary" />
        {t("summary.title")}
        {summary.generated_at ? (
          <span className="text-caption-1-regular text-text-tertiary">· {formatRelative(summary.generated_at)}</span>
        ) : null}
      </h2>
      <p className="whitespace-pre-line text-body-regular text-text-primary">{summary.text}</p>
    </section>
  );
}

/*
 * Reach is who a tracker or incident reaches, as quiet rows beside the thread.
 */
export function Reach({ impact }: { impact: Ticket["impact"] }) {
  const { t } = useTranslation("trackers");
  if (!impact) {
    return null;
  }

  return (
    <section aria-label={t("impact.customers")} className="flex flex-col gap-1 border-t border-separator-border pt-3">
      <h3 className="text-caption-1-medium text-text-tertiary">Reach</h3>
      <dl className="flex flex-col">
        <Row label={t("impact.customers")}>{formatNumber(impact.customers)}</Row>
        <Row label={t("impact.companies")}>{formatNumber(impact.companies)}</Row>
        <Row label={t("impact.revenue")}>{money(impact.revenue)}</Row>
        <Row label={t("impact.conversations")}>{formatNumber(impact.conversations)}</Row>
      </dl>
    </section>
  );
}
