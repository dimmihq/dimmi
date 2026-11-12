/*
 * How a message to everyone went: a bar filling as deliveries go out,
 * counts of sent, held back and failed, and every customer with what
 * happened to their copy and why. The list reads itself again every two
 * seconds while the message is still going out. The tracker page lists
 * the messages sent before, each opening on its results.
 */
import { RiMailSendLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { ProgressBar } from "@/components/base/progress/progress";
import { Glyph, Item, Section } from "@/features/settings/layout";
import { rowLink } from "@/features/settings/row-link";
import {
  type DeliveryStatus,
  type TicketUpdate,
  ticketUpdatesQuery,
  type UpdateStatus,
} from "@/lib/api/ticket-trackers";
import { formatDateTime, formatNumber } from "@/lib/i18n/format";

import { heldReasonLabel } from "./tracker-helpers";

const deliveryTint: Record<DeliveryStatus, ChipColor> = {
  pending: "gray",
  sent: "lime",
  held: "yellow",
  failed: "rose",
};

const updateTint: Record<UpdateStatus, ChipColor> = {
  queued: "gray",
  sending: "blue",
  sent: "lime",
  partial: "yellow",
  failed: "rose",
};

function going(update: TicketUpdate): boolean {
  return update.status === "queued" || update.status === "sending";
}

/*
 * DeliveryResults shows one message's deliveries, or the newest message
 * when updateId is not known.
 */
export function DeliveryResults({
  workspace,
  ticket,
  updateId,
}: {
  workspace: string;
  ticket: string;
  updateId: string | null;
}) {
  const { t } = useTranslation("trackers");
  const { data: updates = [] } = useQuery(ticketUpdatesQuery(workspace, ticket));
  const update = updates.find((u) => u.id === updateId) ?? (updateId ? undefined : updates[0]);

  if (!update) {
    return <ProgressBar isIndeterminate aria-label={t("results.loading")} label={t("results.loading")} />;
  }
  const done = update.delivered + update.held + update.failed;
  const percent = update.recipients === 0 ? 100 : Math.round((done / update.recipients) * 100);

  return (
    <div className="flex flex-col gap-4">
      <ProgressBar
        value={going(update) ? percent : 100}
        aria-label={t("results.progress")}
        label={t(`results.status.${update.status}`)}
        description={t("results.counts", {
          sent: formatNumber(update.delivered),
          held: formatNumber(update.held),
          failed: formatNumber(update.failed),
          total: formatNumber(update.recipients),
        })}
      />
      <ul
        aria-label={t("results.deliveries")}
        className="flex flex-col rounded-2xl bg-background-secondary-default px-4"
      >
        {update.deliveries.map((d) => (
          <li
            key={d.conversation_id}
            className="flex items-center justify-between gap-3 border-separator-border border-t py-2.5 first:border-t-0"
          >
            <span className="flex min-w-0 flex-col">
              <span className="truncate text-body-2-medium text-text-primary">
                {d.contact_name || d.contact_email || `#${d.conversation_number}`}
              </span>
              <Link
                to="/d/inbox/$view/$conversation"
                params={{ view: "all", conversation: d.conversation_id }}
                className="truncate text-caption-1-regular text-text-tertiary hover:text-text-secondary hover:underline"
              >
                #{d.conversation_number}
              </Link>
            </span>
            <span className="flex shrink-0 flex-col items-end gap-0.5">
              <Chip variant="caption" color={deliveryTint[d.status]}>
                {t(`results.delivery.${d.status}`)}
              </Chip>
              {d.reason ? (
                <span className="max-w-60 truncate text-caption-1-regular text-text-tertiary" title={d.reason}>
                  {heldReasonLabel(d.reason)}
                </span>
              ) : null}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/*
 * SentMessages lists the messages a tracker sent, newest first, each
 * opening on its results. It is left out until the first one is sent.
 */
export function SentMessages({
  workspace,
  ticket,
  onOpen,
}: {
  workspace: string;
  ticket: string;
  onOpen: (update: TicketUpdate) => void;
}) {
  const { t } = useTranslation("trackers");
  const { data: updates = [] } = useQuery(ticketUpdatesQuery(workspace, ticket));
  const [all, setAll] = useState(false);
  if (updates.length === 0) {
    return null;
  }
  const shown = all ? updates : updates.slice(0, 3);

  return (
    <Section
      title={t("results.sentTitle")}
      action={
        updates.length > 3 ? (
          <button
            type="button"
            className="rounded-md text-body-2-medium text-text-secondary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            onClick={() => setAll((v) => !v)}
          >
            {all ? t("results.fewer") : t("results.all", { count: updates.length })}
          </button>
        ) : undefined
      }
    >
      {shown.map((u) => (
        <Item
          key={u.id}
          mark={<Glyph icon={RiMailSendLine} />}
          title={
            <button type="button" className={`${rowLink} text-start`} onClick={() => onOpen(u)}>
              {u.body.split("\n")[0]}
            </button>
          }
          detail={`${formatDateTime(u.created_at)} · ${t("results.counts", {
            sent: formatNumber(u.delivered),
            held: formatNumber(u.held),
            failed: formatNumber(u.failed),
            total: formatNumber(u.recipients),
          })}`}
        >
          <Chip variant="caption" color={updateTint[u.status]} className="me-3">
            {t(`results.status.${u.status}`)}
          </Chip>
        </Item>
      ))}
    </Section>
  );
}
