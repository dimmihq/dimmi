/*
 * A series' dry simulation: one person walked through the draft or the
 * live version in a moment. Every wait is skipped and nothing is sent.
 * The teammate picks a real contact or describes a made-up person, says
 * what they would do with the messages, and sees each step with the
 * path it took, every message that would go out, the changes that would
 * be made, and whether the goal was reached. Extract and run action
 * steps take the answers the teammate sets here in place of the AI or
 * the action, and what they find, like the tags earlier steps add, is
 * read by the rules after them. Try it on me stays the way to get the
 * real messages.
 */
import { RiPlayFill } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { Input } from "@/components/base/input/input";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { KindMark } from "@/features/automation/parts";
import { card, fieldLabel, hint, words } from "@/features/automation/procedure-simulator/simulator";
import { type MockDraft, mocksOf } from "@/features/automation/studio/workflow-sim-words";
import { ScrollArea } from "@/features/shell/page";
import type { AnswerAs } from "@/lib/api/agents";
import { attributesQuery } from "@/lib/api/attributes";
import { peoplePageQuery } from "@/lib/api/customers";
import { eventsQuery } from "@/lib/api/events";
import { type NodeKind, type SeriesSpec, seriesQuery } from "@/lib/api/series";
import { type Engagement, type SeriesSimulation, simulateSeries } from "@/lib/api/workflow-simulations";
import { cx } from "@/utils/cx";

import { endReasonName, nodeLook } from "../looks";
import { summary } from "../model";
import { mockableNodes, walkedNote } from "./series-sim-words";

const travelers = [
  ["contact", "A contact"],
  ["made_up", "A made-up person"],
] as const;

const copies = [
  ["draft", "Draft"],
  ["live", "Live"],
] as const;

const engagements: [Engagement, string][] = [
  ["opened", "Opens the messages"],
  ["clicked", "Clicks a link"],
  ["replied", "Replies"],
];

/* ContactPick finds a contact by name or email. */
function ContactPick({
  workspace,
  value,
  onChange,
}: {
  workspace: string;
  value: string;
  onChange: (id: string) => void;
}) {
  const [query, setQuery] = useState("");
  const people = useQuery(peoplePageQuery(workspace, { q: query.trim() || undefined }, 1)).data?.items ?? [];

  return (
    <ComboBox
      label="Contact"
      placeholder="Search by name or email"
      size="small"
      items={people}
      inputValue={query}
      onInputChange={setQuery}
      selectedKey={value || null}
      onSelectionChange={(key) => {
        const picked = people.find((p) => p.id === key);
        if (picked) {
          setQuery(picked.name || picked.email || picked.id);
        }
        onChange(key ? String(key) : "");
      }}
    >
      {(person) => {
        const name = person.name || person.email || person.id;

        return (
          <ComboBoxItem id={person.id} textValue={name}>
            {name}
          </ComboBoxItem>
        );
      }}
    </ComboBox>
  );
}

/* MadeUp describes the made-up person: a name, an email and their attributes. */
function MadeUp({
  workspace,
  person,
  onChange,
}: {
  workspace: string;
  person: { name: string; email: string; attributes: Record<string, string> };
  onChange: (person: { name: string; email: string; attributes: Record<string, string> }) => void;
}) {
  const defined = (useQuery({ ...attributesQuery(workspace), retry: false }).data ?? []).filter(
    (a) => a.of === "contact" && a.status === "active",
  );
  const unset = defined.filter((a) => !(a.key in person.attributes));

  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-2 gap-2">
        <Input label="Name" size="small" value={person.name} onChange={(name) => onChange({ ...person, name })} />
        <Input
          label="Email"
          size="small"
          type="email"
          value={person.email}
          onChange={(email) => onChange({ ...person, email })}
        />
      </div>
      {Object.entries(person.attributes).map(([key, value]) => (
        <Input
          key={key}
          label={defined.find((a) => a.key === key)?.label ?? key}
          size="small"
          value={value}
          onChange={(next) => onChange({ ...person, attributes: { ...person.attributes, [key]: next } })}
        />
      ))}
      {unset.length > 0 ? (
        <Select
          aria-label="Add an attribute"
          size="sm"
          placeholder="Add an attribute"
          selectedKey={null}
          onSelectionChange={(key) => onChange({ ...person, attributes: { ...person.attributes, [String(key)]: "" } })}
        >
          {unset.map((a) => (
            <SelectItem key={a.key} id={a.key} textValue={a.label}>
              {a.label}
            </SelectItem>
          ))}
        </Select>
      ) : null}
    </div>
  );
}

