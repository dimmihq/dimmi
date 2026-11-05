import { RiMailLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { Empty, Glyph, Item, RowsSkeleton } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { type EmailAddress, inboundEmailsQuery } from "@/lib/api/email";

/*
 * Activity lists the last emails that reached the address.
 */
export function Activity({ address }: { address: EmailAddress }) {
  const me = useAccount();
  const { data } = useQuery(inboundEmailsQuery(me.workspace.id, address.id));

  if (!data) {
    return <RowsSkeleton rows={1} />;
  }

  if (data.length === 0) {
    return <Empty>No email has arrived yet.</Empty>;
  }

  return (
    <>
      {data.map((mail) => (
        <Item
          key={mail.id}
          mark={<Glyph icon={RiMailLine} />}
          title={mail.subject ?? "(no subject)"}
          detail={`${mail.from ?? "Unknown sender"} · ${mail.ignore_reason ? `ignored, ${mail.ignore_reason.replaceAll("_", " ")}` : mail.status}`}
        />
      ))}
    </>
  );
}
