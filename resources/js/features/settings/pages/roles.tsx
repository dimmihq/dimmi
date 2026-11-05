/*
 * The workspace's roles: the built-in ones, read only, and its custom
 * roles, each a named part of what an admin or a member may do. People
 * who manage roles create them, from scratch or from a preset, edit and
 * delete them; a role someone holds cannot be deleted until they move.
 */
import { RiShieldUserLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { AlertDialog } from "@/components/base/modal/modal";
import { deleteRole, permissionsQuery, type Role, type RoleDraft, rolesQuery } from "@/lib/api/roles";
import { callerQuery } from "@/lib/api/workspace";

import { Action, Danger } from "../controls";
import { Empty, Glyph, Item, Section, SettingsPage } from "../layout";
import { useRecentAuth } from "../recent-auth";
import { presets } from "../roles/presets";
import { RoleEditor } from "../roles/role-editor";
import { failureText, memberCount } from "../teams";
import { useAccount } from "../use-account";

const blank: RoleDraft = { name: "", description: "", base_role: "member", permissions: ["members:read"] };

/*
 * Editing is the role open in the editor: an existing one, or a new one
 * starting from a draft.
 */
interface Editing {
  role: Role | null;
  start: RoleDraft;
}

function BuiltInRoles({ roles }: { roles: Role[] }) {
  return (
    <Section title="Built-in roles" description="Every workspace has these. Custom roles narrow admin or member.">
      {roles.map((role) => (
        <Item
          key={role.base_role}
          mark={<Glyph icon={RiShieldUserLine} />}
          title={role.name}
          detail={`${role.permissions.length} permissions`}
        />
      ))}
    </Section>
  );
}

function NewRoleMenu({ onPick }: { onPick: (start: RoleDraft) => void }) {
  const workspace = useAccount().workspace.id;
  const { data: catalog } = useQuery(permissionsQuery(workspace));

  return (
    <Dropdown>
      <DropdownTrigger className="rounded-lg px-2 py-1 text-body-2-medium text-text-primary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring">
        New role
      </DropdownTrigger>
      <DropdownPopover aria-label="New role" placement="bottom end">
        <DropdownItem onSelect={() => onPick(blank)}>
          <span className="text-body-2-regular text-text-primary">Start from scratch</span>
        </DropdownItem>
        {presets(catalog ?? []).map((preset) => (
          <DropdownItem key={preset.key} onSelect={() => onPick(preset.draft)}>
            <span className="text-body-2-regular text-text-primary">{preset.label}</span>
          </DropdownItem>
        ))}
      </DropdownPopover>
    </Dropdown>
  );
}

function DeleteRoleDialog({ role, onClose }: { role: Role | null; onClose: () => void }) {
  const { workspace, hasPassword } = useAccount();
  const queryClient = useQueryClient();
  const { run, prompt } = useRecentAuth(hasPassword);
  const remove = useMutation({
    mutationFn: (id: string) => run(() => deleteRole(workspace.id, id)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace.id, "roles"] }),
  });

  return (
    <>
      <AlertDialog
        isOpen={role !== null}
        onOpenChange={(open) => (open ? null : onClose())}
        title={`Delete ${role?.name ?? "role"}`}
        description={remove.error ? failureText(remove.error) : "Nobody holds it. Deleting it cannot be undone."}
        confirmLabel="Delete role"
        onConfirm={async () => {
          if (role?.id) {
            await remove.mutateAsync(role.id);
          }
        }}
      />
      {prompt}
    </>
  );
}

export function RolesSettings() {
  const workspace = useAccount().workspace.id;
  const { data: roles } = useQuery(rolesQuery(workspace));
  const { data: caller } = useQuery(callerQuery(workspace));
  const canManage = caller?.member?.permissions.includes("roles:manage") ?? false;
  const [editing, setEditing] = useState<Editing | null>(null);
  const [deleting, setDeleting] = useState<Role | null>(null);
  const custom = roles?.filter((role) => role.kind === "custom") ?? [];

  return (
    <SettingsPage title="Roles" description="What each person in the workspace may do.">
      <Section
        title="Custom roles"
        description="A custom role keeps part of what its base role allows."
        action={canManage ? <NewRoleMenu onPick={(start) => setEditing({ role: null, start })} /> : undefined}
        loading={!roles}
        rows={2}
      >
        {custom.length === 0 ? <Empty>No custom roles yet.</Empty> : null}
        {custom.map((role) => (
          <Item
            key={role.id}
            mark={<Glyph icon={RiShieldUserLine} />}
            title={role.name}
            detail={[`Based on ${role.base_role}`, role.member_count === null ? null : memberCount(role.member_count)]
              .filter(Boolean)
              .join(" · ")}
          >
            {canManage ? (
              <>
                <Action
                  size="xs"
                  onPress={() =>
                    setEditing({
                      role,
                      start: {
                        name: role.name,
                        description: role.description ?? "",
                        base_role: role.base_role === "admin" ? "admin" : "member",
                        permissions: role.permissions,
                      },
                    })
                  }
                >
                  Edit
                </Action>
                <Danger onPress={() => setDeleting(role)}>Delete</Danger>
              </>
            ) : null}
          </Item>
        ))}
      </Section>
      <BuiltInRoles roles={roles?.filter((role) => role.kind === "built_in") ?? []} />
      {editing ? (
        <RoleEditor
          key={editing.role?.id ?? editing.start.name}
          role={editing.role}
          start={editing.start}
          isOpen
          onClose={() => setEditing(null)}
        />
      ) : null}
      <DeleteRoleDialog role={deleting} onClose={() => setDeleting(null)} />
    </SettingsPage>
  );
}
