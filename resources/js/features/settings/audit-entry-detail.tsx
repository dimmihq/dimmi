/*
 * One audit entry in full, opened from its row: who acted, through what
 * and from which address and browser, what it was about, and every
 * change it recorded, as the server stored it.
 */
import { Drawer, DrawerBody, DrawerHeader } from "@/components/base/drawer/drawer";
import type { AuditEntry } from "@/lib/api/audit";
import { cx } from "@/utils/cx";

import { actorDetail, auditEventLabel, formatTime } from "./log-labels";

function Fact({ label, value, code = false }: { label: string; value: string | null | undefined; code?: boolean }) {
  return (
    <div className="flex flex-col gap-0.5 border-b border-separator-border py-2.5 last:border-b-0">
      <dt className="text-caption-1-medium text-text-tertiary">{label}</dt>
      <dd
        dir={code ? "ltr" : undefined}
        className={cx("text-body-2-regular text-text-primary", code && "break-all font-mono")}
      >
        {value || "—"}
      </dd>
    </div>
  );
}

export function AuditEntryDetail({
  entry,
  zone,
  onClose,
}: {
  entry: AuditEntry | null;
  zone?: string;
  onClose: () => void;
}) {
  return (
    <Drawer isOpen={entry !== null} onOpenChange={(open) => !open && onClose()} size="lg">
      {entry ? (
        <>
          <DrawerHeader title={auditEventLabel(entry.event)} description={formatTime(entry.created_at, zone)} />
          <DrawerBody>
            <dl aria-label="Entry details" className="flex flex-col rounded-2xl bg-background-secondary-default px-3">
              <Fact label="Event" value={entry.event} code />
              <Fact label="Actor" value={entry.actor.label} />
              <Fact label="Channel" value={actorDetail({ actor: entry.actor, ip: null })} />
              <Fact label="IP address" value={entry.ip} code />
              <Fact label="Browser" value={entry.user_agent} code />
              <Fact label="Target" value={entry.subject?.label} />
              <Fact label="Request" value={entry.request_id} code />
            </dl>
            <section aria-label="Changes" className="mt-4 flex flex-col gap-1.5">
              <h3 className="px-1 text-caption-1-medium text-text-tertiary">Changes</h3>
              <pre
                dir="ltr"
                className="overflow-x-auto rounded-2xl bg-background-secondary-default p-3 font-mono text-caption-1-regular text-text-secondary"
              >
                {JSON.stringify(entry.changes, null, 2)}
              </pre>
            </section>
          </DrawerBody>
        </>
      ) : null}
    </Drawer>
  );
}
