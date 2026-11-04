/*
 * The workspace's own settings and its people, on the server: renaming
 * the workspace and moving its address, and seeing who belongs to it,
 * changing roles, deactivating and bringing people back, handing
 * ownership over and leaving. What a person is offered follows the
 * permissions the server gives them here, never their role, and changes
 * that touch owners ask for the password again when the last sign-in is
 * a while ago.
 */
import { RiMore2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearch } from "@tanstack/react-router";
import { useMemo, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { avatarColor } from "@/features/shell/avatar-color";
import { ApiError } from "@/lib/api/client";
import {
  callerQuery,
  confirmPassword,
  leaveWorkspace,
  type Member,
  type MemberChanges,
  membersQuery,
  updateMember,
  updateWorkspace,
  type WorkspaceChanges,
  workspaceQuery,
} from "@/lib/api/workspace";

import { Danger, Field } from "../controls";
import { DeleteWorkspace } from "../delete-workspace";
import { InviteButton, PendingInvitations } from "../invitations";
import { Item, Row, Section, SettingsPage } from "../layout";
import { MemberImportButton } from "../members/member-import-dialog";
import { type FilterKey, filterFor, filterKeyFrom } from "../members/member-review";
import { ExportMembersButton, LastActive, MemberFilterSelect, TwoFactorMark } from "../members/member-review-controls";
import { type Help, type HelpPowers, helpLabel, helpOffers } from "../members/sign-in-help";
import { SignInHelpDialogs } from "../members/two-factor-help-dialog";
import { WorkspaceLanguageRow } from "../preferences/language-row";
import { ZonePicker } from "../preferences/zone-picker";
import { RequireTwoFactorSection } from "../require-two-factor";
import { useAccount } from "../use-account";

/*
 * Draft is what the General section holds before it is saved. An empty
 * name for customers saves as none, so they see the workspace's name.
 */
interface Draft {
  name?: string;
  slug?: string;
  customerName?: string;
}

/*
 * draftChanges turns the draft into the change the server takes.
 */
function draftChanges(draft: Draft): WorkspaceChanges {
  const changes: WorkspaceChanges = { name: draft.name, slug: draft.slug };
  if (draft.customerName !== undefined) {
    changes.customer_facing_name = draft.customerName.trim() === "" ? null : draft.customerName;
  }

  return changes;
}

/*
 * Failure shows what went wrong with the last change, in the error
 * colour, or nothing.
 */
function Failure({ error }: { error: Error | null }) {
  if (!error) {
    return null;
  }

  return (
    <p role="alert" className="px-3 text-body-2-regular text-text-error-primary">
      {error instanceof ApiError ? (Object.values(error.fields)[0] ?? error.message) : error.message}
    </p>
  );
}

export function WorkspaceSettings() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const { data: workspace } = useQuery(workspaceQuery(me.workspace.id));
  const [draft, setDraft] = useState<Draft>({});
  const zones = useMemo(() => ["UTC", ...Intl.supportedValuesOf("timeZone")].map((id) => ({ id, label: id })), []);

  const save = useMutation({
    mutationFn: (changes: WorkspaceChanges) => updateWorkspace(me.workspace.id, changes),
    onSuccess: (saved) => {
      setDraft({});
      queryClient.setQueryData(workspaceQuery(me.workspace.id).queryKey, saved);
      return queryClient.invalidateQueries({ queryKey: ["me"] });
    },
  });

  const name = workspace?.name ?? me.workspace.name;
  const slug = workspace?.slug ?? me.workspace.slug;
  const customerName = workspace?.customer_facing_name ?? "";
  const changed =
    (draft.name !== undefined && draft.name !== name) ||
    (draft.slug !== undefined && draft.slug !== slug) ||
    (draft.customerName !== undefined && draft.customerName.trim() !== customerName);

  return (
    <SettingsPage title="Workspace" description="Seen by everyone in the workspace.">
      <Section
        title="General"
        action={
          <Button size="small" disabled={!changed || save.isPending} onClick={() => save.mutate(draftChanges(draft))}>
            Save changes
          </Button>
        }
      >
        <Row label="Name">
          <Field
            key={`name-${name}`}
            label="Workspace name"
            value={name}
            onChange={(value) => setDraft((d) => ({ ...d, name: value }))}
          />
        </Row>
        <Row label="Address" description="The old address keeps redirecting for 30 days after a change.">
          <span className="text-body-2-regular text-text-tertiary">/w/</span>
          <Field
            key={`slug-${slug}`}
            label="Workspace address"
            value={slug}
            width="w-40"
            onChange={(value) => setDraft((d) => ({ ...d, slug: value }))}
          />
        </Row>
        {workspace ? (
          <Row label="Name customers see" description="Shown in the messenger and in email. Empty uses the name above.">
            <Field
              key={`customer-${customerName}`}
              label="Name customers see"
              value={customerName}
              placeholder={name}
              onChange={(value) => setDraft((d) => ({ ...d, customerName: value }))}
            />
          </Row>
        ) : null}
      </Section>
      <Failure error={save.error} />
      <Section title="Language and region" description="Members and customers fall back to these.">
        <WorkspaceLanguageRow workspace={me.workspace.id} />
        <Row label="Time zone">
          {workspace ? (
            <ZonePicker
              label="Workspace time zone"
              choices={zones}
              selected={workspace.timezone}
              onPick={(timezone) => save.mutate({ timezone })}
            />
          ) : null}
        </Row>
      </Section>
      <RequireTwoFactorSection workspace={me.workspace.id} hasPassword={me.hasPassword} />
      <DeleteWorkspace id={me.workspace.id} name={name} slug={slug} />
    </SettingsPage>
  );
}