/* Doing says what the person does along the way: what they do with the messages, and the events they do. */
function Doing({
  workspace,
  engaged,
  events,
  onEngaged,
  onEvents,
}: {
  workspace: string;
  engaged: Engagement[];
  events: string[];
  onEngaged: (engaged: Engagement[]) => void;
  onEvents: (events: string[]) => void;
}) {
  const all = useQuery({ ...eventsQuery(workspace), retry: false }).data ?? [];
  const defined = all.filter((e) => e.status === "active" && !events.includes(e.name));
  const labelOf = (name: string) => all.find((e) => e.name === name)?.label || name;

  return (
    <div className="flex flex-col gap-2.5">
      <span className={fieldLabel}>Along the way, they</span>
      <div className="flex flex-col gap-2">
        {engagements.map(([id, label]) => (
          <Checkbox
            key={id}
            size="sm"
            isSelected={engaged.includes(id)}
            onChange={(on) => onEngaged(on ? [...engaged, id] : engaged.filter((e) => e !== id))}
          >
            {label}
          </Checkbox>
        ))}
      </div>
      {events.length > 0 ? (
        <span className="flex flex-wrap gap-1">
          {events.map((name) => (
            <Chip key={name} variant="caption" color="neutral" className="py-0.5">
              {labelOf(name)}
            </Chip>
          ))}
        </span>
      ) : null}
      {defined.length > 0 ? (
        <Select
          aria-label="Add an event they do"
          size="sm"
          placeholder="Add an event they do"
          selectedKey={null}
          onSelectionChange={(key) => onEvents([...events, String(key)])}
        >
          {defined.map((e) => (
            <SelectItem key={e.name} id={e.name} textValue={e.label || e.name}>
              {e.label || e.name}
            </SelectItem>
          ))}
        </Select>
      ) : null}
    </div>
  );
}

const pathWords: Record<string, string> = {
  next: "",
  else: "everyone else",
  matched: "matched",
  timed_out: "timed out",
  failed: "could not send",
  fallback: "fallback",
};

/* WalkedStep is one step the person took, with the path it sent them down. */
function WalkedStep({ step, spec }: { step: SeriesSimulation["steps"][number]; spec?: SeriesSpec }) {
  const node = spec?.nodes.find((n) => n.id === step.node_id);
  const look = nodeLook[step.kind as NodeKind];
  const failedAction = step.kind === "run_action" && step.path === "failed";
  const path =
    step.path && step.path !== "next"
      ? failedAction
        ? "failed"
        : (pathWords[step.path] ?? String(step.detail?.label ?? step.path))
      : "";
  const note = walkedNote(step);

  return (
    <li className="flex items-center gap-3 rounded-xl px-2.5 py-2">
      {look ? <KindMark icon={look.icon} tint={look.tint} size="sm" /> : null}
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-2-medium text-text-primary">
          {node && spec ? summary(node, spec) : (look?.name ?? step.kind)}
        </span>
        {path ? <span className="text-body-2-regular text-text-secondary">Took the path {path}</span> : null}
        {note ? <span className="break-words text-body-2-regular text-text-secondary">{note}</span> : null}
      </span>
      <Chip variant="caption" color={step.skipped ? "neutral" : "soft"} className="shrink-0 py-0.5">
        {step.skipped ? "Skipped" : words(step.result.replace(/^ai:/, "AI picked "))}
      </Chip>
    </li>
  );
}

