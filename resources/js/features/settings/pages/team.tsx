/*
 * One team of the workspace: its name, who is in it, and deleting it.
 * Members are picked from the workspace's active members who are not in
 * the team yet. Deleting a team asks where its conversations go first.
 * People who may not change members only see the team.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
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
import { Empty, Item, Row, Section, SettingsPage } from "../layout";
import { TeamDeleteDialog } from "../team-delete-dialog";
import { failureText, memberCount, useCanManageMembers } from "../teams";
import { useAccount } from "../use-account";

/*
 * Problem shows what went wrong with the last change, in the error
 * colour, or nothing.
 */
function Problem({ error }: { error: Error | null }) {
  if (!error) {
    return null;
  }

  return (
    <p role="alert" className="px-3 text-body-2-regular text-text-error-primary">
      {failureText(error)}
    </p>
  );
}

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
 * useTeamChanges holds the calls that change one team, each refreshing
 * the team and the list once it lands.
 */
function useTeamChanges(workspace: string, team: string) {
  const queryClient = useQueryClient();
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "teams"] });

  return {
    rename: useMutation({ mutationFn: (name: string) => renameTeam(workspace, team, name), onSuccess: refresh }),
    add: useMutation({ mutationFn: (member: string) => addTeamMembers(workspace, team, [member]), onSuccess: refresh }),
    remove: useMutation({
      mutationFn: (member: string) => removeTeamMember(workspace, team, member),
      onSuccess: refresh,
    }),
    destroy: useMutation({ mutationFn: (to: ReassignTo) => deleteTeam(workspace, team, to) }),
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
  const changes = useTeamChanges(workspace, team.id);
  const [name, setName] = useState(team.name);
  const [deleting, setDeleting] = useState(false);
  const members = team.members ?? [];
  const candidates = everyone.filter((m) => m.status === "active" && !members.some((t) => t.id === m.id));
  const error = changes.rename.error ?? changes.add.error ?? changes.remove.error;

  return (
    <SettingsPage title={team.name} description={memberCount(team.member_count)}>
      <BackToTeams />
      {canManage ? (
        <Section
          title="General"
          action={
            <Button
              size="small"
              loading={changes.rename.isPending}
              disabled={name.trim() === "" || name === team.name}
              onClick={() => changes.rename.mutate(name)}
            >
              Save changes
            </Button>
          }
        >
          <Row label="Name">
            <Field key={`name-${team.name}`} label="Team name" value={team.name} onChange={setName} />
          </Row>
        </Section>
      ) : null}
      <Section
        title="Members"
        action={canManage ? <MemberPicker candidates={candidates} onPick={(id) => changes.add.mutate(id)} /> : null}
      >
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
              <Action size="xs" onPress={() => changes.remove.mutate(member.id)}>
                Remove
              </Action>
            ) : null}
          </Item>
        ))}
      </Section>
      <Problem error={error} />
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
          await changes.destroy.mutateAsync(to);
          await navigate({ to: "/d/settings/$section", params: { section: "teams" } });
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
    return null;
  }

  return <TeamPage key={data.id} team={data} canManage={canManage} />;
}
