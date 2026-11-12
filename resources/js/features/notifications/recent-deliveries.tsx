/*
 * Recent deliveries: how each notification reached the member on each
 * channel, newest first, so they can see why they did or did not hear
 * about something: sent, failed and why, or held back and why, such as
 * quiet hours or a channel turned off. Someone who manages members can
 * read another member's, to answer the same question for them.
 */
import { RiMailLine, RiNotification3Line, RiSlackLine, RiSmartphoneLine } from "@remixicon/react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Badge } from "@/components/base/badges/badge";
import {
  type DeliveryFilter,
  type DeliveryStatus,
  notificationDeliveriesQuery,
} from "@/lib/api/notification-deliveries";
import type { NotificationChannel } from "@/lib/api/notifications";
import { membersQuery } from "@/lib/api/workspace";
import { formatRelative } from "@/lib/i18n/format";

import { Action, Choice } from "../settings/controls";
import { Empty, Glyph, Item, RowsSkeleton, Section } from "../settings/layout";
import { useCanManageMembers } from "../settings/teams";
import { deliveryOutcome, deliveryStatusLabels } from "./delivery-format";

const channels: Record<NotificationChannel, { label: string; icon: typeof RiMailLine }> = {
  in_app: { label: "In app", icon: RiNotification3Line },
  email: { label: "Email", icon: RiMailLine },
  push: { label: "Push", icon: RiSmartphoneLine },
  slack: { label: "Slack", icon: RiSlackLine },
};

const statusChoices = ["All", "Sent", "Skipped", "Failed"];

export function RecentDeliveries({ workspace }: { workspace: string }) {
  const canManage = useCanManageMembers(workspace);
  const [filter, setFilter] = useState<DeliveryFilter>({});
  const query = useInfiniteQuery({ ...notificationDeliveriesQuery(workspace, filter), enabled: workspace !== "" });
  const lines = query.data?.pages.flatMap((p) => p.data) ?? [];

  return (
    <Section
      title="Recent deliveries"
      description="How each notification reached you on each channel, and why one did not, for the last 30 days."
      action={
        <div className="flex flex-wrap gap-2">
          {canManage ? (
            <MemberChoice workspace={workspace} onChange={(member) => setFilter({ ...filter, member })} />
          ) : null}
          <Choice
            label="Status"
            options={statusChoices}
            value="All"
            onChange={(pick) =>
              setFilter({ ...filter, status: pick === "All" ? undefined : (pick.toLowerCase() as DeliveryStatus) })
            }
          />
        </div>
      }
    >
      {query.isPending ? <RowsSkeleton rows={3} /> : null}
      {!query.isPending && lines.length === 0 ? <Empty>No deliveries yet.</Empty> : null}
      {lines.map((d) => (
        <Item
          key={d.id}
          mark={<Glyph icon={channels[d.channel].icon} />}
          title={d.title || d.trigger}
          detail={`${channels[d.channel].label} · ${deliveryOutcome(d)} · ${formatRelative(d.created_at)}`}
        >
          <Badge className="shrink-0">{deliveryStatusLabels[d.status]}</Badge>
        </Item>
      ))}
      {query.hasNextPage ? (
        <div className="py-2.5">
          <Action size="xs" loading={query.isFetchingNextPage} onPress={() => void query.fetchNextPage()}>
            Show more
          </Action>
        </div>
      ) : null}
    </Section>
  );
}

/*
 * MemberChoice picks whose deliveries to read: the caller's own, or one
 * active member's.
 */
function MemberChoice({ workspace, onChange }: { workspace: string; onChange: (member?: string) => void }) {
  const { data: members = [] } = useQuery(membersQuery(workspace));
  const active = members.filter((m) => m.status === "active");
  const labelOf = (m: (typeof active)[number]) => `${m.user.name} (${m.user.email})`;

  return (
    <Choice
      label="Member"
      options={["You", ...active.map(labelOf)]}
      value="You"
      onChange={(pick) => onChange(active.find((m) => labelOf(m) === pick)?.id)}
    />
  );
}
