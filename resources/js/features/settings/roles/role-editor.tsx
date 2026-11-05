/*
 * The editor of a custom role: its name, what it is for, the built-in
 * role it narrows, chosen once when it is made, and a switch per
 * permission, grouped as tokens reach them. Only permissions the base
 * role holds are offered, and seeing the other members is always on,
 * since the app cannot work without it.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Switch } from "@/components/base/switch/switch";
import { createRole, type Permission, permissionsQuery, type Role, type RoleDraft, updateRole } from "@/lib/api/roles";

import { useRecentAuth } from "../recent-auth";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

const alwaysHeld = "members:read";

const bases: RoleDraft["base_role"][] = ["member", "admin"];

/*
 * groupsOf returns the permissions a role of the base role may hold,
 * by ability group, in catalog order.
 */
function groupsOf(catalog: Permission[], base: RoleDraft["base_role"]): [string, Permission[]][] {
  const groups = new Map<string, Permission[]>();

  for (const permission of catalog) {
    if (!permission.assignable || !permission.base_roles.includes(base)) {
      continue;
    }

    groups.set(permission.group, [...(groups.get(permission.group) ?? []), permission]);
  }

  return [...groups.entries()];
}

function PermissionGroup({
  group,
  permissions,
  held,
  onChange,
}: {
  group: string;
  permissions: Permission[];
  held: string[];
  onChange: (key: string, on: boolean) => void;
}) {
  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="pb-1 text-caption-1-semibold text-text-tertiary">{group}</legend>
      {permissions.map((permission) => (
        <Switch
          key={permission.key}
          size="sm"
          isSelected={permission.key === alwaysHeld || held.includes(permission.key)}
          isDisabled={permission.key === alwaysHeld}
          onChange={(on) => onChange(permission.key, on)}
        >
          <span className="flex flex-col">
            <span className="text-body-2-medium text-text-primary">{permission.label}</span>
            <span className="text-body-2-regular text-text-tertiary">{permission.description}</span>
          </span>
        </Switch>
      ))}
    </fieldset>
  );
}

export function RoleEditor({
  role,
  start,
  isOpen,
  onClose,
}: {
  role: Role | null;
  start: RoleDraft;
  isOpen: boolean;
  onClose: () => void;
}) {
  const { workspace, hasPassword } = useAccount();
  const queryClient = useQueryClient();
  const { data: catalog } = useQuery(permissionsQuery(workspace.id));
  const { run, prompt } = useRecentAuth(hasPassword);
  const [draft, setDraft] = useState<RoleDraft>(start);

  const save = useMutation({
    mutationFn: () =>
      run(async () => {
        const { base_role: _, ...edits } = draft;
        await (role?.id ? updateRole(workspace.id, role.id, edits) : createRole(workspace.id, draft));
      }),
    onSuccess: async (saved) => {
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace.id] });
      if (saved) {
        onClose();
      }
    },
  });

  const toggle = (key: string, on: boolean) =>
    setDraft((current) => ({
      ...current,
      permissions: on ? [...current.permissions, key] : current.permissions.filter((held) => held !== key),
    }));

  const submit = (event: FormEvent) => {
    event.preventDefault();
    save.mutate();
  };

  return (
    <Modal size="lg" isOpen={isOpen} onOpenChange={(open) => (open ? null : onClose())}>
      <form onSubmit={submit}>
        <ModalHeader
          title={role ? `Edit ${role.name}` : "New role"}
          description="Members holding this role do exactly what it allows."
        />
        <ModalBody className="flex max-h-[60vh] flex-col gap-4 overflow-y-auto">
          <Input
            label="Name"
            value={draft.name}
            onChange={(name) => setDraft({ ...draft, name })}
            isInvalid={save.error !== null}
            errorMessage={failureText(save.error)}
          />
          <Input
            label="Description"
            value={draft.description}
            onChange={(description) => setDraft({ ...draft, description })}
          />
          {role ? null : (
            <SegmentedControl
              aria-label="Based on"
              selectedKeys={[draft.base_role]}
              onSelectionChange={(keys) => {
                const base = [...keys][0] === "admin" ? "admin" : "member";
                setDraft({ ...draft, base_role: base });
              }}
            >
              {bases.map((base) => (
                <SegmentedControlItem key={base} id={base}>
                  {`Based on ${base}`}
                </SegmentedControlItem>
              ))}
            </SegmentedControl>
          )}
          {groupsOf(catalog ?? [], draft.base_role).map(([group, permissions]) => (
            <PermissionGroup
              key={group}
              group={group}
              permissions={permissions}
              held={draft.permissions}
              onChange={toggle}
            />
          ))}
        </ModalBody>
        <ModalFooter className="mt-6">
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={save.isPending || draft.name.trim() === ""}>
            {role ? "Save role" : "Create role"}
          </Button>
        </ModalFooter>
      </form>
      {prompt}
    </Modal>
  );
}