function roleLabel(role: string): string {
  return role ? `${role.charAt(0).toUpperCase()}${role.slice(1)}` : "Member";
}

/*
 * Pending is a change waiting for the person to confirm it: what it
 * does, in words, and the change itself.
 */
interface Pending {
  title: string;
  description: string;
  confirm: string;
  member: Member;
  changes: MemberChanges;
}

/*
 * Powers is what the caller may do to members: change members and
 * admins (write), change owners (owners), and help them back in (reset).
 */
type Powers = HelpPowers;

/*
 * canManage reports whether the caller may change the member's status.
 */
function canManage(member: Member, powers: Powers): boolean {
  return powers.owners || (powers.write && member.role !== "owner");
}

/*
 * roleOffers lists the role changes the caller may make to an active
 * member.
 */
function roleOffers(member: Member, powers: Powers): Pending[] {
  const name = member.user.name;

  if (member.role === "owner") {
    return powers.owners
      ? [
          {
            title: `Make ${name} an admin`,
            description: `${name} stops being an owner.`,
            confirm: "Make admin",
            member,
            changes: { role: "admin" },
          },
        ]
      : [];
  }

  const list: Pending[] = [];
  if (powers.write) {
    const role = member.role === "admin" ? "member" : "admin";
    const becomes = role === "admin" ? "an admin" : "a member";
    list.push({
      title: `Make ${name} ${becomes}`,
      description: `${name} becomes ${becomes}.`,
      confirm: `Make ${role}`,
      member,
      changes: { role },
    });
  }
  if (powers.owners) {
    list.push({
      title: `Make ${name} the owner`,
      description: `${name} becomes an owner and you become an admin.`,
      confirm: "Transfer ownership",
      member,
      changes: { role: "owner" },
    });
  }

  return list;
}

/*
 * offers lists what the caller may do to a member, as menu entries,
 * from the permissions they hold.
 */
function offers(member: Member, powers: Powers): Pending[] {
  const name = member.user.name;

  if (member.status === "deactivated") {
    return canManage(member, powers)
      ? [
          {
            title: `Reactivate ${name}`,
            description: `${name} gets their access back.`,
            confirm: "Reactivate",
            member,
            changes: { status: "active" },
          },
        ]
      : [];
  }

  const list = roleOffers(member, powers);
  if (member.id !== powers.self && canManage(member, powers)) {
    list.push({
      title: `Deactivate ${name}`,
      description: `${name} loses access straight away. Their history stays.`,
      confirm: "Deactivate",
      member,
      changes: { status: "deactivated" },
    });
  }

  return list;
}

function Person({
  member,
  powers,
  timeZone,
  onPick,
  onHelp,
}: {
  member: Member;
  powers: Powers;
  timeZone?: string;
  onPick: (p: Pending) => void;
  onHelp: (help: Help) => void;
}) {
  const name = member.user.name;
  const choices = offers(member, powers);
  const helps = helpOffers(member, powers);
  const detail = member.status === "deactivated" ? `${member.user.email} · Deactivated` : member.user.email;

  return (
    <Item
      mark={<Avatar size="md" color={avatarColor(name)} initials={name.charAt(0)} />}
      title={member.id === powers.self ? `${name} (you)` : name}
      detail={detail}
    >
      <LastActive member={member} timeZone={timeZone} />
      <TwoFactorMark member={member} />
      <Chip variant="caption" color={member.role === "owner" ? "blue" : "neutral"}>
        {roleLabel(member.role)}
      </Chip>
      {choices.length + helps.length > 0 ? (
        <Dropdown>
          <DropdownTrigger
            aria-label={`Manage ${name}`}
            className="flex size-8 items-center justify-center rounded-lg outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiMore2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
          </DropdownTrigger>
          <DropdownPopover aria-label={`Manage ${name}`} placement="bottom end">
            {choices.map((choice) => (
              <DropdownItem key={choice.title} onSelect={() => onPick(choice)}>
                <span className="text-body-2-regular text-text-primary">{choice.confirm}</span>
              </DropdownItem>
            ))}
            {helps.map((help) => (
              <DropdownItem key={help.kind} onSelect={() => onHelp(help)}>
                <span className="text-body-2-regular text-text-primary">{helpLabel(help)}</span>
              </DropdownItem>
            ))}
          </DropdownPopover>
        </Dropdown>
      ) : null}
    </Item>
  );
}