/* Walked is what the dry simulation did, step by step, with what it would send and change. */
function Walked({ result, spec }: { result: SeriesSimulation; spec?: SeriesSpec }) {
  if (!result.entered) {
    return (
      <p className="text-body-2-regular text-text-error-primary">
        They would not enter:{" "}
        {result.reason === "event_not_done" ? "they do not do the entry event" : "they do not match who enters"}.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <span className="flex flex-wrap items-center gap-2">
        <Chip
          variant="caption"
          color={result.end === "finished" || result.end === "goal" ? "lime" : "orange"}
          className="py-0.5"
        >
          {endReasonName[result.end] ?? words(result.end)}
        </Chip>
        {result.goal_reached ? (
          <Chip variant="caption" color="lime" className="py-0.5">
            Reached the goal
          </Chip>
        ) : null}
        <span className={hint}>Nothing was sent, and no wait held them.</span>
      </span>
      <ol aria-label="Steps they took" className={cx("flex flex-col p-1", card)}>
        {result.steps.map((step, index) => (
          <WalkedStep key={`${step.node_id}-${String(index)}`} step={step} spec={spec} />
        ))}
      </ol>
      {result.messages.length > 0 ? (
        <div className="flex flex-col gap-2.5">
          <span className={fieldLabel}>Would send</span>
          {result.messages.map((m) => (
            <div key={m.node_id} className="flex flex-col gap-1 rounded-xl bg-background-secondary-default px-3 py-2.5">
              <span className={hint}>
                {nodeLook[m.kind as NodeKind]?.name ?? m.kind}
                {m.to ? ` to ${m.to}` : ""}
              </span>
              {m.subject ? <span className="text-body-2-medium text-text-primary">{m.subject}</span> : null}
              <span className="line-clamp-3 whitespace-pre-wrap text-body-2-regular text-text-secondary">{m.body}</span>
            </div>
          ))}
        </div>
      ) : null}
      {result.actions.length > 0 ? (
        <div className="flex flex-col gap-1.5">
          <span className={fieldLabel}>Would change</span>
          <ul className="flex flex-col gap-1">
            {result.actions.map((a, index) => {
              const node = spec?.nodes.find((n) => n.id === a.node_id);

              return (
                <li key={`${a.node_id}-${String(index)}`} className="text-body-2-regular text-text-primary">
                  {node && spec ? summary(node, spec) : (nodeLook[a.kind as NodeKind]?.name ?? a.kind)}
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

/*
 * Answers are what extract and run action steps get in place of the AI
 * and the action. The action never runs; an extract step with no answer
 * asks the AI.
 */
function Answers({
  nodes,
  drafts,
  bad,
  onChange,
}: {
  nodes: { id: string; kind: "extract" | "run_action" }[];
  drafts: Record<string, MockDraft>;
  bad: string[];
  onChange: (drafts: Record<string, MockDraft>) => void;
}) {
  if (nodes.length === 0) {
    return null;
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col">
        <span className={fieldLabel}>Answers in place of the AI and actions</span>
        <span className={hint}>No action runs. One with no answer succeeds with nothing.</span>
      </div>
      {nodes.map(({ id, kind }) => {
        const look = nodeLook[kind as NodeKind];
        const draft = drafts[id] ?? { status: "", body: "" };
        const set = (next: Partial<MockDraft>) => onChange({ ...drafts, [id]: { ...draft, ...next } });
        const wrong = bad.includes(id);

        return (
          <div key={id} className="flex flex-col gap-2">
            <span className="flex items-center gap-2">
              {look ? <KindMark icon={look.icon} tint={look.tint} size="sm" /> : null}
              <span className="text-body-2-medium text-text-primary">{look?.name ?? kind}</span>
              <span className={hint}>{id}</span>
            </span>
            {kind === "run_action" ? (
              <Input
                label="Status"
                size="small"
                inputMode="numeric"
                placeholder="200"
                value={draft.status}
                onChange={(status) => set({ status })}
              />
            ) : null}
            <Textarea
              label={kind === "extract" ? "Values" : "Result"}
              hint={
                wrong
                  ? "Write a status from 100 to 599 and the answer in JSON."
                  : kind === "extract"
                    ? "The values it finds, by attribute. Left empty, the AI reads the person."
                    : "The action's result, by response field. A status outside 2xx takes the failed path."
              }
              isInvalid={wrong}
              placeholder={kind === "extract" ? '{"fit": "team"}' : '{"seats": 12}'}
              rows={2}
              autoResize
              maxRows={8}
              value={draft.body}
              onChange={(body) => set({ body })}
            />
          </div>
        );
      })}
    </div>
  );
}

export function SeriesSimulations({ workspace, series }: { workspace: string; series: string }) {
  const row = useQuery(seriesQuery(workspace, series)).data;
  const [who, setWho] = useState<"contact" | "made_up">("made_up");
  const [contact, setContact] = useState("");
  const [person, setPerson] = useState({ name: "", email: "", attributes: {} as Record<string, string> });
  const [engaged, setEngaged] = useState<Engagement[]>(["opened"]);
  const [events, setEvents] = useState<string[]>([]);
  const [as, setAs] = useState<AnswerAs>("draft");
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<SeriesSimulation | null>(null);
  const [drafts, setDrafts] = useState<Record<string, MockDraft>>({});
  const hasLive = Boolean(row?.live_version_id);
  const copy = hasLive ? as : "draft";
  const answering = mockableNodes(row?.spec?.nodes ?? []);
  const { mocks, bad } = mocksOf({}, drafts);
  const ready = (who === "contact" ? contact !== "" : true) && bad.length === 0;

  const run = async () => {
    setRunning(true);
    try {
      setResult(
        await simulateSeries(workspace, series, {
          version: copy,
          ...(who === "contact"
            ? { contact_id: contact }
            : {
                person: {
                  name: person.name.trim() || undefined,
                  email: person.email.trim() || undefined,
                  attributes: person.attributes,
                },
              }),
          engaged,
          events,
          action_mocks: mocks,
        }),
      );
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The simulation did not run.");
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-11 shrink-0 items-center justify-between gap-2 ps-4 pe-3">
        <p className={cx(hint, "min-w-0 truncate")}>Walk one person through it. Nothing is sent.</p>
        <Button
          variant="tertiary"
          size="xs"
          leadingIcon={RiPlayFill}
          loading={running}
          disabled={running || !ready}
          onClick={() => void run()}
        >
          Run
        </Button>
      </div>
      <ScrollArea>
        <div className="flex flex-col gap-6 px-4 pt-2 pb-10">
          {hasLive ? (
            <SegmentedControl
              aria-label="Run against"
              selectedKeys={new Set([copy])}
              onSelectionChange={(keys) => setAs([...keys][0] === "live" ? "live" : "draft")}
            >
              {copies.map(([id, label]) => (
                <SegmentedControlItem key={id} id={id}>
                  {label}
                </SegmentedControlItem>
              ))}
            </SegmentedControl>
          ) : null}
          <div className="flex flex-col gap-3">
            <SegmentedControl
              aria-label="Who travels it"
              selectedKeys={new Set([who])}
              onSelectionChange={(keys) => setWho([...keys][0] === "contact" ? "contact" : "made_up")}
            >
              {travelers.map(([id, label]) => (
                <SegmentedControlItem key={id} id={id}>
                  {label}
                </SegmentedControlItem>
              ))}
            </SegmentedControl>
            {who === "contact" ? (
              <ContactPick workspace={workspace} value={contact} onChange={setContact} />
            ) : (
              <MadeUp workspace={workspace} person={person} onChange={setPerson} />
            )}
          </div>
          <Doing workspace={workspace} engaged={engaged} events={events} onEngaged={setEngaged} onEvents={setEvents} />
          <Answers nodes={answering} drafts={drafts} bad={bad} onChange={setDrafts} />
          {result ? <Walked result={result} spec={copy === "draft" ? row?.spec : undefined} /> : null}
          <p className={hint}>
            Waits are skipped and every message stays here. To get the real messages, use Try it on me.
          </p>
        </div>
      </ScrollArea>
    </div>
  );
}
