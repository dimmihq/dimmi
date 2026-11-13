/*
 * The ticket moves in a conversation's header, behind one button that
 * shows the ticket's reference once it is one: convert it (also T),
 * escalate it to another team, and add it to a tracker. Each opens its
 * own dialog; the conversation view
 * holds which one is open, so the keyboard can open them too.
 */
import { RiArrowDownSLine, RiBriefcase4Line, RiCoupon3Line, RiRouteLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import {
  Dropdown,
  DropdownGroup,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { Kbd } from "@/components/base/kbd/kbd";
import { conversationTicketsQuery } from "@/lib/api/tickets";
import { cx } from "@/utils/cx";

import { ConvertDialog } from "./convert-dialog";
import { EscalateDialog } from "./escalate-dialog";
import { TrackerDialog } from "./tracker-dialog";

import type { TicketDialog, TicketDialogs } from "./use-ticket-dialogs";

const trigger = cx(
  "inline-flex h-8 shrink-0 items-center gap-1 rounded-2lg px-2",
  "border border-border-button-default bg-background-primary-default text-body-2-medium text-text-primary shadow-xs",
  "transition-colors duration-150 hover:border-border-button-hover hover:bg-background-primary-hover",
  "outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring",
);

export function TicketMenu({
  workspace,
  conversation,
  subject,
  dialogs,
}: {
  workspace: string;
  conversation: string;
  subject: string;
  dialogs: TicketDialogs;
}) {
  const { data } = useQuery(conversationTicketsQuery(workspace, conversation));
  const [menu, setMenu] = useState(false);
  const own = data?.ticket ?? null;
  const items: { kind: TicketDialog; label: string; icon: typeof RiCoupon3Line; keys?: string }[] = [
    ...(own ? [] : [{ kind: "convert" as const, label: "Convert to ticket", icon: RiCoupon3Line, keys: "T" }]),
    ...(own?.kind === "customer" ? [{ kind: "escalate" as const, label: "Escalate", icon: RiBriefcase4Line }] : []),
    { kind: "tracker", label: "Add to tracker", icon: RiRouteLine },
  ];

  return (
    <>
      <Dropdown isOpen={menu} onOpenChange={setMenu}>
        <DropdownTrigger aria-label="Ticket" className={trigger}>
          <RiCoupon3Line aria-hidden className="size-4 text-foreground-icon-secondary" />
          <span className="max-lg:sr-only @max-3xl/thread:sr-only">{own ? own.reference : "Ticket"}</span>
          <RiArrowDownSLine aria-hidden className="size-4 text-foreground-icon-tertiary @max-3xl/thread:hidden" />
        </DropdownTrigger>
        <DropdownPopover aria-label="Ticket" placement="bottom end" className="w-[220px] p-2">
          <DropdownGroup>
            {items.map((item) => (
              <DropdownItem
                key={item.kind}
                onSelect={() => {
                  setMenu(false);
                  dialogs.show(item.kind);
                }}
                className="px-2 py-1.5"
              >
                <item.icon aria-hidden className="size-[18px] shrink-0 text-foreground-icon-secondary" />
                <span className="flex-1 truncate text-body-medium text-text-primary">{item.label}</span>
                {item.keys ? <Kbd>{item.keys}</Kbd> : null}
              </DropdownItem>
            ))}
          </DropdownGroup>
        </DropdownPopover>
      </Dropdown>
      {dialogs.open === "convert" && !own ? (
        <ConvertDialog workspace={workspace} conversation={conversation} subject={subject} onClose={dialogs.close} />
      ) : null}
      {dialogs.open === "escalate" && own ? (
        <EscalateDialog workspace={workspace} ticket={own} onClose={dialogs.close} />
      ) : null}
      {dialogs.open === "tracker" ? (
        <TrackerDialog workspace={workspace} conversation={conversation} subject={subject} onClose={dialogs.close} />
      ) : null}
    </>
  );
}
