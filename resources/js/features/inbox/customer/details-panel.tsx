/*
 * What a teammate needs beside the thread, as one quiet list. Its header
 * holds the moves they make on the case, in one Actions menu beside the
 * close button. Below come the case's own properties, its tags, what the
 * AI read off it, its ticket when it is one (or the trackers it joined), then who the
 * customer is, the team's notes on them, their company, their other
 * conversations and the feature requests the conversation was linked
 * to, all read from the conversation's context in one request. Each part
 * is an item of BoardUI's plain accordion, and a part with nothing to say
 * is left out. Nothing is drawn as a field: a value that can change or be copied
 * reads as text until the pointer is on it.
 */
import { RiCloseLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { Accordion } from "@/components/base/accordion/accordion";
import { IconButton } from "@/components/base/buttons/icon-button";
import { ConversationRequests } from "@/features/feedback/conversation-requests";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { TicketPanel } from "@/features/tickets/ticket-panel";
import { contextQuery } from "@/lib/api/customers";
import type { Conversation } from "@/lib/api/inbox";
import { callerQuery } from "@/lib/api/workspace";

import type { CaseControls } from "../case";
import { RatingSection } from "../rating";
import { CustomerSkeleton } from "../skeletons";
import { AIAttributesSection } from "./ai-attributes-section";
import { Actions, CaseFields, CaseTags, Refusal } from "./case-section";
import { CompanySection } from "./company-section";
import { ContactSection } from "./contact-section";
import { DeliverySection } from "./delivery-section";
import { DuplicateBanner } from "./duplicate-banner";
import { PastConversations } from "./past-conversations";
import { Nothing, Section } from "./rows";
import { SocialSection } from "./social-section";

/*
 * The sections the panel can show, all open at first. Each is named by
 * its title.
 */
const sections = [
  "Tags",
  "AI attributes",
  "Ticket",
  "Back-office",
  "Trackers",
  "Issues",
  "Customer",
  "Customer notes",
  "Instagram",
  "Facebook Messenger",
  "Company",
  "Past conversations",
  "Feature requests",
];

function Customer({ workspace, conversation }: { workspace: string; conversation: Conversation }) {
  const { data: context, isPending } = useQuery(contextQuery(workspace, conversation.id));

  if (isPending) {
    return <CustomerSkeleton />;
  }

  if (!context) {
    return <Nothing>The customer could not be loaded.</Nothing>;
  }

  return (
    <>
      <DuplicateBanner workspace={workspace} contact={context.contact.id} count={context.duplicates_count} />
      <ContactSection workspace={workspace} context={context} />
      <SocialSection workspace={workspace} conversation={conversation.id} channelType={conversation.channel.type} />
      <CompanySection context={context} />
      <PastConversations workspace={workspace} context={context} current={conversation.id} />
      <Section title="Feature requests">
        <ConversationRequests
          workspace={workspace}
          conversation={conversation.id}
          subject={conversation.subject ?? ""}
        />
      </Section>
      <DeliverySection workspace={workspace} contact={context.contact.id} />
    </>
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
  const { data: caller } = useQuery(callerQuery(workspace));

  return (
    <Panel label="Details" className="h-full w-full">
      <PageHeader title="Details" level={2}>
        <div className="flex items-center gap-1.5">
          <Actions conversation={conversation} caze={caze} me={caller?.member?.id ?? null} />
          <IconButton size="small" icon={RiCloseLine} aria-label="Close details" onClick={onClose} />
        </div>
      </PageHeader>
      <ScrollArea className="px-4 pb-6">
        <Accordion variant="plain" size="sm" allowsMultipleExpanded defaultExpandedKeys={sections} className="pt-1">
          <div className="flex flex-col gap-1">
            <Refusal error={caze.error} />
            <CaseFields workspace={workspace} conversation={conversation} caze={caze} />
          </div>
          <CaseTags workspace={workspace} conversation={conversation} caze={caze} />
          <RatingSection workspace={workspace} conversation={conversation.id} />
          <AIAttributesSection workspace={workspace} conversation={conversation.id} />
          <TicketPanel
            workspace={workspace}
            conversation={conversation.id}
            onConvert={onConvert ?? (() => undefined)}
          />
          <Customer workspace={workspace} conversation={conversation} />
        </Accordion>
      </ScrollArea>
    </Panel>
  );
}
