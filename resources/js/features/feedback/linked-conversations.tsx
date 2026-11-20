/*
 * The conversations a request came up in, newest link first: who asked,
 * from which company, and the conversation itself, which opens in the
 * inbox. Teammates link them here, by searching for the conversation,
 * or from the conversation's details.
 */
import { RiAddLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { SearchComboBox } from "@/components/application/search-combobox/search-combobox";
import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { customerName, subjectOf } from "@/features/inbox/conversations";
import { feedbackKeys, type LinkedConversation, linkConversation, linkedConversationsQuery } from "@/lib/api/feedback";
import { type Conversation, conversationsQuery } from "@/lib/api/inbox";

function who(row: LinkedConversation): string {
  return row.name ?? row.email ?? "Customer";
}

function day(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}

/*
 * LinkConversation finds a conversation by its words or number and
 * links it to the request, which counts its customer's vote.
 */
function LinkConversation({ workspace, postId, onClose }: { workspace: string; postId: string; onClose: () => void }) {
  const client = useQueryClient();
  const [words, setWords] = useState("");
  const [picked, setPicked] = useState<Conversation | null>(null);
  const found = useQuery({
    ...conversationsQuery(workspace, { view: "all", status: "any", search: words, limit: 20 }),
    enabled: workspace !== "" && words !== "",
  });
  const link = useMutation({
    mutationFn: (conversation: string) => linkConversation(workspace, conversation, { post_id: postId }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: feedbackKeys.post(workspace, postId) });
      toast.success("Conversation linked.");
      onClose();
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title="Link a conversation"
        description="Its customer's vote is counted, and they hear when the request ships."
      />
      <ModalBody className="pb-2">
        <SearchComboBox
          label="Conversation"
          placeholder="Search by words, customer or number"
          autoFocus
          items={found.data ?? []}
          searching={found.isFetching}
          keyOf={(c: Conversation) => c.id}
          textOf={(c) => `#${c.number} ${subjectOf(c)}`}
          onSearch={setWords}
          onPick={setPicked}
          empty="No conversation matches."
        >
          {(c) => (
            <>
              <span className="shrink-0 text-body-2-medium text-text-tertiary tabular-nums">#{c.number}</span>
              <span className="min-w-0 flex-1 truncate text-text-primary">{subjectOf(c)}</span>
              <span className="max-w-32 shrink-0 truncate text-body-2-regular text-text-tertiary">
                {customerName(c.contact)}
              </span>
            </>
          )}
        </SearchComboBox>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button
          loading={link.isPending}
          disabled={picked === null}
          onClick={() => (picked ? link.mutate(picked.id) : undefined)}
        >
          Link
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * LinkedConversations lists the conversations linked to the request,
 * under a way to link another.
 */
export function LinkedConversations({ workspace, postId }: { workspace: string; postId: string }) {
  const { data, isPending } = useQuery(linkedConversationsQuery(workspace, postId));
  const [linking, setLinking] = useState(false);
  const rows = data?.items ?? [];

  if (isPending) {
    return null;
  }

  return (
    <div className="flex flex-col">
      <div className="flex min-h-8 items-center">
        <LinkButton size="xs" variant="secondary" leadingIcon={RiAddLine} onClick={() => setLinking(true)}>
          Link a conversation
        </LinkButton>
      </div>
      {rows.length === 0 ? (
        <p className="text-body-2-regular text-text-tertiary">No conversations linked yet.</p>
      ) : null}
      <ul aria-label="Linked conversations" className="flex flex-col divide-y divide-separator-border">
        {rows.map((row) => (
          <li key={row.conversation_id} className="flex items-center gap-3 py-2.5">
            <span className="min-w-0 flex-1">
              <span className="block truncate text-body-2-medium text-text-primary">
                {who(row)}
                {row.company ? <span className="text-text-tertiary"> · {row.company}</span> : null}
              </span>
              <Link
                to="/d/inbox/$view/$conversation"
                params={{ view: "all", conversation: row.conversation_id }}
                className="block truncate text-body-2-regular text-text-secondary outline-none hover:text-text-primary hover:underline focus-visible:underline"
              >
                #{row.conversation_number} {row.subject ?? "Conversation"}
              </Link>
            </span>
            <time dateTime={row.linked_at} className="shrink-0 text-caption-1-medium text-text-tertiary">
              {day(row.linked_at)}
            </time>
          </li>
        ))}
      </ul>
      {linking ? <LinkConversation workspace={workspace} postId={postId} onClose={() => setLinking(false)} /> : null}
    </div>
  );
}
