/*
 * Joining a conversation to the work many customers share. Add to
 * tracker searches the open trackers by title or reference and links
 * the conversation to the one picked, or starts a new tracker with it.
 */
import { RiSearchLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import { Resize, Swap } from "@/features/inbox/motion";
import {
  addTicketLink,
  createTicket,
  type TicketKind,
  ticketKeys,
  ticketsQuery,
  ticketTypesQuery,
} from "@/lib/api/tickets";

import { typeIcon } from "./labels";

/*
 * useNewLinked starts a tracker linked to the conversation.
 */
function useNewLinked(workspace: string, conversation: string, onDone: () => void) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (input: { type: string; title: string; description?: string }) =>
      createTicket(workspace, {
        type_id: input.type,
        title: input.title,
        description: input.description,
        link_to: [{ kind: "tracker", conversation_id: conversation }],
      }),
    onSuccess: async (made) => {
      await client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
      toast.success(`${made.reference} started`);
      onDone();
    },
    onError: (error) => toast.error((error as Error).message),
  });
}

function useTypesOf(workspace: string, kind: TicketKind) {
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));

  return types.filter((t) => t.kind === kind && !t.archived_at);
}

export function TrackerDialog({
  workspace,
  conversation,
  subject,
  onClose,
}: {
  workspace: string;
  conversation: string;
  subject: string;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const [mode, setMode] = useState<"pick" | "new">("pick");
  const [search, setSearch] = useState("");
  const q = useDeferredValue(search.trim());
  const { data, isPending } = useQuery(ticketsQuery(workspace, { kind: "tracker", active: true, q, limit: 20 }));
  const trackers = data?.items ?? [];
  const [picked, setPicked] = useState<string | null>(null);
  const [title, setTitle] = useState(subject);
  const types = useTypesOf(workspace, "tracker");
  const start = useNewLinked(workspace, conversation, onClose);
  const link = useMutation({
    mutationFn: (tracker: string) =>
      addTicketLink(workspace, tracker, { kind: "tracker", conversation_id: conversation }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
      toast.success("Added to the tracker");
      onClose();
    },
    onError: (error) => toast.error((error as Error).message),
  });

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={mode === "pick" ? "Add to tracker" : "New tracker"}
        description={
          mode === "pick"
            ? "Link this conversation to the problem it reports. One update then reaches everyone."
            : "Start a tracker for a problem many customers report, with this conversation on it."
        }
      />
      <ModalBody className="pb-4">
        <Resize>
          <Swap view={mode} forward={mode === "new"}>
            {mode === "pick" ? (
              <div className="flex flex-col gap-3">
                <Input
                  aria-label="Search trackers"
                  placeholder="Search trackers"
                  leadingIcon={RiSearchLine}
                  value={search}
                  onChange={setSearch}
                  autoFocus
                />
                {!isPending && trackers.length === 0 ? (
                  <p className="text-body-2-regular text-text-tertiary">No open tracker matches.</p>
                ) : (
                  <RadioGroup
                    aria-label="Trackers"
                    value={picked}
                    onChange={setPicked}
                    className="flex max-h-80 flex-col gap-2 overflow-y-auto"
                  >
                    {trackers.map((t) => (
                      <RadioCard
                        key={t.id}
                        value={t.id}
                        title={`${t.reference} · ${t.title}`}
                        description={`${t.state.name} · ${t.linked_conversations} linked`}
                        icon={typeIcon(t.type.icon, t.kind)}
                      />
                    ))}
                  </RadioGroup>
                )}
              </div>
            ) : (
              <Input
                label="Title"
                size="small"
                value={title}
                onChange={setTitle}
                maxLength={200}
                isRequired
                autoFocus
              />
            )}
          </Swap>
        </Resize>
      </ModalBody>
      <ModalFooter>
        {mode === "pick" ? (
          <>
            <Button variant="secondary" size="small" disabled={types.length === 0} onClick={() => setMode("new")}>
              New tracker
            </Button>
            <Button
              size="small"
              loading={link.isPending}
              disabled={picked === null}
              onClick={() => picked && link.mutate(picked)}
            >
              Add to tracker
            </Button>
          </>
        ) : (
          <>
            <Button variant="secondary" size="small" onClick={() => setMode("pick")}>
              Back
            </Button>
            <Button
              size="small"
              loading={start.isPending}
              disabled={title.trim() === "" || !types[0]}
              onClick={() => types[0] && start.mutate({ type: types[0].id, title: title.trim() })}
            >
              Start tracker
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
