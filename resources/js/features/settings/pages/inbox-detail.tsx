/*
 * One inbox of the workspace: its name, colour and default team, who may
 * open it, the channels that feed it, how long a reply reopens a closed
 * conversation, whether a reply takes a conversation nobody holds, how
 * new conversations are assigned, how that assignment stands right now,
 * and deleting it. Changes collect on the page and save together.
 * Deleting moves the inbox's conversations to another inbox; the default
 * inbox is never deleted. People who may not manage inboxes see every
 * setting without changing it.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { avatarColor } from "@/features/shell/avatar-color";
import {
  assignmentStatusQuery,
  deleteInbox,
  type Inbox,
  type InboxAssignment,
  type InboxChange,
  type InboxSettings,
  inboxesQuery,
  inboxQuery,
  sameAccess,
  updateInbox,
  updateInboxAccess,
} from "@/lib/api/inboxes";
import { type Team, teamsQuery } from "@/lib/api/teams";
import { membersQuery } from "@/lib/api/workspace";

import { Choice, Colour, Danger, Field, Toggle } from "../controls";
import { assignmentModes, keyOf, poolLine, queueOrders, reopenLabel, reopenWindows, usePermission } from "../inboxes";
import { Empty, Item, PageSkeleton, Row, Section, SettingsPage } from "../layout";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import { AccessSection } from "./inbox-access";
import { ChannelsSection } from "./inbox-channels";

function BackToInboxes() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "inboxes" } })}
      >
        All inboxes
      </Button>
    </div>
  );
}

/*
 * TeamPick picks one of the workspace's teams, or none.
 */
