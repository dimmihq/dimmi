/*
 * Escalating a customer ticket: the teammate picks a back-office type on
 * radio cards and the team that does the work, names the job and says
 * what is needed. The back-office ticket is linked to the customer's,
 * and the customer's ticket moves to waiting on that team, so the
 * customer sees who has it while the agent's queue stays clear.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { teamsQuery } from "@/lib/api/teams";
import { escalateTicket, type Ticket, ticketKeys, ticketTypesQuery } from "@/lib/api/tickets";

import { typeIcon } from "./labels";

const noTeam = "none";

export function EscalateDialog({
  workspace,
  ticket,
  onClose,
}: {
  workspace: string;
  ticket: Ticket;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));
  const { data: teams = [] } = useQuery(teamsQuery(workspace));
  const backOffice = types.filter((t) => t.kind === "back_office" && !t.archived_at);
  const [typeId, setTypeId] = useState<string | null>(null);
  const chosen = backOffice.find((t) => t.id === (typeId ?? backOffice[0]?.id));
  const [team, setTeam] = useState<string | null>(null);
  const teamId = team ?? chosen?.team_id ?? noTeam;
  const [title, setTitle] = useState(`${ticket.reference}: ${ticket.title}`);
  const [description, setDescription] = useState("");
  const escalate = useMutation({
    mutationFn: () =>
      escalateTicket(workspace, ticket.id, {
        type_id: chosen?.id ?? "",
        title: title.trim(),
        description: description.trim() || undefined,
        team_id: teamId === noTeam ? undefined : teamId,
      }),
    onSuccess: async (made) => {
      await client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
      toast.success(`${made.reference} sent to the team`);
      onClose();
    },
    onError: (error) => toast.error((error as Error).message),
  });

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={`Escalate ${ticket.reference}`}
        description="Opens back-office work for another team. The customer sees it is with them."
      />
      <ModalBody className="flex flex-col gap-4 pb-4">
        {backOffice.length === 0 ? (
          <p className="text-body-2-regular text-text-secondary">
            Add a back-office type under Settings → Tickets first.
          </p>
        ) : (
          <RadioGroup
            aria-label="Back-office type"
            value={chosen?.id ?? null}
            onChange={setTypeId}
            className="flex flex-col gap-2"
          >
            {backOffice.map((t) => (
              <RadioCard
                key={t.id}
                value={t.id}
                title={t.name}
                description={t.description || undefined}
                icon={typeIcon(t.icon, t.kind)}
              />
            ))}
          </RadioGroup>
        )}
        <Select
          label="Team"
          size="sm"
          selectedKey={teamId}
          onSelectionChange={(key) => setTeam(key === null ? null : String(key))}
        >
          {[{ id: noTeam, name: "No team" }, ...teams].map((t) => (
            <SelectItem key={t.id} id={t.id} textValue={t.name}>
              {t.name}
            </SelectItem>
          ))}
        </Select>
        <Input label="Title" size="small" value={title} onChange={setTitle} maxLength={200} isRequired />
        <Textarea label="What the team needs to do" rows={3} value={description} onChange={setDescription} />
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button
          size="small"
          loading={escalate.isPending}
          disabled={!chosen || title.trim() === ""}
          onClick={() => escalate.mutate()}
        >
          Escalate
        </Button>
      </ModalFooter>
    </Modal>
  );
}
