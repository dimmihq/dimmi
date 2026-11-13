/*
 * A source's settings, each group in its own section of the source's
 * page and edited where it stands, with no separate editing dialog. Its
 * name, what it feeds, how often it syncs and its connection, the
 * account and whatever was picked from it, such as a repository and its
 * branch, all go into one draft that the source's page saves from the
 * bar at its bottom, or discards there. Saving a changed connection
 * tries it against the service and syncs again, and says so.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Input } from "@/components/base/input/input";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Switch } from "@/components/base/switch/switch";
import { toast } from "@/components/base/toast/toast";
import { Row, Section } from "@/features/settings/layout";
import { ApiError } from "@/lib/api/client";
import {
  type KnowledgeSource,
  type SourceInput,
  type SourceSchedule,
  type SourceTarget,
  type SourceType,
  updateSource,
} from "@/lib/api/knowledge-sources";

import {
  type FormValues,
  initialValues,
  missing,
  scheduleLabels,
  settingsOf,
  setupFields,
  targetLabels,
} from "./model";
import { SchemaForm } from "./schema-form";

/*
 * Pending holds the settings changed on the page but not saved yet;
 * whatever it leaves out stays as the source holds it.
 */
interface Pending {
  name?: string;
  targets?: SourceTarget[];
  schedule?: SourceSchedule;
  values?: FormValues;
}

/*
 * useSourceDraft holds the source's settings changed but not saved yet.
 * It is called before the source has loaded, so it takes the source and
 * its kind when they are there. Save sends only what differs from the
 * source, then reads it again; Discard puts every setting back. A blank
 * name, or a connection missing a required setting, cannot be saved.
 */
export function useSourceDraft(workspace: string, source?: KnowledgeSource, kind?: SourceType) {
  const client = useQueryClient();
  const [pending, setPending] = useState<Pending>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const stored = source && kind ? initialValues(kind.schema, source.settings) : {};
  const name = pending.name ?? source?.name ?? "";
  const targets = pending.targets ?? source?.targets ?? [];
  const schedule = pending.schedule ?? source?.schedule ?? "manual";
  const values = pending.values ?? stored;
  const input = source && kind ? inputOf(source, kind, { name, targets, schedule, values }) : {};
  const dirty = Object.keys(input).length > 0;
  const canSave =
    dirty &&
    name.trim() !== "" &&
    (input.settings === undefined || !kind || missing(kind.schema, values, source?.secrets_set).length === 0);
  const save = useMutation({
    mutationFn: () => updateSource(workspace, source?.id ?? "", input),
    onSuccess: async () => {
      const synced = input.settings !== undefined;
      setErrors({});
      await client.invalidateQueries({ queryKey: ["workspace", workspace, "knowledge"] });
      setPending({});
      toast.success(synced ? "Saved. A sync is running." : "Saved.");
    },
    onError: (err) => setErrors(err instanceof ApiError ? err.fields : {}),
  });

  return {
    name,
    targets,
    schedule,
    values,
    errors,
    dirty,
    canSave,
    save,
    setName: (next: string) => setPending((p) => ({ ...p, name: next })),
    setTarget: (target: SourceTarget, on: boolean) =>
      setPending((p) => {
        const now = p.targets ?? source?.targets ?? [];
        const next = on ? [...now.filter((t) => t !== target), target] : now.filter((t) => t !== target);

        return next.length > 0 ? { ...p, targets: next } : p;
      }),
    setSchedule: (next: SourceSchedule) => setPending((p) => ({ ...p, schedule: next })),
    setValues: (next: FormValues) => setPending((p) => ({ ...p, values: next })),
    discard: () => {
      setPending({});
      setErrors({});
      save.reset();
    },
  };
}

export type SourceDraft = ReturnType<typeof useSourceDraft>;

/*
 * inputOf is the change a save sends: only the settings that differ
 * from what the source holds.
 */
function inputOf(source: KnowledgeSource, kind: SourceType, shown: Required<Pending>): SourceInput {
  const input: SourceInput = {};
  if (shown.name !== source.name) {
    input.name = shown.name.trim();
  }
  if (!sameTargets(shown.targets, source.targets)) {
    input.targets = shown.targets;
  }
  if (shown.schedule !== source.schedule) {
    input.schedule = shown.schedule;
  }
  const settings = settingsOf(kind.schema, shown.values);
  if (
    JSON.stringify(settings) !== JSON.stringify(settingsOf(kind.schema, initialValues(kind.schema, source.settings)))
  ) {
    input.settings = settings;
  }

  return input;
}

function sameTargets(a: SourceTarget[], b: SourceTarget[]): boolean {
  return a.length === b.length && a.every((t) => b.includes(t));
}

export function SettingsSections({
  workspace,
  source,
  kind,
  canManage,
  draft,
}: {
  workspace: string;
  source: KnowledgeSource;
  kind: SourceType;
  canManage: boolean;
  draft: SourceDraft;
}) {
  return (
    <>
      <GeneralSection source={source} kind={kind} canManage={canManage} draft={draft} />
      {setupFields(kind).length > 0 || kind.integration ? (
        <ConnectionSection workspace={workspace} source={source} kind={kind} draft={draft} />
      ) : null}
    </>
  );
}

function GeneralSection({
  source,
  kind,
  canManage,
  draft,
}: {
  source: KnowledgeSource;
  kind: SourceType;
  canManage: boolean;
  draft: SourceDraft;
}) {
  const feeds = kind.targets.filter((t) => t !== "help_center");

  return (
    <Section title="General">
      <Row label="Name">
        <Input
          aria-label="Name"
          value={draft.name}
          maxLength={80}
          isDisabled={!canManage}
          onChange={draft.setName}
          className="w-64 max-w-full"
        />
      </Row>
      {source.mode === "sync"
        ? feeds.map((target) => (
            <Row key={target} label={targetLabels[target]} description={feedDetail(target)}>
              <Switch
                aria-label={targetLabels[target]}
                size="sm"
                isDisabled={!canManage || (draft.targets.length === 1 && draft.targets.includes(target))}
                isSelected={draft.targets.includes(target)}
                onChange={(on) => draft.setTarget(target, on)}
              />
            </Row>
          ))
        : null}
      {source.mode === "sync" && kind.schedules.length > 1 ? (
        <Row label="Sync">
          <SegmentedControl
            aria-label="Sync"
            selectedKeys={[draft.schedule]}
            isDisabled={!canManage}
            onSelectionChange={(keys) => {
              const [key] = [...keys];
              if (key !== undefined) {
                draft.setSchedule(String(key) as SourceSchedule);
              }
            }}
          >
            {kind.schedules.map((s) => (
              <SegmentedControlItem key={s} id={s}>
                {scheduleLabels[s]}
              </SegmentedControlItem>
            ))}
          </SegmentedControl>
        </Row>
      ) : null}
    </Section>
  );
}

function feedDetail(target: SourceTarget): string {
  return target === "agent" ? "dimmi answers customers from it." : "Operator uses it to help your team.";
}

function ConnectionSection({
  workspace,
  source,
  kind,
  draft,
}: {
  workspace: string;
  source: KnowledgeSource;
  kind: SourceType;
  draft: SourceDraft;
}) {
  return (
    <Section title="Connection" description="Where it reads from. Saving checks it again and syncs.">
      <div className="px-3 py-3">
        <SchemaForm
          workspace={workspace}
          kind={kind}
          values={draft.values}
          secretsSet={source.secrets_set}
          errors={draft.errors}
          onChange={draft.setValues}
        />
      </div>
    </Section>
  );
}
