/*
 * "Describe it and Operator drafts it": from the workflows or series
 * list, the teammate says in their own words what they want, and
 * Operator drafts a whole workflow or series on a fresh thread of their
 * own. What Operator drafted shows here by name and reason; creating it
 * makes the draft and opens the builder on it, where it is reviewed and
 * published like any other. Asking again replaces the draft Operator
 * suggested. When Operator asks something back, its answer shows here
 * and the teammate replies in the same box. Nothing runs or is sent
 * until the teammate publishes it in the builder.
 */
import { RiSparkling2Line } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import {
  askOperator,
  decideChangeset,
  type OperatorAnswer,
  type OperatorChangeset,
  startWorkspaceThread,
} from "@/lib/api/operator";

import { failureText } from "./use-operator-chat";

type DraftKind = "workflow" | "series";

const words: Record<DraftKind, { title: string; placeholder: string; ask: string; kind: string }> = {
  workflow: {
    title: "Describe a workflow",
    placeholder:
      "When a customer writes about billing outside office hours, tell them when we reply, tag it billing and assign it to the Billing team.",
    ask:
      "Draft a new workflow from what I describe below. Read get_workflow_reference first, look up any team, " +
      "teammate, tag or agent it needs, then call propose_new_workflow once with the whole draft. If something " +
      "I describe is unclear, ask me one short question instead.",
    kind: "workflow.create_draft",
  },
  series: {
    title: "Describe a series",
    placeholder:
      "Welcome new trial users: a hello email on day one, a tips email two days later for those who have not invited a teammate, and a check-in before the trial ends.",
    ask:
      "Draft a new series from what I describe below. Read get_series_reference first, look up any audience, tag " +
      "or topic it needs, write every message in full, then call propose_new_series once with the whole draft. If " +
      "something I describe is unclear, ask me one short question instead.",
    kind: "series.create_draft",
  },
};

/*
 * drafted is the new draft an answer suggests, when it suggests one.
 */
function drafted(answer: OperatorAnswer | undefined, kind: DraftKind): OperatorChangeset | null {
  return (
    answer?.changesets.find((c) => c.status === "pending" && c.operations.some((o) => o.kind === words[kind].kind)) ??
    null
  );
}

export function DraftFromDescription({
  workspace,
  kind,
  isOpen,
  onOpenChange,
}: {
  workspace: string;
  kind: DraftKind;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [text, setText] = useState("");
  const [thread, setThread] = useState<string | null>(null);
  const [answer, setAnswer] = useState<OperatorAnswer | undefined>(undefined);
  const suggested = drafted(answer, kind);
  const copy = words[kind];

  const ask = useMutation({
    mutationFn: async (said: string) => {
      if (suggested) {
        await decideChangeset(workspace, suggested.id, "declines");
      }
      const id = thread ?? (await startWorkspaceThread(workspace)).id;
      setThread(id);
      return askOperator(workspace, id, answer ? said : `${copy.ask}\n\n${said}`);
    },
    onSuccess: (next) => {
      setAnswer(next);
      setText("");
    },
    onError: (error) => toast.error(failureText(error)),
  });

  const create = useMutation({
    mutationFn: async (changeset: OperatorChangeset) => decideChangeset(workspace, changeset.id, "applications"),
    onSuccess: (applied) => {
      const made = applied.operations.find((o) => o.kind === copy.kind)?.created_id;
      void queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
      close(false);
      if (!made) {
        return;
      }
      if (kind === "workflow") {
        void navigate({ to: "/d/workflows/$workflow", params: { workflow: made } });
      } else {
        void navigate({ to: "/d/series/$series", params: { series: made } });
      }
    },
    onError: (error) => toast.error(failureText(error)),
  });

  const close = (open: boolean) => {
    onOpenChange(open);
    if (!open) {
      setText("");
      setThread(null);
      setAnswer(undefined);
    }
  };
  const said = text.trim();
  const reply = answer?.message.body.trim();

  return (
    <Modal isOpen={isOpen} onOpenChange={close} size="md">
      <ModalHeader
        title={copy.title}
        description={`Say what it should do in your own words. Operator drafts the whole ${kind}, and nothing runs until you publish it.`}
      />
      <ModalBody className="flex flex-col gap-4">
        {suggested ? (
          <section aria-label="Operator's draft" className="flex flex-col gap-1">
            <h3 className="text-body-medium text-text-primary">{suggested.title}</h3>
            <p className="whitespace-pre-wrap text-body-2-regular text-text-secondary">{suggested.reason}</p>
          </section>
        ) : reply ? (
          <p className="whitespace-pre-wrap text-body-2-regular text-text-secondary">{reply}</p>
        ) : null}
        <Textarea
          aria-label={copy.title}
          placeholder={answer ? "Ask Operator to change something, or answer its question" : copy.placeholder}
          value={text}
          onChange={setText}
          rows={4}
          autoResize
          maxRows={10}
          autoFocus
        />
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={() => close(false)}>
          Cancel
        </Button>
        {suggested && !said ? (
          <Button loading={create.isPending} onClick={() => create.mutate(suggested)}>
            Create draft
          </Button>
        ) : (
          <Button
            leadingIcon={RiSparkling2Line}
            loading={ask.isPending}
            disabled={!said}
            onClick={() => ask.mutate(said)}
          >
            {ask.isPending ? "Drafting…" : answer ? "Send" : "Draft it"}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}
