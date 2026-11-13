/*
 * One team of the workspace: its name, who is in it, and deleting it.
 * Members are picked from the workspace's active members who are not in
 * the team yet. Deleting a team asks where its conversations go first.
 * People who may not change members only see the team. A new name and
 * members added or removed wait for Save in the bar at the bottom;
 * leaving with unsaved changes asks first.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { avatarColor } from "@/features/shell/avatar-color";
import {
  addTeamMembers,
  deleteTeam,
  type ReassignTo,
  removeTeamMember,
  renameTeam,
  type Team,
  teamQuery,
} from "@/lib/api/teams";
import { type Member, membersQuery } from "@/lib/api/workspace";

import { Action, Danger, Field } from "../controls";
import { Empty, Item, PageSkeleton, Row, Section, SettingsPage } from "../layout";
import { SaveBar } from "../save-bar";
import { TeamDeleteDialog } from "../team-delete-dialog";
import { failureText, memberCount, useCanManageMembers } from "../teams";
import { useAccount } from "../use-account";

function BackToTeams() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "teams" } })}
      >
        All teams
      </Button>
    </div>
  );
}

/*
 * useTeamDraft holds the name and the members added or removed but not
 * saved yet. Taking back an addition or a removal drops it. Save renames,
 * adds and removes in turn, then reads the team and the list again;
 * Discard puts everything back.
 */
function useTeamDraft(workspace: string, team: Team) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(team.name);
  const [added, setAdded] = useState<string[]>([]);
  const [removed, setRemoved] = useState<string[]>([]);
  const [version, setVersion] = useState(0);
  const renamed = name.trim() !== "" && name !== team.name;
  const reset = () => {
    setName(team.name);
    setAdded([]);
    setRemoved([]);
    setVersion((count) => count + 1);
  };
  const save = useMutation({
    mutationFn: async () => {
      if (renamed) {
        await renameTeam(workspace, team.id, name);
      }
      if (added.length > 0) {
        await addTeamMembers(workspace, team.id, added);
      }
      await Promise.all(removed.map((member) => removeTeamMember(workspace, team.id, member)));
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "teams"] });
      setAdded([]);
      setRemoved([]);
      toast.success("Team saved");
    },
  });

  return {
    name,
    version,
    added,
    removed,
    dirty: name !== team.name || added.length > 0 || removed.length > 0,
    canSave: name.trim() !== "",
    save,
    rename: setName,
    add: (member: string) =>
      removed.includes(member)
        ? setRemoved((current) => current.filter((id) => id !== member))
        : setAdded((current) => [...current, member]),
    remove: (member: string) =>
      added.includes(member)
        ? setAdded((current) => current.filter((id) => id !== member))
        : setRemoved((current) => [...current, member]),
    discard: reset,
  };
}

/*
 * MemberPicker adds one of the workspace's active members who is not in
 * the team yet.
 */
function MemberPicker({ candidates, onPick }: { candidates: Member[]; onPick: (member: string) => void }) {
  if (candidates.length === 0) {
    return null;
  }

  return (
    <Select
      aria-label="Add a member"
      placeholder="Add a member"
      size="sm"
      selectedKey={null}
      onSelectionChange={(key) => (key === null ? undefined : onPick(String(key)))}
      popoverClassName="w-64"
    >
      {candidates.map((member) => (
        <SelectItem key={member.id} id={member.id} textValue={member.user.name}>
          {member.user.name}
        </SelectItem>
      ))}
    </Select>
  );
}

function TeamPage({ team, canManage }: { team: Team; canManage: boolean }) {
  const workspace = useAccount().workspace.id;
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: everyone = [] } = useQuery(membersQuery(workspace));
  const draft = useTeamDraft(workspace, team);
  const destroy = useMutation({ mutationFn: (to: ReassignTo) => deleteTeam(workspace, team.id, to) });
  const [deleting, setDeleting] = useState(false);
  const members = [
    ...(team.members ?? []).filter((m) => !draft.removed.includes(m.id)),
    ...everyone.filter((m) => draft.added.includes(m.id)),
  ];
  const candidates = everyone.filter((m) => m.status === "active" && !members.some((t) => t.id === m.id));
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  return (
    <SettingsPage
      title={team.name}
      description={memberCount(team.member_count)}
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error ? failureText(draft.save.error) : null}
          canSave={draft.dirty && draft.canSave}
          onSave={() => draft.save.mutate()}
          onDiscard={draft.discard}
        />
      }
    >
      <BackToTeams />
      {canManage ? (
        <Section title="General">
          <Row label="Name">
            <Field
              key={`name-${team.name}-${draft.version}`}
              label="Team name"
              value={draft.name}
              onChange={draft.rename}
            />
          </Row>
        </Section>
      ) : null}
      <Section title="Members" action={canManage ? <MemberPicker candidates={candidates} onPick={draft.add} /> : null}>
        {members.length === 0 ? <Empty>Nobody is in this team yet.</Empty> : null}
        {members.map((member) => (
          <Item
            key={member.id}
            mark={
              <Avatar
                size="md"
                color={avatarColor(member.user.name)}
                initials={member.user.name.charAt(0)}
                src={member.user.avatar_url ?? undefined}
              />
            }
            title={member.user.name}
            detail={member.user.email}
          >
            {canManage ? (
              <Action size="xs" onPress={() => draft.remove(member.id)}>
                Remove
              </Action>
            ) : null}
          </Item>
        ))}
      </Section>
      {canManage ? (
        <Section title="Danger zone">
          <Row label="Delete team" description="You choose where its conversations go. Its members stay.">
            <Danger onPress={() => setDeleting(true)}>Delete team</Danger>
          </Row>
        </Section>
      ) : null}
      <TeamDeleteDialog
        workspace={workspace}
        team={team}
        isOpen={deleting}
        onOpenChange={setDeleting}
        onDelete={async (to) => {
          await destroy.mutateAsync(to);
          draft.discard();
          await navigate({ to: "/d/settings/$section", params: { section: "teams" }, ignoreBlocker: true });
          await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "teams"] });
        }}
      />
    </SettingsPage>
  );
}

export function TeamSettings({ team }: { team: string }) {
  const workspace = useAccount().workspace.id;
  const canManage = useCanManageMembers(workspace);
  const { data, isError } = useQuery(teamQuery(workspace, team));

  if (isError) {
    return (
      <SettingsPage title="Team not found" description="It was deleted, or it belongs to another workspace.">
        <BackToTeams />
      </SettingsPage>
    );
  }

  if (!data) {
    return <PageSkeleton />;
  }

  return <TeamPage key={data.id} team={data} canManage={canManage} />;
}
