/*
 * The settings of each kind of step, as the inspector shows them. A
 * message holds what it says and, in an account series, who in the
 * account gets it; a wait how long or until when; a wait-for what it
 * waits for and for how long; a check its paths, each with its rules; a
 * split its variants and their shares; an AI path its instructions and
 * the paths it picks from; actions what they change. Every change goes
 * straight into the working copy.
 */
import { RiAddLine, RiDeleteBinLine, RiQuillPenLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { useEvents, useTopics } from "@/features/broadcasts/sources";
import { Row, Section } from "@/features/settings/layout";
import type { NodeSettings, SeriesNode, SeriesSpec } from "@/lib/api/series";
import { tagsQuery } from "@/lib/api/tags";

import { Amount, Body, Field, Flag, Pick, Rules, Text } from "./inputs";
import { nodeLook } from "./looks";
import { isMessage, newID } from "./model";

export interface StepProps {
  workspace: string;
  series: string;
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

/*
 * plainLines turns the start of a Markdown message into plain text for
 * the summary.
 */
function plainLines(markdown: string | undefined): string {
  return (markdown ?? "")
    .replace(/\{\{\s*[a-z]+\.([a-z_]+)[^}]*\}\}/g, (_, key: string) => `[${key.replaceAll("_", " ")}]`)
    .replace(/[*`#>]|\]\([^)]*\)/g, "")
    .replace(/\[(?=[^\]]*$)/g, "")
    .split("\n")
    .filter((line) => line.trim() !== "")
    .slice(0, 3)
    .join(" ");
}

/*
 * Written shows what the message says, and opens the composer to write
 * it.
 */
function Written({ series, node, editable }: { series: string; node: SeriesNode; editable: boolean }) {
  const navigate = useNavigate();
  const s = node.settings ?? {};
  const compose = () => void navigate({ to: "/d/series/$series/compose/$step", params: { series, step: node.id } });
  const heading = node.kind === "chat" ? "" : s.subject;
  const body = plainLines(s.body);

  return (
    <div className="flex flex-col gap-3 py-3">
      <button
        type="button"
        onClick={compose}
        className="flex w-full cursor-pointer flex-col gap-1 rounded-xl border border-border-button-default bg-background-primary-default p-3 text-start outline-none transition-colors duration-150 hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        {node.kind === "chat" ? null : (
          <span className={heading ? "text-body-medium text-text-primary" : "text-body-medium text-text-placeholder"}>
            {heading || (node.kind === "email" ? "No subject yet" : "No title yet")}
          </span>
        )}
        {node.kind === "email" && s.preview ? (
          <span className="truncate text-body-2-regular text-text-secondary">{s.preview}</span>
        ) : null}
        <span
          className={
            body ? "line-clamp-3 text-body-2-regular text-text-secondary" : "text-body-2-regular text-text-placeholder"
          }
        >
          {body || "Nothing written yet"}
        </span>
      </button>
      <span>
        <Button
          size="small"
          variant={editable ? "primary" : "secondary"}
          leadingIcon={RiQuillPenLine}
          onClick={compose}
        >
          {editable ? "Compose" : "Read the message"}
        </Button>
      </span>
    </div>
  );
}

function MessageStep({ workspace, series, node, account, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const patch = (change: Partial<NodeSettings>) => set({ ...s, ...change });

  return (
    <Section title="Message" description={node.kind === "slack" ? "Posts in the account's Slack channel." : undefined}>
      <Written series={series} node={node} editable={editable} />
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
          <div key={p.id} className="flex flex-col gap-1 py-2">
            <Field label={`Path ${i + 1}`}>
              <Text
                label="Path name"
                value={p.label}
                placeholder="Name"
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
              {editable && paths.length > 2 ? (
                <span>
                  <Button
                    size="xs"
                    variant="ghost"
                    leadingIcon={RiDeleteBinLine}
                    onClick={() => setPaths(paths.filter((_, j) => j !== i))}
                  >
                    Remove path
                  </Button>
                </span>
              ) : null}
            </Field>
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
