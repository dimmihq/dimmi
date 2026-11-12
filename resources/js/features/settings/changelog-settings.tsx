/*
 * How the help center's changelog shows and mails: whether it is on, its
 * title, whether entries name their authors and take reactions, its
 * labels, and what subscribers get: the whole entry or its summary,
 * after an undo window to take an entry back. Every switch is on until
 * the team turns it off. Each change saves at once.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { type LabelAct, labelActions, useLabels } from "@/features/changelog/use-labels";
import { InlineName } from "@/features/help-center/inline-name";
import { RowMenu } from "@/features/shell/row-menu";
import { type ChangelogLabel, labelColors, labelsQuery } from "@/lib/api/changelog";
import { type HelpCenter, type HelpCenterSettings, helpCentersQuery, updateHelpCenter } from "@/lib/api/help-centers";

import { Action, Choice, Field, Toggle } from "./controls";
import { Empty, Item, Row, Section, SettingsPage } from "./layout";
import { useAccount } from "./use-account";

const emailChoices = [
  { value: "full", label: "The full entry" },
  { value: "summary", label: "A summary" },
] as const;

const undoChoices = [
  { value: 0, label: "None" },
  { value: 5, label: "5 minutes" },
  { value: 15, label: "15 minutes" },
  { value: 60, label: "1 hour" },
] as const;

export function ChangelogSettings() {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const { data: centers = [], isPending } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers.find((c) => c.is_default) ?? centers[0];

  const update = useMutation({
    mutationFn: (settings: HelpCenterSettings) => updateHelpCenter(workspace, center?.id ?? "", { settings }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey }),
    onError: (error) => toast.error(error.message),
  });
  const save = (settings: HelpCenterSettings) => update.mutate(settings);

  return (
    <SettingsPage title="Changelog" description="Release notes on your help center and in subscribers' inboxes.">
      {center ? (
        <>
          <ChangelogSection center={center} onSave={save} />
          <LabelsSection workspace={workspace} helpCenter={center.id} />
          <EmailSection center={center} onSave={save} />
        </>
      ) : (
        <Section title="Changelog" loading={isPending} rows={4} />
      )}
    </SettingsPage>
  );
}

function ChangelogSection({ center, onSave }: { center: HelpCenter; onSave: (s: HelpCenterSettings) => void }) {
  const s = center.settings;
  const [title, setTitle] = useState(s.changelog_title ?? "");

  return (
    <Section title="Changelog" description={`At ${center.url}/changelog and in the messenger's What's new tab.`}>
      <Row label="On" description="Show the changelog on your help center and in the messenger.">
        <Toggle label="Changelog on" on={s.changelog ?? true} onChange={(on) => onSave({ changelog: on })} />
      </Row>
      <Row label="Title" description="The heading of the changelog's pages and its link in the header.">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            onSave({ changelog_title: title.trim() });
          }}
          onBlur={() => {
            if (title.trim() !== (s.changelog_title ?? "")) {
              onSave({ changelog_title: title.trim() });
            }
          }}
        >
          <Field label="Changelog title" value={title} placeholder="Changelog" onChange={setTitle} />
        </form>
      </Row>
      <Row label="Show authors" description="Name the teammates who wrote each entry.">
        <Toggle
          label="Show authors"
          on={s.changelog_authors ?? true}
          onChange={(on) => onSave({ changelog_authors: on })}
        />
      </Row>
      <Row label="Reactions" description="Readers react to entries with an emoji.">
        <Toggle
          label="Reactions"
          on={s.changelog_reactions ?? true}
          onChange={(on) => onSave({ changelog_reactions: on })}
        />
      </Row>
    </Section>
  );
}

function EmailSection({ center, onSave }: { center: HelpCenter; onSave: (s: HelpCenterSettings) => void }) {
  const s = center.settings;
  const email = emailChoices.find((c) => c.value === (s.changelog_email ?? "full")) ?? emailChoices[0];
  const undo = undoChoices.find((c) => c.value === (s.changelog_undo_minutes ?? 0)) ?? undoChoices[0];

  return (
    <Section title="Email" description="Entries you choose to send go to every confirmed subscriber once.">
      <Row label="Subscriptions" description="Readers can subscribe to new entries by email.">
        <Toggle
          label="Subscriptions"
          on={s.changelog_subscriptions ?? true}
          onChange={(on) => onSave({ changelog_subscriptions: on })}
        />
      </Row>
      <Row label="What the email holds" description="A summary links to the entry for the rest.">
        <Choice
          label="What the email holds"
          options={emailChoices.map((c) => c.label)}
          value={email.label}
          onChange={(label) => {
            const picked = emailChoices.find((c) => c.label === label);
            if (picked) {
              onSave({ changelog_email: picked.value });
            }
          }}
        />
      </Row>
      <Row label="Undo window" description="Time to take an entry back before it is mailed.">
        <Choice
          label="Undo window"
          options={undoChoices.map((c) => c.label)}
          value={undo.label}
          onChange={(label) => {
            const picked = undoChoices.find((c) => c.label === label);
            if (picked) {
              onSave({ changelog_undo_minutes: picked.value });
            }
          }}
        />
      </Row>
    </Section>
  );
}

/*
 * LabelsSection adds, renames, recolors and deletes the changelog's
 * labels, the same way the changelog's own pane does.
 */
