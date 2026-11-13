/*
 * The incidents active right now, pinned beside the inbox so nobody
 * answers an outage one conversation at a time: each with its severity,
 * how many conversations and companies it has reached so far, and the
 * way to its page. The counts stay live, read again every thirty
 * seconds. Nothing shows while no incident is active.
 */
import { RiAlarmWarningLine } from "@remixicon/react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { activeIncidentsQuery } from "@/lib/api/ticket-trackers";
import { type Ticket, ticketQuery } from "@/lib/api/tickets";
import { formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

function Pin({ incident, companies }: { incident: Ticket; companies: number | undefined }) {
  const { t } = useTranslation("trackers");
  const severity = String(incident.fields.severity ?? "");
  const counts = [
    t("pins.conversations", {
      count: incident.linked_conversations,
      formatted: formatNumber(incident.linked_conversations),
    }),
    companies === undefined ? null : t("pins.companies", { count: companies, formatted: formatNumber(companies) }),
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <li>
      <Link
        to="/d/inbox/$view/$conversation"
        params={{ view: "incidents", conversation: incident.id }}
        title={[
          incident.reference,
          incident.title,
          severity ? t("pins.severity", { level: severity }) : "",
          incident.state.name,
        ]
          .filter(Boolean)
          .join(" · ")}
        className="group flex items-start gap-2 rounded-lg px-2 py-1.5 outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiAlarmWarningLine
          aria-hidden
          className={cx(
            "mt-0.5 size-4 shrink-0",
            severity === "minor" ? "text-foreground-icon-secondary" : "text-text-error-primary",
          )}
        />
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-body-2-medium text-text-primary">{incident.title}</span>
          <span className="truncate text-caption-1-regular text-text-tertiary tabular-nums">{counts}</span>
        </span>
      </Link>
    </li>
  );
}

/*
 * IncidentPins lists the active incidents, at most five, newest first.
 */
export function IncidentPins({ workspace, className }: { workspace: string; className?: string }) {
  const { t } = useTranslation("trackers");
  const { data: incidents = [] } = useQuery(activeIncidentsQuery(workspace));
  const shown = incidents.slice(0, 5);
  const details = useQueries({
    queries: shown.map((i) => ({ ...ticketQuery(workspace, i.id), refetchInterval: 30_000 })),
  });
  if (shown.length === 0) {
    return null;
  }

  return (
    <section aria-label={t("pins.label")} className={cx("flex flex-col gap-0.5", className)}>
      <h2 className="px-2 pb-0.5 text-caption-1-medium text-text-tertiary">{t("pins.label")}</h2>
      <ul className="flex flex-col">
        {shown.map((incident, i) => (
          <Pin
            key={incident.id}
            incident={details[i]?.data ?? incident}
            companies={details[i]?.data?.impact?.companies}
          />
        ))}
      </ul>
    </section>
  );
}
