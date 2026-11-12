/*
 * Merging the open conversation into another of the same customer's, or
 * of someone at the same company. The dialog lists the customer's other
 * conversations, newest first, then those of others at their company,
 * as cards to pick from; confirming moves every message of this one into the one
 * picked, where they read in the order they happened. This one is
 * closed and moved to the trash, and the screen follows to the merged
 * conversation.
 */
import { RiMergeCellsHorizontal } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { AlertDialog } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import { companyConversationsQuery, contactConversationsQuery, contactQuery } from "@/lib/api/customers";
import { type Conversation, mergeConversations } from "@/lib/api/inbox";
import { useMutationId } from "@/lib/realtime/context";

import { statusLabel } from "./case";
import { ago, customerName, subjectOf } from "./conversations";

/*
 * useCandidates returns the conversations the open one may merge into:
 * the customer's own, and those of others at the customer's company,
 * each list without the open conversation.
 */
function useCandidates(workspace: string, conversation: Conversation, enabled: boolean) {
  const contactId = conversation.contact.id;
  const own = useQuery({ ...contactConversationsQuery(workspace, contactId, 25), enabled });
  const { data: contact } = useQuery({ ...contactQuery(workspace, contactId), enabled });
  const company = contact?.company ?? null;
  const { data: atCompany = [] } = useQuery({
    ...companyConversationsQuery(workspace, company?.id ?? ""),
    enabled: enabled && company !== null,
  });
  const others = (c: Conversation) => c.id !== conversation.id;

  return {
    pending: own.isPending,
    company,
    theirs: (own.data ?? []).filter(others),
    colleagues: atCompany.filter((c) => others(c) && c.contact.id !== contactId),
  };
}

function Heading({ children }: { children: string }) {
  return <h3 className="pt-1 text-caption-1-medium text-text-tertiary">{children}</h3>;
}

export function MergeDialog({
  view,
  workspace,
  conversation,
  isOpen,
  onOpenChange,
}: {
  view: string;
  workspace: string;
  conversation: Conversation;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const mutationId = useMutationId();
  const [target, setTarget] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const { pending, company, theirs, colleagues } = useCandidates(workspace, conversation, isOpen);
  const card = (c: Conversation, who?: string) => (
    <RadioCard
      key={c.id}
      value={c.id}
      title={subjectOf(c)}
      description={[who, `#${c.number}`, statusLabel[c.status], ago(c.last_message_at ?? c.created_at)]
        .filter(Boolean)
        .join(" · ")}
    />
  );

  const merge = async () => {
    if (!target) {
      return;
    }
    setFailure(null);
    try {
      await mergeConversations({ workspace, mutation: mutationId() }, target, [conversation.id]);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : "The conversations could not be merged.");
      throw error;
    }
    toast.success(`Merged #${conversation.number} into the conversation`);
    void client.invalidateQueries({ queryKey: ["workspace", workspace] });
    void navigate({ to: "/d/inbox/$view/$conversation", params: { view, conversation: target } });
  };

  return (
    <AlertDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        onOpenChange(open);
        if (!open) {
          setTarget(null);
          setFailure(null);
        }
      }}
      isDismissable
      variant="default"
      icon={RiMergeCellsHorizontal}
      size="md"
      title={`Merge #${conversation.number} into…`}
      description="Its messages move into the conversation you pick, in the order they happened. This one is closed and moved to the trash."
      confirmLabel="Merge"
      pendingLabel="Merging…"
      isConfirmDisabled={!target}
      onConfirm={merge}
    >
      <div className="flex max-h-80 flex-col gap-2 overflow-y-auto">
        {theirs.length + colleagues.length === 0 ? (
          <p className="text-body-2-regular text-text-secondary">
            {pending ? "Loading their conversations…" : "This customer has no other conversation to merge into."}
          </p>
        ) : (
          <RadioGroup aria-label="Conversation to merge into" value={target} onChange={setTarget}>
            {theirs.length > 0 ? <Heading>This customer</Heading> : null}
            {theirs.map((c) => card(c))}
            {colleagues.length > 0 && company ? <Heading>{`Others at ${company.name}`}</Heading> : null}
            {colleagues.map((c) => card(c, customerName(c.contact)))}
          </RadioGroup>
        )}
        {failure ? <p className="text-body-2-regular text-text-error-primary">{failure}</p> : null}
      </div>
    </AlertDialog>
  );
}
