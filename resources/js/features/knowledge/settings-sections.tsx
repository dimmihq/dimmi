/*
 * A source's settings, each group in its own section of the source's
 * page and edited where it stands, with no separate editing dialog. The
 * name saves when the field is left; what the source feeds and how
 * often it syncs save as soon as they change. The connection, the
 * account and whatever was picked from it, such as a repository and its
 * branch, saves with its own button, since saving tries it against the
 * service and syncs again. Each save says so.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
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

import { initialValues, missing, scheduleLabels, settingsOf, setupFields, targetLabels } from "./model";
import { SchemaForm } from "./schema-form";

/*
 * useSaveSource saves a change to the source and says it did.
 */
function useSaveSource(workspace: string, source: KnowledgeSource) {
  const client = useQueryClient();
  const [errors, setErrors] = useState<Record<string, string>>({});
  const save = useMutation({
    mutationFn: ({ input }: { input: SourceInput; done: string }) => updateSource(workspace, source.id, input),
    onSuccess: async (_, { done }) => {
      setErrors({});
      await client.invalidateQueries({ queryKey: ["workspace", workspace, "knowledge"] });
      toast.success(done);
    },
    onError: (err) => {
      setErrors(err instanceof ApiError ? err.fields : {});
      toast.error(err instanceof ApiError ? err.message : "The change could not be saved.");
    },
  });

  return { save, errors };
}

export function SettingsSections({
  workspace,
  source,
  kind,
  canManage,
}: {
  workspace: string;
  source: KnowledgeSource;
  kind: SourceType;
  canManage: boolean;
}) {
  return (
    <>
      <GeneralSection workspace={workspace} source={source} kind={kind} canManage={canManage} />
      {setupFields(kind).length > 0 || kind.integration ? (
        <ConnectionSection workspace={workspace} source={source} kind={kind} canManage={canManage} />
      ) : null}
    </>
  );
}

function GeneralSection({
  workspace,
  source,
  kind,
  canManage,
}: {
  workspace: string;
  source: KnowledgeSource;
  kind: SourceType;
  canManage: boolean;
}) {
  const { save } = useSaveSource(workspace, source);
  const [name, setName] = useState(source.name);
  const feeds = kind.targets.filter((t) => t !== "help_center");
  const toggle = (target: SourceTarget, on: boolean) => {
    const targets = on ? [...source.targets, target] : source.targets.filter((t) => t !== target);
    if (targets.length > 0) {
      save.mutate({ input: { targets }, done: on ? `${targetLabels[target]} reads it now.` : "Saved." });
    }
  };

  return (
    <Section title="General">
      <Row label="Name">
        <Input
          aria-label="Name"
          value={name}
          maxLength={80}
          isDisabled={!canManage}
          onChange={setName}
          onBlur={() => name.trim() && name !== source.name && save.mutate({ input: { name }, done: "Name saved." })}
          className="w-64 max-w-full"
        />
      </Row>
      {source.mode === "sync"
        ? feeds.map((target) => (
            <Row key={target} label={targetLabels[target]} description={feedDetail(target)}>
              <Switch
                aria-label={targetLabels[target]}
                size="sm"
                isDisabled={!canManage || (source.targets.length === 1 && source.targets.includes(target))}
                isSelected={source.targets.includes(target)}
                onChange={(on) => toggle(target, on)}
              />
            </Row>
          ))
        : null}
      {source.mode === "sync" && kind.schedules.length > 1 ? (
        <Row label="Sync">
          <SegmentedControl
            aria-label="Sync"
            selectedKeys={[source.schedule]}
            isDisabled={!canManage}
            onSelectionChange={(keys) => {
              const [key] = [...keys];
              if (key !== undefined && key !== source.schedule) {
                const schedule = String(key) as SourceSchedule;
                save.mutate({ input: { schedule }, done: `Syncs ${scheduleLabels[schedule].toLowerCase()} now.` });
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
  canManage,
}: {
  workspace: string;
  source: KnowledgeSource;
  kind: SourceType;
  canManage: boolean;
}) {
  const { save, errors } = useSaveSource(workspace, source);
  const [values, setValues] = useState(() => initialValues(kind.schema, source.settings));
  const stored = JSON.stringify(settingsOf(kind.schema, initialValues(kind.schema, source.settings)));
  const changed = JSON.stringify(settingsOf(kind.schema, values)) !== stored;

  return (
    <Section
      title="Connection"
      description="Where it reads from. Saving checks it again and syncs."
      action={
        canManage && changed ? (
          <Button
            size="small"
            loading={save.isPending}
            disabled={missing(kind.schema, values, source.secrets_set).length > 0}
            onClick={() =>
              save.mutate({ input: { settings: settingsOf(kind.schema, values) }, done: "Saved. A sync is running." })
            }
          >
            Save and sync
          </Button>
        ) : null
      }
    >
      <div className="px-3 py-3">
        <SchemaForm
          workspace={workspace}
          kind={kind}
          values={values}
          secretsSet={source.secrets_set}
          errors={errors}
          onChange={setValues}
        />
      </div>
    </Section>
  );
}