function LabelsSection({ workspace, helpCenter }: { workspace: string; helpCenter: string }) {
  const { data: labels = [], isPending } = useQuery(labelsQuery(workspace, helpCenter));
  const edits = useLabels(workspace, helpCenter);
  const [adding, setAdding] = useState(false);
  const [renaming, setRenaming] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<ChangelogLabel | null>(null);
  const onAct = (label: ChangelogLabel, act: LabelAct) => {
    if (act === "rename") {
      setRenaming(label.id);
    } else if (act === "color") {
      edits.recolor(label);
    } else {
      setDeleting(label);
    }
  };

  return (
    <Section
      title="Labels"
      description="Kinds of change entries are filed under, such as New or Fixed."
      loading={isPending}
      rows={3}
      action={<Action onPress={() => setAdding(true)}>New label</Action>}
    >
      {labels.map((l) => (
        <Item
          key={l.id}
          mark={<Dot color={l.color} />}
          title={
            renaming === l.id ? (
              <InlineName
                initial={l.name}
                label="Label name"
                onCommit={async (name) => {
                  await edits.rename(l.id, name);
                  setRenaming(null);
                }}
                onCancel={() => setRenaming(null)}
              />
            ) : (
              l.name
            )
          }
        >
          <RowMenu label={l.name} actions={labelActions} onPick={(act) => onAct(l, act)} />
        </Item>
      ))}
      {adding ? (
        <Item
          mark={<Dot color={labelColors[3] ?? "#a855f7"} />}
          title={
            <InlineName
              initial=""
              label="New label name"
              onCommit={async (name) => {
                await edits.add(name);
                setAdding(false);
              }}
              onCancel={() => setAdding(false)}
            />
          }
        />
      ) : null}
      {labels.length === 0 && !adding && !isPending ? <Empty>No labels yet.</Empty> : null}
      <AlertDialog
        isOpen={deleting !== null}
        onOpenChange={(open) => (open ? undefined : setDeleting(null))}
        title={deleting ? `Delete ${deleting.name}?` : "Delete label?"}
        description="Updates filed under it stay, without this label."
        confirmLabel="Delete label"
        onConfirm={async () => {
          if (deleting) {
            await edits.remove(deleting.id);
          }
        }}
      />
    </Section>
  );
}

function Dot({ color }: { color: string }) {
  return (
    <span className="grid size-8 place-items-center">
      <span aria-hidden className="size-2.5 rounded-full" style={{ background: color }} />
    </span>
  );
}
