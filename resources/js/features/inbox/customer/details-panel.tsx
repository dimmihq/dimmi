/*
 * What a teammate needs beside the thread. Its header holds the moves
 * they make on the case, in one Actions menu beside the close button. Below, in BoardUI's divided accordion straight on the panel,
 * who the customer is, their company and their other conversations, all
 * read from the conversation's context in one request. A section only
 * shows when it has something to say.
 */
import { RiBuilding2Line, RiChatHistoryLine, RiCloseLine, RiUser3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { Accordion, AccordionItem } from "@/components/base/accordion/accordion";
import { IconButton } from "@/components/base/buttons/icon-button";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { contextQuery } from "@/lib/api/customers";
import type { Conversation } from "@/lib/api/inbox";
import { callerQuery, membersQuery } from "@/lib/api/workspace";

import type { CaseControls } from "../case";
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
const section = { customer: "customer", company: "company", past: "past" } as const;

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
        defaultExpandedKeys={[section.customer, section.company, section.past]}
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
}: {
  workspace: string;
  conversation: Conversation;
  caze: CaseControls;
  onClose: () => void;
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
        </div>
        <Customer workspace={workspace} conversation={conversation} />
      </ScrollArea>
    </Panel>
  );
}
