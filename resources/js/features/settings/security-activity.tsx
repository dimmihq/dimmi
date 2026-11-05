/*
 * Recent security activity on the person's own account, from the
 * server's security log: sign-ins, failed attempts, and changes to the
 * password, two-factor authentication and sessions, newest first, each
 * with the device and address it came from.
 */
import { RiShieldKeyholeLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { securityEventsQuery } from "@/lib/api/audit";

import { Empty, Glyph, Item, Section } from "./layout";
import { formatTime, securityEventLabel } from "./log-labels";
import { useAccount } from "./use-account";

export function RecentSecurityActivity() {
  const me = useAccount();
  const { data, error, isPending } = useQuery(securityEventsQuery());
  const events = data?.items ?? [];

  return (
    <Section title="Recent security activity" description="Sign-ins and changes to your account.">
      {error ? <Empty>{error.message}</Empty> : null}
      {!(error || isPending) && events.length === 0 ? <Empty>Nothing yet</Empty> : null}
      {events.map((event) => (
        <Item
          key={event.id}
          mark={<Glyph icon={RiShieldKeyholeLine} />}
          title={securityEventLabel(event.type)}
          detail={[formatTime(event.created_at, me.displayZone), event.device_label, event.ip]
            .filter(Boolean)
            .join(" · ")}
        />
      ))}
    </Section>
  );
}