/*
 * PasswordPrompt asks for the password before a change that touches an
 * owner, when the person last signed in a while ago, then lets the
 * change go ahead.
 */
function PasswordPrompt({
  isOpen,
  onClose,
  onConfirmed,
}: {
  isOpen: boolean;
  onClose: () => void;
  onConfirmed: () => Promise<unknown>;
}) {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  return (
    <AlertDialog
      isOpen={isOpen}
      onOpenChange={(open) => (open ? null : onClose())}
      variant="default"
      icon={false}
      title="Confirm your password"
      description="Changes to owners need your password again."
      confirmLabel="Confirm"
      isConfirmDisabled={password === ""}
      onConfirm={async () => {
        try {
          await confirmPassword(password);
          setPassword("");
          await onConfirmed();
        } catch (failure) {
          setError(failure instanceof Error ? failure.message : "Something went wrong.");
          throw failure;
        }
      }}
    >
      <Input
        type="password"
        label="Password"
        value={password}
        onChange={setPassword}
        isInvalid={error !== ""}
        errorMessage={error}
      />
    </AlertDialog>
  );
}

export function MembersSettings() {
  const me = useAccount();
  const workspace = me.workspace.id;
  const queryClient = useQueryClient();
  const [view, setView] = useState<FilterKey>(filterKeyFrom(useSearch({ strict: false })));
  const { data: caller } = useQuery(callerQuery(workspace));
  const manages = caller?.member?.permissions.includes("members:write") ?? false;
  const filter = manages ? filterFor(view) : {};
  const { data: members = [] } = useQuery({ ...membersQuery(workspace, filter), enabled: caller !== undefined });
  const [pending, setPending] = useState<Pending | null>(null);
  const [retry, setRetry] = useState<Pending | null>(null);
  const [leaving, setLeaving] = useState(false);
  const [help, setHelp] = useState<Help | null>(null);

  const self = caller?.member?.id ?? "";
  const permissions = caller?.member?.permissions ?? [];
  const powers: Powers = {
    self,
    write: permissions.includes("members:write"),
    owners: permissions.includes("members:manage_owners"),
    reset: permissions.includes("members:reset_two_factor"),
  };
  const active = members.filter((member) => member.status === "active").length;

  const change = useMutation({
    mutationFn: (p: Pending) => updateMember(workspace, p.member.id, p.changes),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace] }),
    onError: (error, p) => {
      if (error instanceof ApiError && error.code === "password_confirmation_required") {
        setRetry(p);
      }
    },
  });
  const leave = useMutation({
    mutationFn: () => leaveWorkspace(self),
    onSuccess: () => {
      window.location.assign("/d");
    },
  });
  const shownError =
    change.error instanceof ApiError && change.error.code === "password_confirmation_required" ? null : change.error;

  return (
    <SettingsPage title="Members" description="Owners and admins manage the workspace; members work the inbox.">
      <Section
        title="Members"
        description={`${active} active`}
        action={
          powers.write ? (
            <div className="flex flex-wrap items-center gap-2">
              <MemberFilterSelect value={view} onChange={setView} />
              <ExportMembersButton workspace={workspace} filter={filter} />
              {permissions.includes("members:import") ? (
                <MemberImportButton workspace={workspace} hasPassword={me.hasPassword} />
              ) : null}
              <InviteButton workspace={workspace} />
            </div>
          ) : null
        }
      >
        {members.map((member) => (
          <Person
            key={member.id}
            member={member}
            powers={powers}
            timeZone={me.displayZone ?? undefined}
            onPick={setPending}
            onHelp={setHelp}
          />
        ))}
      </Section>
      {powers.write ? <PendingInvitations workspace={workspace} /> : null}
      <Failure error={shownError ?? leave.error} />
      <Section title="Your access">
        <Row label={`Leave ${me.workspace.name}`} description="You lose access to its conversations straight away.">
          <Danger onPress={() => setLeaving(true)}>Leave workspace</Danger>
        </Row>
      </Section>
      <AlertDialog
        isOpen={pending !== null}
        onOpenChange={(open) => (open ? null : setPending(null))}
        variant={pending?.changes.status === "deactivated" ? "danger" : "default"}
        title={pending?.title ?? ""}
        description={pending?.description}
        confirmLabel={pending?.confirm}
        onConfirm={() => (pending ? change.mutateAsync(pending).catch(() => undefined) : undefined)}
      />
      <SignInHelpDialogs workspace={workspace} help={help} hasPassword={me.hasPassword} onClose={() => setHelp(null)} />
      <PasswordPrompt
        isOpen={retry !== null}
        onClose={() => setRetry(null)}
        onConfirmed={() => (retry ? change.mutateAsync(retry) : Promise.resolve())}
      />
      <AlertDialog
        isOpen={leaving}
        onOpenChange={setLeaving}
        title={`Leave ${me.workspace.name}?`}
        description="You lose access straight away. Someone in the workspace has to add you again."
        confirmLabel="Leave workspace"
        onConfirm={() => leave.mutateAsync()}
      />
    </SettingsPage>
  );
}
