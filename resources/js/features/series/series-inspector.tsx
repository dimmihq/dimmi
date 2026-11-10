/*
 * The inspector beside the series canvas: the settings of the selected
 * step, laid out like the workflow inspector. Who enters holds the
 * series' own settings: the audience, whether people already matching
 * join or only people who start matching from now, whether someone can
 * go through it twice, the goal, the rules that take people out early,
 * and the hours messages may go out in. A message step holds its
 * content and how long it waits for the person to come by; a check
 * holds its question and how long it keeps asking. With results on, the
 * step's numbers unfold on top; picking another step brings its settings
 * in with a short rise.
 */
import { RiCloseLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { MessagePreview } from "@/features/broadcasts/parts";
import type { Broadcast, Channel } from "@/features/broadcasts/sample";
import { Choice, Field, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { formatNumber } from "@/lib/i18n/format";

import { messageKinds, nodeLook } from "./looks";
import { Rise, Unfold } from "./motion";
import { percent, type SeriesNode, type SeriesRow } from "./sample";

function EntrySettings({ series }: { series: SeriesRow }) {
  return (
    <>
      <Section title="Who enters" description="People join the moment they match.">
        <Row label="Audience" description={series.entry}>
          <Choice label="Audience" options={["New signups", "Trial users", "Active customers", "Low NPS answers"]} />
        </Row>
        <Row label="People already matching" description="Off: only people who start matching from now on.">
          <Toggle label="People already matching" />
        </Row>
        <Row label="Can enter again" description="After leaving, a person can go through it once more.">
          <Toggle label="Can enter again" />
        </Row>
      </Section>
      <Section title="Goal" description="Reaching it takes a person out of the series.">
        <Row label="Counts when a person">
          <Choice label="Goal" options={["Installs the messenger", "Starts a paid plan", "Turns the agent on"]} />
        </Row>
        <Row label="Within">
          <Choice label="Goal window" options={["14 days", "7 days", "30 days"]} />
        </Row>
      </Section>
      <Section title="Leave early" description="Anyone who matches leaves at once, wherever they are.">
        <Row label="When a person">
          <Choice label="Exit rule" options={["Unsubscribes", "Becomes a customer", "Is deleted", "No exit rule"]} />
        </Row>
      </Section>
      <Section title="Sending hours">
        <Row label="Messages go out" description="Waiting messages leave at the next allowed hour.">
          <Choice label="Messages go out" options={["Office hours", "Any time", "Weekdays, 9:00–18:00 their time"]} />
        </Row>
        <Row label="At most" description="Across every series and broadcast.">
          <Choice label="Frequency cap" options={["1 message a day", "2 messages a day", "No limit"]} />
        </Row>
      </Section>
    </>
  );
}

function previewOf(node: SeriesNode): Broadcast {
  return {
    id: node.id,
    name: node.summary,
    channel: node.kind as Channel,
    status: "draft",
    audience: "",
    audienceSize: 0,
    subject: node.subject ?? node.summary,
    preview: "",
    body: node.body?.join("\n\n") ?? node.summary,
    sender: "Wren Tester",
    when: "",
    edited: "",
  };
}

function MessageSettings({ node }: { node: SeriesNode }) {
  return (
    <>
      <Section title="Message">
        {node.kind === "chat" ? null : (
          <Row label={node.kind === "email" ? "Subject" : "Title"}>
            <Field label="Subject" value={node.subject ?? node.summary} />
          </Row>
        )}
        <Row label="Sent as">
          <Choice label="Sent as" options={["Wren Tester", "The Acme team"]} />
        </Row>
        {node.kind === "email" ? null : (
          <Row label="Wait for them to come by" description="If they don't, they move on without it.">
            <Choice label="Wait for them" options={["3 days", "1 day", "7 days", "30 days"]} />
          </Row>
        )}
      </Section>
      <MessagePreview broadcast={previewOf(node)} />
    </>
  );
}

function StepSettings({ node }: { node: SeriesNode }): ReactNode {
  if (messageKinds.includes(node.kind)) {
    return <MessageSettings node={node} />;
  }
  if (node.kind === "wait") {
    return (
      <Section title="Wait">
        <Row label="For">
          <Choice label="Wait for" options={[node.summary.replace("Wait ", ""), "1 day", "1 week", "Until a date"]} />
        </Row>
      </Section>
    );
  }
  if (node.kind === "branch") {
    return (
      <Section title="Check" description="People who match go down Yes, the rest down No.">
        <Row label="Question" description={node.summary}>
          <Choice label="Attribute" options={["Messenger installed", "Plan", "Seats used"]} />
        </Row>
        <Row label="Keep checking for" description="Someone who matches during this time goes down Yes.">
          <Choice label="Keep checking for" options={["Only once", "1 day", "3 days", "7 days"]} />
        </Row>
      </Section>
    );
  }
  if (node.kind === "tag") {
    return (
      <Section title="Tag">
        <Row label="Add the tag">
          <Choice label="Tag" options={["activated", "needs-setup-call", "vip"]} />
        </Row>
      </Section>
    );
  }
  if (node.kind === "webhook") {
    return (
      <Section title="Webhook" description="Posts the person and the series to your endpoint.">
        <Row label="Endpoint">
          <Field label="Endpoint" value="https://crm.acme.com/hooks/dimmi" width="w-64" />
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

function StepResults({ node, series }: { node: SeriesNode; series: SeriesRow }) {
  const stats = node.stats;
  if (!stats) {
    return null;
  }
  const rows: [string, string][] = [["Reached this step", formatNumber(stats.reached)]];
  if (node.kind === "entry") {
    rows.push(["In the series now", formatNumber(series.active)]);
    rows.push(["Reached the goal", `${formatNumber(series.goal)} · ${percent(series.goal, series.entered)}`]);
  }
  if (stats.sent !== undefined) {
    rows.push(["Sent", formatNumber(stats.sent)]);
    rows.push(["Opened", percent(stats.opened ?? 0, stats.sent)]);
  }
  if (stats.clicked !== undefined && stats.sent !== undefined) {
    rows.push(["Clicked", percent(stats.clicked, stats.sent)]);
  }
  if (stats.replied !== undefined && stats.sent !== undefined) {
    rows.push(["Replied", percent(stats.replied, stats.sent)]);
  }
  if (stats.waiting !== undefined) {
    rows.push(["Waiting here now", formatNumber(stats.waiting)]);
  }

  return (
    <Section title="Results" description="Since it went live.">
      {rows.map(([label, value]) => (
        <Row key={label} label={label}>
          <span className="text-body-2-medium text-text-primary tabular-nums">{value}</span>
        </Row>
      ))}
    </Section>
  );
}

export function SeriesInspector({
  node,
  series,
  results,
  onClose,
  className,
}: {
  node: SeriesNode;
  series: SeriesRow;
  results: boolean;
  onClose?: () => void;
  className?: string;
}) {
  return (
    <Panel label="Step settings" ground className={className}>
      <PageHeader title={nodeLook[node.kind].name} level={2}>
        {onClose ? (
          <IconButton size="small" icon={RiCloseLine} aria-label="Close step settings" onClick={onClose} />
        ) : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <Rise view={node.id} className="flex min-w-0 flex-col pt-2">
          <Unfold shown={results}>
            <div className="pb-6">
              <StepResults node={node} series={series} />
            </div>
          </Unfold>
          <div className="flex min-w-0 flex-col gap-6">
            {node.kind === "entry" ? <EntrySettings series={series} /> : <StepSettings node={node} />}
          </div>
        </Rise>
      </ScrollArea>
    </Panel>
  );
}
