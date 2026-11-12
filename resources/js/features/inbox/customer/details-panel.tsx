/*
 * What a teammate needs beside the thread. Its header holds the moves
 * they make on the case, in one Actions menu beside the close button. Below, in BoardUI's divided accordion straight on the panel,
 * who the customer is, their company and their other conversations, all
 * read from the conversation's context in one request, its ticket when
 * it is one (or the trackers it joined), and the feature
 * requests the conversation was linked to. A section only
 * shows when it has something to say.
 */
import { RiBuilding2Line, RiChatHistoryLine, RiCloseLine, RiLightbulbLine, RiUser3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { Accordion, AccordionItem } from "@/components/base/accordion/accordion";
import { IconButton } from "@/components/base/buttons/icon-button";
import { ConversationRequests } from "@/features/feedback/conversation-requests";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { TicketPanel } from "@/features/tickets/ticket-panel";
import { contextQuery } from "@/lib/api/customers";
import type { Conversation } from "@/lib/api/inbox";
import { callerQuery, membersQuery } from "@/lib/api/workspace";

import type { CaseControls } from "../case";
import { RatingSection } from "../rating";
import { CustomerSkeleton } from "../skeletons";
import { Actions, CaseFields, CaseTags, Refusal } from "./case-section";
import { CompanySection } from "./company-section";
import { ContactSection } from "./contact-section";
import { DeliverySection } from "./delivery-section";
import { DuplicateBanner } from "./duplicate-banner";
import { PastConversations } from "./past-conversations";
import { Nothing } from "./rows";

/*
 * The sections' keys, which the accordion uses to remember which are open.
 */
const section = { customer: "customer", company: "company", past: "past", requests: "requests" } as const;

function Customer({ workspace, conversation }: { workspace: string; conversation: Conversation }) {
  const { data: context, isPending } = useQuery(contextQuery(workspace, conversation.id));

  if (isPending) {
    return <CustomerSkeleton />;
  }

  if (!context) {
    return <Nothing>The customer could not be loaded.</Nothing>;
  }

  return (
    <div className="flex flex-col gap-3">
      <DuplicateBanner workspace={workspace} contact={context.contact.id} count={context.duplicates_count} />
      <Accordion
        variant="divided"
        size="sm"
        allowsMultipleExpanded
        defaultExpandedKeys={[section.customer, section.company, section.past, section.requests]}
        className="border-t border-separator-border"
      >
        <AccordionItem id={section.customer} title="Customer" icon={RiUser3Line}>
          <ContactSection workspace={workspace} context={context} />
        </AccordionItem>
        <AccordionItem id={section.company} title="Company" icon={RiBuilding2Line}>
          <CompanySection context={context} />
        </AccordionItem>
        <AccordionItem id={section.past} title="Past conversations" icon={RiChatHistoryLine}>
          <PastConversations workspace={workspace} context={context} current={conversation.id} />
        </AccordionItem>
        <AccordionItem id={section.requests} title="Feature requests" icon={RiLightbulbLine}>
          <ConversationRequests
            workspace={workspace}
            conversation={conversation.id}
            subject={conversation.subject ?? ""}
          />
        </AccordionItem>
      </Accordion>
      <DeliverySection workspace={workspace} contact={context.contact.id} />
    </div>
  );
}

export function DetailsPanel({
  workspace,
  conversation,
  caze,
  onClose,
  onConvert,
}: {
  workspace: string;
  conversation: Conversation;
  caze: CaseControls;
  onClose: () => void;
  onConvert?: () => void;
}) {
  const { data: members = [] } = useQuery(membersQuery(workspace));
  const { data: caller } = useQuery(callerQuery(workspace));
  const people = members
    .filter((member) => member.status === "active")
    .map((member) => ({ id: member.id, name: member.user.name }));

  return (
    <Panel label="Details" className="h-full w-full">
      <PageHeader title="Details" level={2}>
        <div className="flex items-center gap-1.5">
          <Actions conversation={conversation} caze={caze} me={caller?.member?.id ?? null} />
          <IconButton size="small" icon={RiCloseLine} aria-label="Close details" onClick={onClose} />
        </div>
      </PageHeader>
      <ScrollArea className="px-4 pb-4">
        <div className="flex flex-col gap-2 pt-1 pb-2">
          <Refusal error={caze.error} />
          <CaseFields conversation={conversation} caze={caze} people={people} />
          <CaseTags workspace={workspace} conversation={conversation} caze={caze} />
          <RatingSection workspace={workspace} conversation={conversation.id} />
        </div>
        <TicketPanel workspace={workspace} conversation={conversation.id} onConvert={onConvert ?? (() => undefined)} />
        <Customer workspace={workspace} conversation={conversation} />
      </ScrollArea>
    </Panel>
  );
}
