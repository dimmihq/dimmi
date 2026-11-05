/*
 * The workspace's teams, on the server: every live team with how many
 * people are in it, and creating a new one. Each team opens on its own
 * page to rename it, delete it and choose its members. Creating is
 * offered only to people the server lets change members.
 */
import { RiTeamLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { createTeam, teamsQuery } from "@/lib/api/teams";

import { Action } from "../controls";
import { Empty, Glyph, Item, Section, SettingsPage } from "../layout";
import { failureText, memberCount, useCanManageMembers } from "../teams";
import { useAccount } from "../use-account";

/*
 * NewTeamDialog asks for the name of a new team and opens its page once
 * it exists.
 */
function NewTeamDialog({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [name, setName] = useState("");

  const create = useMutation({
    mutationFn: () => createTeam(workspace, name),
    onSuccess: async (team) => {
      setName("");
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "teams"] });
      await navigate({ to: "/d/settings/teams/$team", params: { team: team.id } });
    },
  });

  return (
    <AlertDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          create.reset();
          onClose();
        }
      }}
      variant="default"
      icon={false}
      title="New team"
      description="Conversations can be assigned to the team, and inboxes can route to it."
      confirmLabel="Create team"
      isConfirmDisabled={name.trim() === ""}
      onConfirm={() => create.mutateAsync()}
    >
      <Input
        label="Team name"
        value={name}
        onChange={setName}
        isInvalid={create.error !== null}
        errorMessage={failureText(create.error)}
      />
    </AlertDialog>
  );
}

export function TeamsSettings() {
  const workspace = useAccount().workspace.id;
  const navigate = useNavigate();
  const { data: teams } = useQuery(teamsQuery(workspace));
  const canManage = useCanManageMembers(workspace);
  const [creating, setCreating] = useState(false);
  const newTeam = canManage ? <Action onPress={() => setCreating(true)}>New team</Action> : undefined;

  return (
    <SettingsPage title="Teams" description="Assign conversations to a team, and let an inbox route to one.">
      <Section title="Teams" action={newTeam} loading={!teams}>
        {teams?.length === 0 ? <Empty>No teams yet.</Empty> : null}
        {teams?.map((team) => (
          <Item
            key={team.id}
            mark={<Glyph icon={RiTeamLine} />}
            title={team.name}
            detail={memberCount(team.member_count)}
          >
            <Action size="xs" onPress={() => navigate({ to: "/d/settings/teams/$team", params: { team: team.id } })}>
              {canManage ? "Manage" : "View"}
            </Action>
          </Item>
        ))}
      </Section>
      <NewTeamDialog isOpen={creating} onClose={() => setCreating(false)} />
    </SettingsPage>
  );
}