function TeamPick({
  label,
  teams,
  value,
  disabled,
  onChange,
}: {
  label: string;
  teams: Team[];
  value: string | null;
  disabled: boolean;
  onChange: (team: string | null) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      isDisabled={disabled}
      defaultSelectedKey={value ?? "none"}
      onSelectionChange={(key) => onChange(key === null || key === "none" ? null : String(key))}
      popoverClassName="w-56"
    >
      {[{ id: "none", name: "No team" }, ...teams].map((team) => (
        <SelectItem key={team.id} id={team.id} textValue={team.name}>
          {team.name}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * useDraft collects the changes made on the page into one update, the
 * settings merged as the server merges them.
 */
function useDraft() {
  const [draft, setDraft] = useState<InboxChange>({});

  return {
    draft,
    reset: () => setDraft({}),
    set: (change: Omit<InboxChange, "settings">) => setDraft((d) => ({ ...d, ...change })),
    setting: (change: Partial<Omit<InboxSettings, "assignment">>) =>
      setDraft((d) => ({ ...d, settings: { ...d.settings, ...change } })),
    rule: (change: Partial<InboxAssignment>) =>
      setDraft((d) => ({
        ...d,
        settings: { ...d.settings, assignment: { ...d.settings?.assignment, ...change } },
      })),
  };
}

type Draft = ReturnType<typeof useDraft>;

/*
 * capacityOf reads a typed capacity: a whole number, or no limit when
 * left empty.
 */
function capacityOf(typed: string): number | null {
  const n = Number.parseInt(typed, 10);

  return Number.isNaN(n) ? null : n;
}

function GeneralSection({ inbox, teams, edit, locked }: { inbox: Inbox; teams: Team[]; edit: Draft; locked: boolean }) {
  const s = inbox.settings;
  const windows = reopenWindows.includes(s.reopen_window_days)
    ? reopenWindows
    : [...reopenWindows, s.reopen_window_days];
  const labels = windows.map(reopenLabel);

  return (
    <Section title="General">
      <Row label="Name">
        <Field label="Inbox name" value={inbox.name} disabled={locked} onChange={(name) => edit.set({ name })} />
      </Row>
      <Row label="Colour" description="Marks its conversations in lists.">
        <Colour
          label="Inbox colour"
          value={s.color || "#3b82f6"}
          disabled={locked}
          onChange={(color) => edit.setting({ color })}
        />
      </Row>
      <Row label="Default team" description="New conversations go to this team.">
        <TeamPick
          label="Default team"
          teams={teams}
          value={inbox.default_team_id}
          disabled={locked}
          onChange={(team) => edit.set({ default_team_id: team })}
        />
      </Row>
      <Row label="Reopen window" description="A customer reply within this time reopens a closed conversation.">
        <Choice
          label="Reopen window"
          options={labels}
          value={reopenLabel(s.reopen_window_days)}
          disabled={locked}
          onChange={(label) => edit.setting({ reopen_window_days: windows[labels.indexOf(label)] })}
        />
      </Row>
      <Row
        label="Assign to whoever replies"
        description="The first teammate to answer an unassigned conversation takes it."
      >
        <Toggle
          label="Assign to whoever replies"
          on={s.assign_on_reply}
          disabled={locked}
          onChange={(on) => edit.setting({ assign_on_reply: on })}
        />
      </Row>
    </Section>
  );
}

function AssignmentSection({
  inbox,
  teams,
  edit,
  locked,
}: {
  inbox: Inbox;
  teams: Team[];
  edit: Draft;
  locked: boolean;
}) {
  const a = inbox.settings.assignment;

  return (
    <Section title="Automatic assignment" description="How new conversations find a teammate in the default team.">
      <Row label="Mode" description="Round robin takes turns; balanced picks whoever holds fewest.">
        <Choice
          label="Assignment mode"
          options={Object.values(assignmentModes)}
          value={assignmentModes[a.mode]}
          disabled={locked}
          onChange={(label) => edit.rule({ mode: keyOf(assignmentModes, label) })}
        />
      </Row>
      <Row label="Include away teammates" description="Away teammates still get new conversations.">
        <Toggle
          label="Include away teammates"
          on={a.include_away}
          disabled={locked}
          onChange={(on) => edit.rule({ include_away: on })}
        />
      </Row>
      <Row label="Capacity" description="Most open conversations per teammate. Empty for no limit.">
        <Field
          label="Capacity"
          value={a.default_capacity?.toString() ?? ""}
          placeholder="No limit"
          width="w-28"
          disabled={locked}
          onChange={(typed) => edit.rule({ default_capacity: capacityOf(typed) })}
        />
      </Row>
      <Row label="Overflow team" description="Tried when nobody in the default team can take one.">
        <TeamPick
          label="Overflow team"
          teams={teams}
          value={a.overflow_team_id}
          disabled={locked}
          onChange={(team) => edit.rule({ overflow_team_id: team })}
        />
      </Row>
      <Row label="Queue order" description="Which waiting conversation goes first when someone frees up.">
        <Choice
          label="Queue order"
          options={Object.values(queueOrders)}
          value={queueOrders[a.queue_order]}
          disabled={locked}
          onChange={(label) => edit.rule({ queue_order: keyOf(queueOrders, label) })}
        />
      </Row>
    </Section>
  );
}

/*
 * StatusSection shows who in the pool can take the next conversation,
 * and how many wait.
 */
function StatusSection({ inbox }: { inbox: Inbox }) {
  const workspace = useAccount().workspace.id;
  const { data: status } = useQuery(assignmentStatusQuery(workspace, inbox.id));

  if (!status) {
    return <Section title="Assignment status" loading rows={2} />;
  }

  const waiting = status.waiting.count;

  return (
    <Section
      title="Assignment status"
      description={waiting === 1 ? "1 conversation waits." : `${waiting} conversations wait.`}
    >
      {status.members.length === 0 ? <Empty>Nobody is in the pool.</Empty> : null}
      {status.members.map((m) => (
        <Item
          key={m.member.id}
          mark={<Avatar size="md" color={avatarColor(m.member.name)} initials={m.member.name.charAt(0)} />}
          title={m.member.name}
          detail={poolLine(m.open_count, m.why_not)}
        >
          {m.eligible ? (
            <Chip variant="caption" color="neutral">
              Next in line
            </Chip>
          ) : null}
        </Item>
      ))}
    </Section>
  );
}

/*
 * DeleteSection deletes the inbox after picking where its conversations
 * go. The default inbox shows why it stays instead.
 */
function DeleteSection({ inbox }: { inbox: Inbox }) {
  const workspace = useAccount().workspace.id;
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: inboxes = [] } = useQuery(inboxesQuery(workspace));
  const others = inboxes.filter((i) => i.id !== inbox.id);
  const [deleting, setDeleting] = useState(false);
  const [target, setTarget] = useState<string | null>(null);
  const moveTo = target ?? others.find((i) => i.default)?.id ?? others[0]?.id;

  if (inbox.default) {
    return (
      <Section title="Danger zone">
        <Row label="Delete inbox" description="The default inbox cannot be deleted." muted />
      </Section>
    );
  }

  return (
    <Section title="Danger zone">
      <Row label="Delete inbox" description="Its conversations and channels move to another inbox.">
        <Danger onPress={() => setDeleting(true)}>Delete inbox</Danger>
      </Row>
      <AlertDialog
        isOpen={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${inbox.name}?`}
        description="Its conversations and channels move to the inbox you pick. This cannot be undone."
        confirmLabel="Delete inbox"
        isConfirmDisabled={!moveTo}
        onConfirm={async () => {
          await deleteInbox(workspace, inbox.id, moveTo);
          await navigate({ to: "/d/settings/$section", params: { section: "inboxes" } });
          await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "inboxes"] });
        }}
      >
        <Select
          label="Move conversations to"
          size="sm"
          selectedKey={moveTo ?? null}
          onSelectionChange={(key) => setTarget(key === null ? null : String(key))}
        >
          {others.map((i) => (
            <SelectItem key={i.id} id={i.id} textValue={i.name}>
              {i.name}
            </SelectItem>
          ))}
        </Select>
      </AlertDialog>
    </Section>
  );
}

function InboxPage({ inbox, canManage }: { inbox: Inbox; canManage: boolean }) {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const { data: teams = [] } = useQuery(teamsQuery(workspace));
  const { data: members = [] } = useQuery(membersQuery(workspace));
  const { data: inboxes = [] } = useQuery(inboxesQuery(workspace));
  const edit = useDraft();
  const [access, setAccess] = useState(inbox.access);
  const accessChanged = !sameAccess(access, inbox.access);
  const save = useMutation({
    mutationFn: async () => {
      if (Object.keys(edit.draft).length > 0) {
        await updateInbox(workspace, inbox.id, edit.draft);
      }
      if (accessChanged) {
        await updateInboxAccess(workspace, inbox.id, access);
      }
    },
    onSuccess: async () => {
      edit.reset();
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "inboxes"] });
    },
  });
  const canReadSettings = usePermission(workspace, "settings:read");
  const locked = !canManage;
  const changed = Object.keys(edit.draft).length > 0 || accessChanged;

  return (
    <SettingsPage title={inbox.name} description={canManage ? undefined : "Only admins can change inboxes."}>
      <BackToInboxes />
      <GeneralSection inbox={inbox} teams={teams} edit={edit} locked={locked} />
      <AccessSection access={access} teams={teams} members={members} locked={locked} onChange={setAccess} />
      <ChannelsSection workspace={workspace} inbox={inbox} inboxes={inboxes} canManage={canManage} />
      <AssignmentSection inbox={inbox} teams={teams} edit={edit} locked={locked} />
      {canManage ? (
        <div className="flex items-center justify-end gap-3">
          {save.error ? (
            <p role="alert" className="text-body-2-regular text-text-error-primary">
              {failureText(save.error)}
            </p>
          ) : null}
          <Button size="small" loading={save.isPending} disabled={!changed} onClick={() => save.mutate()}>
            Save changes
          </Button>
        </div>
      ) : null}
      {canReadSettings ? <StatusSection inbox={inbox} /> : null}
      {canManage ? <DeleteSection inbox={inbox} /> : null}
    </SettingsPage>
  );
}

export function InboxSettingsPage({ inbox }: { inbox: string }) {
  const workspace = useAccount().workspace.id;
  const canManage = usePermission(workspace, "inboxes:manage");
  const { data, isError } = useQuery(inboxQuery(workspace, inbox));

  if (isError) {
    return (
      <SettingsPage title="Inbox not found" description="It was deleted, or it belongs to another workspace.">
        <BackToInboxes />
      </SettingsPage>
    );
  }

  if (!data) {
    return <PageSkeleton />;
  }

  return <InboxPage key={data.updated_at} inbox={data} canManage={canManage} />;
}
