/*
 * How the help center's changelog shows and mails: whether it is on, its
 * title, whether entries name their authors and take reactions, its
 * labels, and what subscribers get: the whole entry or its summary,
 * after an undo window to take an entry back. Every switch is on until
 * the team turns it off. Changes wait in one draft for the save bar at
 * the foot; leaving with them unsaved asks first. Labels are added,
 * renamed and deleted at once, each its own action.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
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
import { SaveBar } from "./save-bar";
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

/*
 * ChangelogDraft is the changelog's settings as the page shows them,
 * every switch on until the team turns it off.
 */
type ChangelogDraft = {
  changelog: boolean;
  changelog_title: string;
  changelog_authors: boolean;
  changelog_reactions: boolean;
  changelog_subscriptions: boolean;
  changelog_email: "full" | "summary";
  changelog_undo_minutes: NonNullable<HelpCenterSettings["changelog_undo_minutes"]>;
};

/*
 * savedChangelog reads the help center's saved changelog settings with
 * their defaults filled in.
 */
function savedChangelog(settings: HelpCenterSettings): ChangelogDraft {
  return {
    changelog: settings.changelog ?? true,
    changelog_title: settings.changelog_title ?? "",
    changelog_authors: settings.changelog_authors ?? true,
    changelog_reactions: settings.changelog_reactions ?? true,
    changelog_subscriptions: settings.changelog_subscriptions ?? true,
    changelog_email: settings.changelog_email ?? "full",
    changelog_undo_minutes: settings.changelog_undo_minutes ?? 0,
  };
}

/*
 * useChangelogDraft holds the settings changed but not saved yet. A
 * setting put back to its saved value drops out of the draft, the title
 * compared without the spaces around it. Save sends only what changed;
 * Discard puts the rest back. generation counts the discards and saves,
 * so the title field, which keeps its own text, starts over.
 */
function useChangelogDraft(workspace: string, center: HelpCenter | undefined) {
  const queryClient = useQueryClient();
  const [pending, setPending] = useState<Partial<ChangelogDraft>>({});
  const [generation, setGeneration] = useState(0);
  const saved = center ? savedChangelog(center.settings) : undefined;
  const clear = () => {
    setPending({});
    setGeneration((count) => count + 1);
  };
  const save = useMutation({
    mutationFn: () =>
      updateHelpCenter(workspace, center?.id ?? "", {
        settings: {
          ...pending,
          ...(pending.changelog_title === undefined ? {} : { changelog_title: pending.changelog_title.trim() }),
        },
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey });
      clear();
      toast.success("Changelog settings saved");
    },
  });
  const same = <K extends keyof ChangelogDraft>(key: K, value: ChangelogDraft[K]) =>
    saved !== undefined &&
    (typeof value === "string" ? value.trim() === String(saved[key]).trim() : saved[key] === value);

  return {
    settings: saved ? { ...saved, ...pending } : undefined,
    generation,
    dirty: Object.keys(pending).length > 0,
    save,
    set: <K extends keyof ChangelogDraft>(key: K, value: ChangelogDraft[K]) =>
      setPending(({ [key]: _, ...rest }) => (same(key, value) ? rest : { ...rest, [key]: value })),
    discard: () => {
      save.reset();
      clear();
    },
  };
}

type Draft = ReturnType<typeof useChangelogDraft>;

export function ChangelogSettings() {
  const workspace = useAccount().workspace.id;
  const { data: centers = [], isPending } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers.find((c) => c.is_default) ?? centers[0];
  const draft = useChangelogDraft(workspace, center);
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  return (
    <SettingsPage
      title="Changelog"
      description="Release notes on your help center and in subscribers' inboxes."
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error?.message}
          onSave={() => draft.save.mutate()}
          onDiscard={draft.discard}
        />
      }
    >
      {center && draft.settings ? (
        <>
          <ChangelogSection center={center} settings={draft.settings} draft={draft} />
          <LabelsSection workspace={workspace} helpCenter={center.id} />
          <EmailSection settings={draft.settings} draft={draft} />
        </>
      ) : (
        <Section title="Changelog" loading={isPending} rows={4} />
      )}
    </SettingsPage>
  );
}

function ChangelogSection({ center, settings, draft }: { center: HelpCenter; settings: ChangelogDraft; draft: Draft }) {
  return (
    <Section title="Changelog" description={`At ${center.url}/changelog and in the messenger's What's new tab.`}>
      <Row label="On" description="Show the changelog on your help center and in the messenger.">
        <Toggle
          key={`on-${settings.changelog}`}
          label="Changelog on"
          on={settings.changelog}
          onChange={(on) => draft.set("changelog", on)}
        />
      </Row>
      <Row label="Title" description="The heading of the changelog's pages and its link in the header.">
        <Field
          key={`title-${draft.generation}`}
          label="Changelog title"
          value={settings.changelog_title}
          placeholder="Changelog"
          onChange={(title) => draft.set("changelog_title", title)}
        />
      </Row>
      <Row label="Show authors" description="Name the teammates who wrote each entry.">
        <Toggle
          key={`authors-${settings.changelog_authors}`}
          label="Show authors"
          on={settings.changelog_authors}
          onChange={(on) => draft.set("changelog_authors", on)}
        />
      </Row>
      <Row label="Reactions" description="Readers react to entries with an emoji.">
        <Toggle
          key={`reactions-${settings.changelog_reactions}`}
          label="Reactions"
          on={settings.changelog_reactions}
          onChange={(on) => draft.set("changelog_reactions", on)}
        />
      </Row>
    </Section>
  );
}

function EmailSection({ settings, draft }: { settings: ChangelogDraft; draft: Draft }) {
  const email = emailChoices.find((c) => c.value === settings.changelog_email) ?? emailChoices[0];
  const undo = undoChoices.find((c) => c.value === settings.changelog_undo_minutes) ?? undoChoices[0];

  return (
    <Section title="Email" description="Entries you choose to send go to every confirmed subscriber once.">
      <Row label="Subscriptions" description="Readers can subscribe to new entries by email.">
        <Toggle
          key={`subscriptions-${settings.changelog_subscriptions}`}
          label="Subscriptions"
          on={settings.changelog_subscriptions}
          onChange={(on) => draft.set("changelog_subscriptions", on)}
        />
      </Row>
      <Row label="What the email holds" description="A summary links to the entry for the rest.">
        <Choice
          key={`email-${email.value}`}
          label="What the email holds"
          options={emailChoices.map((c) => c.label)}
          value={email.label}
          onChange={(label) => {
            const picked = emailChoices.find((c) => c.label === label);
            if (picked) {
              draft.set("changelog_email", picked.value);
            }
          }}
        />
      </Row>
      <Row label="Undo window" description="Time to take an entry back before it is mailed.">
        <Choice
          key={`undo-${undo.value}`}
          label="Undo window"
          options={undoChoices.map((c) => c.label)}
          value={undo.label}
          onChange={(label) => {
            const picked = undoChoices.find((c) => c.label === label);
            if (picked) {
              draft.set("changelog_undo_minutes", picked.value);
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
