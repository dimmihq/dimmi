/*
 * Choosing which conversations a teammate sees: every one, only their
 * own, or the conversations of some teams, with or without those nobody
 * holds yet. The dialog explains each level in a sentence and only lets
 * a level that names teams be saved once a team is picked.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { type Team, teamsQuery } from "@/lib/api/teams";
import { type Member, updateMember } from "@/lib/api/workspace";

import { type AccessDraft, accessChanges, levelLabel, levelSentence, levels, needsTeams } from "./member-access";

function draftOf(member: Member): AccessDraft {
  return {
    level: member.conversation_access,
    unassigned: member.conversation_access_unassigned,
    teams: member.access_team_ids,
  };
}

function AccessFields({
  draft,
  teams,
  onChange,
}: {
  draft: AccessDraft;
  teams: Team[];
  onChange: (next: Partial<AccessDraft>) => void;
}) {
  return (
    <>
      <Select
        label="Sees"
        size="sm"
        selectedKey={draft.level}
        onSelectionChange={(key) => {
          const level = levels.find((l) => l === key);
          if (level) {
            onChange({ level });
          }
        }}
      >
        {levels.map((level) => (
          <SelectItem key={level} id={level} textValue={levelLabel(level)}>
            {levelLabel(level)}
          </SelectItem>
        ))}
      </Select>
      {draft.level === "all" ? null : (
        <Checkbox isSelected={draft.unassigned} onChange={(on) => onChange({ unassigned: on })}>
          Also see conversations nobody is assigned to
        </Checkbox>
      )}
      {needsTeams(draft.level) ? <TeamPicker draft={draft} teams={teams} onChange={onChange} /> : null}
    </>
  );
}

function TeamPicker({
  draft,
  teams,
  onChange,
}: {
  draft: AccessDraft;
  teams: Team[];
  onChange: (next: Partial<AccessDraft>) => void;
}) {
  const toggle = (id: string, on: boolean) =>
    onChange({ teams: on ? [...draft.teams, id] : draft.teams.filter((team) => team !== id) });

  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="mb-2 text-body-2-medium text-text-secondary">Teams</legend>
      {teams.length === 0 ? <p className="text-body-2-regular text-text-tertiary">Create a team first.</p> : null}
      {teams.map((team) => (
        <Checkbox key={team.id} isSelected={draft.teams.includes(team.id)} onChange={(on) => toggle(team.id, on)}>
          {team.name}
        </Checkbox>
      ))}
    </fieldset>
  );
}

export function MemberAccessDialog({
  workspace,
  member,
  onClose,
}: {
  workspace: string;
  member: Member | null;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: member !== null });
  const [draft, setDraft] = useState<AccessDraft | null>(null);
  const current = draft ?? (member ? draftOf(member) : null);
  const changes = current ? accessChanges(current) : null;

  const save = useMutation({
    mutationFn: () => (member && changes ? updateMember(workspace, member.id, changes) : Promise.resolve(null)),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
      close();
    },
  });

  const close = () => {
    setDraft(null);
    save.reset();
    onClose();
  };

  const change = (next: Partial<AccessDraft>) => {
    if (current) {
      setDraft({ ...current, ...next });
    }
  };

  return (
    <Modal size="sm" isOpen={member !== null} onOpenChange={(open) => (open ? null : close())}>
      <ModalHeader
        title={`Conversation access for ${member?.user.name ?? ""}`}
        description={current ? levelSentence(current.level) : undefined}
      />
      <ModalBody className="flex flex-col gap-4">
        {current ? <AccessFields draft={current} teams={teams} onChange={change} /> : null}
        {save.error ? (
          <p role="alert" className="text-body-2-regular text-text-error-primary">
            {save.error.message}
          </p>
        ) : null}
      </ModalBody>
      <ModalFooter className="mt-6">
        <Button variant="secondary" onClick={close}>
          Cancel
        </Button>
        <Button disabled={changes === null || save.isPending} onClick={() => save.mutate()}>
          Save access
        </Button>
      </ModalFooter>
    </Modal>
  );
}
