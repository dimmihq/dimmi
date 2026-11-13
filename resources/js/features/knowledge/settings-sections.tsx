/*
 * A source's settings, each group in its own section of the source's
 * page and edited where it stands, with no separate editing dialog. Its
 * name, what it feeds, how often it syncs and its connection, the
 * account and whatever was picked from it, such as a repository and its
 * branch, all go into one draft that the source's page saves from the
 * bar at its bottom, or discards there. Saving a changed connection
 * tries it against the service and syncs again, and says so.
 */
import { Input } from "@/components/base/input/input";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Switch } from "@/components/base/switch/switch";
import { Row, Section } from "@/features/settings/layout";
import type { KnowledgeSource, SourceSchedule, SourceTarget, SourceType } from "@/lib/api/knowledge-sources";

import { scheduleLabels, setupFields, targetLabels } from "./model";
import { SchemaForm } from "./schema-form";
import type { SourceDraft } from "./source-draft";

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
