/*
 * Deleting a team says first how many conversations it holds and asks
 * where they go: to another team, keeping who holds each, to one
 * teammate, or to nobody. With conversations at stake nothing is picked
 * for the admin; an empty team deletes straight away.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { type ReassignTo, type Team, teamCountsQuery, teamsQuery } from "@/lib/api/teams";
import { membersQuery } from "@/lib/api/workspace";

import { type Choice, choices, countLine, target } from "./team-delete";

export function TeamDeleteDialog({
  workspace,
  team,
  isOpen,
  onOpenChange,
  onDelete,
}: {
  workspace: string;
  team: Team;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onDelete: (to: ReassignTo) => Promise<void>;
}) {
  const { data: counts } = useQuery({ ...teamCountsQuery(workspace, team.id), enabled: isOpen });
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: isOpen });
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: isOpen });
  const [choice, setChoice] = useState<Choice | null>(null);
  const [picked, setPicked] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const empty = counts !== undefined && counts.open + counts.snoozed + counts.closed === 0;
  const to = empty ? null : target(choice, picked);
  const others = teams.filter((t) => t.id !== team.id);
  const active = members.filter((m) => m.status === "active");

  const confirm = async () => {
    if (to === undefined) {
      return;
    }
    setPending(true);
    try {
      await onDelete(to);
    } finally {
      setPending(false);
    }
  };

  return (
    <Modal size="sm" isOpen={isOpen} onOpenChange={onOpenChange}>
      <ModalHeader
        title={`Delete ${team.name}?`}
        description={counts ? countLine(counts) : "Counting its conversations…"}
      />
      <ModalBody className="flex flex-col gap-4">
        {counts && !empty ? (
          <>
            <SegmentedControl
              aria-label="Where its conversations go"
              selectedKeys={choice ? [choice] : []}
              onSelectionChange={(selected) => {
                const next = [...selected][0];
                if (next === choices.team || next === choices.member || next === choices.none) {
                  setChoice(next);
                  setPicked(null);
                }
              }}
            >
              <SegmentedControlItem id={choices.team} isDisabled={others.length === 0}>
                Another team
              </SegmentedControlItem>
              <SegmentedControlItem id={choices.member}>A teammate</SegmentedControlItem>
              <SegmentedControlItem id={choices.none}>Nobody</SegmentedControlItem>
            </SegmentedControl>
            {choice === choices.team ? (
              <Select
                aria-label="Team to move them to"
                placeholder="Pick a team"
                size="sm"
                selectedKey={picked}
                onSelectionChange={(key) => setPicked(key === null ? null : String(key))}
              >
                {others.map((t) => (
                  <SelectItem key={t.id} id={t.id} textValue={t.name}>
                    {t.name}
                  </SelectItem>
                ))}
              </Select>
            ) : null}
            {choice === choices.member ? (
              <Select
                aria-label="Teammate to give them to"
                placeholder="Pick a teammate"
                size="sm"
                selectedKey={picked}
                onSelectionChange={(key) => setPicked(key === null ? null : String(key))}
              >
                {active.map((m) => (
                  <SelectItem key={m.id} id={m.id} textValue={m.user.name}>
                    {m.user.name}
                  </SelectItem>
                ))}
              </Select>
            ) : null}
          </>
        ) : null}
      </ModalBody>
      <ModalFooter className="mt-6">
        <Button variant="secondary" onClick={() => onOpenChange(false)}>
          Cancel
        </Button>
        <Button variant="danger" disabled={counts === undefined || to === undefined || pending} onClick={confirm}>
          Delete team
        </Button>
      </ModalFooter>
    </Modal>
  );
}
