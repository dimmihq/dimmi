/*
 * The settings of each kind of step, as the inspector shows them. A
 * message holds what it says and, in an account series, who in the
 * account gets it; a wait how long or until when; a wait-for what it
 * waits for and for how long; a check its paths, each with its rules; a
 * split its variants and their shares; an AI path its instructions and
 * the paths it picks from; actions what they change. Every change goes
 * straight into the working copy.
 */
import { RiAddLine, RiDeleteBinLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { useEvents, useTopics } from "@/features/broadcasts/sources";
import { Row, Section } from "@/features/settings/layout";
import type { NodeSettings, SeriesNode, SeriesSpec } from "@/lib/api/series";
import { tagsQuery } from "@/lib/api/tags";

import { Amount, Body, Flag, Pick, Rules, Text } from "./inputs";
import { nodeLook } from "./looks";
import { isMessage, newID } from "./model";

export interface StepProps {
  workspace: string;
  node: SeriesNode;
  spec: SeriesSpec;
  account: boolean;
  editable: boolean;
  set: (settings: NodeSettings) => void;
}

const unitOptions: ["minutes" | "hours" | "days" | "weeks", string][] = [
  ["minutes", "Minutes"],
  ["hours", "Hours"],
  ["days", "Days"],
  ["weeks", "Weeks"],
];

function MessageStep({ workspace, node, account, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const patch = (change: Partial<NodeSettings>) => set({ ...s, ...change });

  return (
    <Section title="Message" description={node.kind === "slack" ? "Posts in the account's Slack channel." : undefined}>
      {node.kind === "email" || node.kind === "post" ? (
        <Row label={node.kind === "email" ? "Subject" : "Title"}>
          <Text
            label="Subject"
            value={s.subject}
            onChange={(subject) => patch({ subject })}
            width="w-64"
            disabled={!editable}
          />
        </Row>
      ) : null}
      {node.kind === "email" ? (
        <Row label="Preview line">
          <Text
            label="Preview"
            value={s.preview}
            onChange={(preview) => patch({ preview })}
            width="w-64"
            disabled={!editable}
          />
        </Row>
      ) : null}
      <div className="flex flex-col gap-2 py-3">
        <span className="text-body-2-medium text-text-secondary">Message</span>
        <Body label="Message" value={s.body} onChange={(body) => patch({ body })} rows={8} disabled={!editable} />
        <span className="text-caption-1-regular text-text-tertiary">
          {"Markdown. Use {{ contact.first_name }}, {{ company.name }} or {{ sender.first_name }}."}
        </span>
      </div>
      {node.kind === "email" ? (
        <Row label="Track clicks">
          <Flag
            label="Track clicks"
            on={s.track_clicks ?? true}
            onChange={(on) => patch({ track_clicks: on })}
            disabled={!editable}
          />
        </Row>
      ) : null}
      {account && node.kind !== "slack" ? (
        <Row label="Who in the account" description="Without rules, everyone in the account with an address.">
          <Rules
            workspace={workspace}
            title="Who in the account gets this"
            rules={s.recipients}
            onChange={(recipients) => patch({ recipients })}
            disabled={!editable}
          />
        </Row>
      ) : null}
    </Section>
  );
}

function WaitStep({ node, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const patch = (change: Partial<NodeSettings>) => set({ ...s, ...change });

  return (
    <Section title="Wait">
      <Row label="Wait">
        <Pick
          label="Wait mode"
          value={s.mode ?? "duration"}
          options={[
            ["duration", "For a while"],
            ["time", "Until a time of day"],
            ["attribute", "Until a date they have"],
          ]}
          onChange={(mode) => patch({ mode })}
          disabled={!editable}
        />
      </Row>
      {(s.mode ?? "duration") === "duration" ? (
        <Row label="For">
          <span className="flex gap-2">
            <Amount label="Amount" value={s.amount} onChange={(amount) => patch({ amount })} disabled={!editable} />
            <Pick
              label="Unit"
              value={s.unit ?? "days"}
              options={unitOptions}
              onChange={(unit) => patch({ unit })}
              disabled={!editable}
            />
          </span>
        </Row>
      ) : null}
      {s.mode === "time" ? (
        <Row label="At" description="In the person's own time zone.">
          <Text
            label="Time of day"
            value={s.at ?? "09:00"}
            onChange={(at) => patch({ at })}
            width="w-24"
            disabled={!editable}
          />
        </Row>
      ) : null}
      {s.mode === "attribute" ? (
        <>
          <Row label="Date attribute" description="Like trial_ends_at.">
            <Text
              label="Attribute"
              value={s.attribute}
              onChange={(attribute) => patch({ attribute })}
              disabled={!editable}
            />
          </Row>
          <Row label="Days from it" description="Negative is before the date.">
            <Amount
              label="Offset"
              value={s.offset_days}
              onChange={(offset_days) => patch({ offset_days })}
              disabled={!editable}
            />
          </Row>
        </>
      ) : null}
    </Section>
  );
}

function WaitForStep({ workspace, node, spec, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const patch = (change: Partial<NodeSettings>) => set({ ...s, ...change });
  const events = useEvents(workspace);
  const messages = spec.nodes.filter((n) => isMessage(n.kind));

  return (
    <Section title="Wait for" description="Takes the first path when it happens, the other when time is up.">
      <Row label="An event">
        <Pick
          label="Event"
          value={s.event || "_none"}
          options={[["_none", "No event"], ...events.map((e): [string, string] => [e.name, e.label || e.name])]}
          onChange={(event) => patch({ event: event === "_none" ? "" : event })}
          disabled={!editable}
        />
      </Row>
      <Row label="Or they match">
        <Rules
          workspace={workspace}
          title="Wait until they match"
          rules={s.rules}
          onChange={(rules) => patch({ rules })}
          disabled={!editable}
        />
      </Row>
      <Row label="Or they act on">
        <Pick
          label="Message"
          value={s.engagement?.node ?? "_none"}
          options={[
            ["_none", "No message"],
            ...messages.map((m): [string, string] => [
              m.id,
              `${nodeLook[m.kind].name}: ${m.settings?.subject || m.id}`,
            ]),
          ]}
          onChange={(id) =>
            patch({ engagement: id === "_none" ? undefined : { node: id, action: s.engagement?.action ?? "clicked" } })
          }
          disabled={!editable}
        />
      </Row>
      {s.engagement ? (
        <Row label="By">
          <Pick
            label="Action"
            value={s.engagement.action}
            options={[
              ["opened", "Opening it"],
              ["clicked", "Clicking it"],
              ["replied", "Replying"],
            ]}
            onChange={(action) => patch({ engagement: { node: s.engagement?.node ?? "", action } })}
            disabled={!editable}
          />
        </Row>
      ) : null}
      <Row label="For at most">
        <span className="flex gap-2">
          <Amount label="Amount" value={s.amount} onChange={(amount) => patch({ amount })} disabled={!editable} />
          <Pick
            label="Unit"
            value={s.unit ?? "days"}
            options={unitOptions}
            onChange={(unit) => patch({ unit })}
            disabled={!editable}
          />
        </span>
      </Row>
    </Section>
  );
}

function ConditionStep({ workspace, node, spec, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const paths = s.paths ?? [];
  const setPaths = (next: typeof paths) => set({ ...s, paths: next });

  return (
    <Section
      title="Paths"
      description="People take the first path they match, the rest go down everyone else."
      action={
        editable && paths.length < 7 ? (
          <Button
            size="xs"
            variant="secondary"
            leadingIcon={RiAddLine}
            onClick={() => setPaths([...paths, { id: newID(spec, "path"), label: `Path ${paths.length + 1}` }])}
          >
            Add path
          </Button>
        ) : null
      }
    >
      {paths.map((p, i) => (
        <Row
          key={p.id}
          label={
            <Text
              label="Path name"
              value={p.label}
              onChange={(label) => setPaths(paths.map((q, j) => (i === j ? { ...q, label } : q)))}
              width="w-40"
              disabled={!editable}
            />
          }
        >
          <span className="flex items-center gap-1">
            <Rules
              workspace={workspace}
              title={`Who goes down ${p.label}`}
              rules={p.rules}
              onChange={(rules) => setPaths(paths.map((q, j) => (i === j ? { ...q, rules } : q)))}
              disabled={!editable}
            />
            {editable && paths.length > 1 ? (
              <IconButton
                size="small"
                icon={RiDeleteBinLine}
                aria-label="Remove path"
                onClick={() => setPaths(paths.filter((_, j) => j !== i))}
              />
            ) : null}
          </span>
        </Row>
      ))}
    </Section>
  );
}

function SplitStep({ node, spec, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const variants = s.variants ?? [];
  const setVariants = (next: typeof variants) => set({ ...s, variants: next });

  return (
    <Section
      title="Variants"
      description="Each person is given one variant for good, by its share."
      action={
        editable && variants.length < 4 ? (
          <Button
            size="xs"
            variant="secondary"
            leadingIcon={RiAddLine}
            onClick={() =>
              setVariants([
                ...variants,
                { id: newID(spec, "variant"), label: String.fromCharCode(65 + variants.length), weight: 25 },
              ])
            }
          >
            Add variant
          </Button>
        ) : null
      }
    >
      {variants.map((v, i) => (
        <Row
          key={v.id}
          label={
            <Text
              label="Variant name"
              value={v.label}
              onChange={(label) => setVariants(variants.map((w, j) => (i === j ? { ...w, label } : w)))}
              width="w-32"
              disabled={!editable}
            />
          }
        >
          <span className="flex items-center gap-1">
            <Amount
              label="Share"
              value={v.weight}
              onChange={(weight) => setVariants(variants.map((w, j) => (i === j ? { ...w, weight } : w)))}
              disabled={!editable}
            />
            <span className="text-body-2-regular text-text-tertiary">%</span>
          </span>
        </Row>
      ))}
    </Section>
  );
}

function AIPathStep({ node, spec, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const paths = s.paths ?? [];
  const setPaths = (next: typeof paths) => set({ ...s, paths: next });

  return (
    <>
      <Section
        title="How the AI picks"
        description="It reads the person, their company and recent conversations. Anything going wrong takes the fallback path."
      >
        <div className="py-3">
          <Body
            label="Instructions"
            value={s.prompt}
            onChange={(prompt) => set({ ...s, prompt })}
            rows={4}
            disabled={!editable}
          />
        </div>
      </Section>
      <Section
        title="Paths"
        action={
          editable && paths.length < 7 ? (
            <Button
              size="xs"
              variant="secondary"
              leadingIcon={RiAddLine}
              onClick={() => setPaths([...paths, { id: newID(spec, "path"), label: "Another path", description: "" }])}
            >
              Add path
            </Button>
          ) : null
        }
      >
        {paths.map((p, i) => (
          <div key={p.id} className="flex flex-col gap-2 py-3">
            <Text
              label="Path name"
              value={p.label}
              onChange={(label) => setPaths(paths.map((q, j) => (i === j ? { ...q, label } : q)))}
              width="w-full"
              disabled={!editable}
            />
            <Text
              label="When to pick it"
              value={p.description}
              placeholder="When to pick it"
              onChange={(description) => setPaths(paths.map((q, j) => (i === j ? { ...q, description } : q)))}
              width="w-full"
              disabled={!editable}
            />
          </div>
        ))}
      </Section>
    </>
  );
}

function ActionStep({ workspace, node, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const tags = useQuery({ ...tagsQuery(workspace), enabled: node.kind === "tag" || node.kind === "untag" }).data ?? [];
  const topics = useTopics(workspace);

  if (node.kind === "tag" || node.kind === "untag") {
    return (
      <Section title={node.kind === "tag" ? "Add the tag" : "Remove the tag"}>
        <Row label="Tag">
          <Pick
            label="Tag"
            value={s.tag_ids?.[0]}
            options={tags.map((t): [string, string] => [t.id, t.name])}
            onChange={(id) => set({ ...s, tag_ids: [id] })}
            disabled={!editable}
          />
        </Row>
      </Section>
    );
  }
  if (node.kind === "set_attribute") {
    return (
      <Section title="Set an attribute">
        <Row label="Attribute">
          <Text label="Attribute" value={s.key} onChange={(key) => set({ ...s, key })} disabled={!editable} />
        </Row>
        <Row label="Value">
          <Text
            label="Value"
            value={typeof s.value === "string" ? s.value : JSON.stringify(s.value ?? "")}
            onChange={(value) => set({ ...s, value })}
            disabled={!editable}
          />
        </Row>
      </Section>
    );
  }
  if (node.kind === "topic") {
    return (
      <Section title="Topic">
        <Row label="Topic">
          <Pick
            label="Topic"
            value={s.topic_id}
            options={topics.map((t): [string, string] => [t.id, t.name])}
            onChange={(topic_id) => set({ ...s, topic_id })}
            disabled={!editable}
          />
        </Row>
        <Row label="Subscribe" description="Off takes them off the topic.">
          <Flag
            label="Subscribe"
            on={s.subscribed}
            onChange={(subscribed) => set({ ...s, subscribed })}
            disabled={!editable}
          />
        </Row>
      </Section>
    );
  }
  if (node.kind === "webhook") {
    return (
      <Section title="Webhook" description="Posts the person, the series and the step to your endpoint.">
        <Row label="Endpoint">
          <Text
            label="Endpoint"
            value={s.url}
            onChange={(url) => set({ ...s, url })}
            width="w-64"
            disabled={!editable}
          />
        </Row>
      </Section>
    );
  }

  return (
    <Section title="Leave">
      <Row label="People leave the series here" description="They can enter again only if the series allows it." />
    </Section>
  );
}

export function StepSettings(props: StepProps) {
  const kind = props.node.kind;
  if (isMessage(kind)) {
    return <MessageStep {...props} />;
  }
  switch (kind) {
    case "wait":
      return <WaitStep {...props} />;
    case "wait_for":
      return <WaitForStep {...props} />;
    case "condition":
      return <ConditionStep {...props} />;
    case "split":
      return <SplitStep {...props} />;
    case "ai_path":
      return <AIPathStep {...props} />;
    default:
      return <ActionStep {...props} />;
  }
}
