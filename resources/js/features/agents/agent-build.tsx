/*
 * The Build canvas: the whole agent on one page, top to bottom, the way
 * a procedure's steps page reads. What still stops it from being
 * published comes first; then who it is, the guidance it follows, what
 * it may search, follow and call, where and when it answers, who takes
 * over, and the model. Every change goes to the draft at once; nothing
 * reaches customers until it is published.
 */
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Textarea } from "@/components/base/textarea/textarea";
import { Field } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { type AgentPlacement, type AgentView, agentsQuery, placementsQuery, type ScopeMode } from "@/lib/api/agents";
import { helpCentersQuery } from "@/lib/api/help-centers";
import { sourcesQuery } from "@/lib/api/knowledge-sources";

import { modelsFor, usableKeys } from "../ai/format";
import { MultiPick, Pick } from "../automation/pick";
import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";
import { AgentGuidance } from "./agent-guidance";
import { AgentMark } from "./agent-mark";
import type { AgentDraft } from "./use-agent-draft";

/* Room to spell out several picked names; the control still hugs what it says. */
const widePick = "max-w-96 shrink-0";

const prefixes: Record<keyof AgentPlacement, string> = {
  channel_connection_ids: "chn_",
  channel_room_ids: "chr_",
  inbox_ids: "inbox_",
};

/* placementOf splits picked places into the three kinds the API keeps. */
function placementOf(ids: string[]): AgentPlacement {
  const of = (key: keyof AgentPlacement) => ids.filter((id) => id.startsWith(prefixes[key]));

  return {
    channel_connection_ids: of("channel_connection_ids"),
    channel_room_ids: of("channel_room_ids"),
    inbox_ids: of("inbox_ids"),
  };
}

function Issues({ reasons }: { reasons: string[] }) {
  if (reasons.length === 0) {
    return null;
  }

  return (
    <Section title="To fix before publishing">
      <ul className="flex flex-col gap-1.5 py-3 pe-3">
        {reasons.map((reason) => (
          <li key={reason} className="text-body-2-regular text-text-error-primary first-letter:uppercase">
            {reason}.
          </li>
        ))}
      </ul>
    </Section>
  );
}

/* scoped reads a scope as the ids MultiPick shows: none picked is everything. */
function scoped(mode: ScopeMode, ids: string[]): string[] {
  return mode === "all" ? [] : ids;
}

function Scope({ draft, workspace, sources }: { draft: AgentDraft; workspace: string; sources: ChipSources }) {
  const d = draft.draft;
  const centers = useQuery(helpCentersQuery(workspace)).data ?? [];
  const knowledge = useQuery(sourcesQuery(workspace)).data ?? [];
  const k = d.knowledge_scope;
  const picked = [...k.help_center_ids, ...k.source_ids];

  return (
    <>
      <Section title="Knowledge" description="What it may search. Enforced in search, not only asked for." bare>
        <RadioGroup
          aria-label="Knowledge"
          value={k.mode}
          onChange={(mode) => draft.change({ knowledge_scope: { ...k, mode: mode as ScopeMode } })}
          className="grid grid-cols-2 gap-2"
        >
          <RadioCard value="all" title="Everything" description="Every help center and source." />
          <RadioCard value="selected" title="Only what I pick" description="Help centers and sources." />
        </RadioGroup>
      </Section>
      {k.mode === "selected" ? (
        <Section title="Sources">
          <Row label="Searches" description="Nothing else reaches its answers.">
            <MultiPick
              label="Sources"
              choices={[
                ...centers.map((c) => [c.id, c.name] as const),
                ...knowledge.map((s) => [s.id, s.name] as const),
              ]}
              value={picked}
              shown={3}
              allLabel="Nothing yet"
              empty="No help centers or sources yet"
              className={widePick}
              onChange={(ids) =>
                draft.change({
                  knowledge_scope: {
                    mode: "selected",
                    help_center_ids: ids.filter((id) => centers.some((c) => c.id === id)),
                    source_ids: ids.filter((id) => knowledge.some((s) => s.id === id)),
                  },
                })
              }
            />
          </Row>
        </Section>
      ) : null}
      <Section
        title="Procedures and actions"
        description={`What ${d.name || "it"} may follow and call. Everything else stays out of reach.`}
      >
        <Row label="Procedures">
          <MultiPick
            label="Procedures"
            choices={sources.procedures.map((p) => [p.id, p.name] as const)}
            value={scoped(d.procedure_scope.mode, d.procedure_scope.ids)}
            allLabel="All live procedures"
            empty="No live procedures yet"
            className={widePick}
            onChange={(ids) => draft.change({ procedure_scope: { mode: ids.length > 0 ? "selected" : "all", ids } })}
          />
        </Row>
        <Row label="Actions">
          <MultiPick
            label="Actions"
            choices={sources.actions.filter((a) => a.status === "published").map((a) => [a.id, a.title] as const)}
            value={scoped(d.action_scope.mode, d.action_scope.ids)}
            allLabel="All published actions"
            empty="No published actions yet"
            className={widePick}
            onChange={(ids) => draft.change({ action_scope: { mode: ids.length > 0 ? "selected" : "all", ids } })}
          />
        </Row>
      </Section>
    </>
  );
}

function WhereAndWhen({ draft, view, workspace }: { draft: AgentDraft; view: AgentView; workspace: string }) {
  const d = draft.draft;
  const places = useQuery(placementsQuery(workspace)).data ?? [];
  const others = new Map((useQuery(agentsQuery(workspace)).data ?? []).map((a) => [a.id, a.name]));
  const picked = [...d.placement.channel_connection_ids, ...d.placement.channel_room_ids, ...d.placement.inbox_ids];
  const named = (p: (typeof places)[number]) => {
    const holder = p.agent_id && p.agent_id !== draft.id ? others.get(p.agent_id) : undefined;
    const what = p.kind === "room" ? `#${p.name}` : p.name;

    return holder ? `${what} (now ${holder})` : what;
  };

  const workflows = (view.ties ?? []).filter((tie) => tie.kind === "workflow");

  return (
    <Section
      title="Where it answers"
      description={
        view.agent.is_default
          ? "It answers every conversation no other agent takes. Pick channels to keep them for it even when others are placed."
          : "New conversations here go to this agent, unless a workflow hands them to another."
      }
    >
      <Row label="Channels and inboxes">
        <MultiPick
          label="Channels and inboxes"
          choices={places.map((p) => [p.id, named(p)] as const)}
          value={picked}
          allLabel={view.agent.is_default ? "Everything else" : "Nowhere yet"}
          empty="No channels connected yet"
          className={widePick}
          onChange={(ids) => draft.change({ placement: placementOf(ids) })}
        />
      </Row>
      {workflows.length > 0 ? (
        <Row label="Workflows" description="These hand conversations to this agent.">
          <span className="flex min-w-0 flex-wrap justify-end gap-x-3 gap-y-1 py-2 text-body-2-regular">
            {workflows.map((w) => (
              <Link
                key={w.id}
                to="/d/workflows/$workflow"
                params={{ workflow: w.id }}
                className="text-text-primary underline"
              >
                {w.name}
              </Link>
            ))}
          </span>
        </Row>
      ) : null}
      <Row label="Hours">
        <Pick
          key={`hours-${draft.generation}`}
          label="Hours"
          choices={[
            ["always", "Always"],
            ["inside_office_hours", "Office hours only"],
            ["outside_office_hours", "Outside office hours"],
          ]}
          value={d.schedule}
          onChange={(schedule) => draft.change({ schedule })}
        />
      </Row>
      <Row label="Once a teammate steps in">
        <Pick
          key={`assigned-${draft.generation}`}
          label="Once a teammate steps in"
          choices={[
            ["skip_when_human_active", "Stops while they reply"],
            ["never_when_assigned", "Stops once assigned"],
            ["always", "Keeps replying"],
          ]}
          value={d.assigned_behavior}
          onChange={(assigned_behavior) => draft.change({ assigned_behavior })}
        />
      </Row>
    </Section>
  );
}

function Handover({ draft, workspace, sources }: { draft: AgentDraft; workspace: string; sources: ChipSources }) {
  const d = draft.draft;
  const agents = (useQuery(agentsQuery(workspace)).data ?? []).filter((a) => a.id !== draft.id);

  return (
    <Section title="Handover and transfers" description="Who takes over when it stops.">
      <Row label="Hand over to" description="The team that picks up its conversations.">
        <Pick
          key={`team-${draft.generation}`}
          label="Hand over to"
          choices={[["none", "Whoever the inbox assigns"], ...sources.teams.map((t) => [t.id, t.name] as const)]}
          value={d.handoff_team_id ?? "none"}
          onChange={(team) => draft.change({ handoff_team_id: team === "none" ? null : team })}
        />
      </Row>
      <Row label="May transfer to" description="Other agents. The customer is not told; your team sees it.">
        <MultiPick
          label="May transfer to"
          choices={agents.map((a) => [a.id, a.name] as const)}
          value={d.transfer_agent_ids}
          allLabel="No other agent"
          empty="No other agents yet"
          className={widePick}
          onChange={(transfer_agent_ids) => draft.change({ transfer_agent_ids })}
        />
      </Row>
    </Section>
  );
}

function Model({ draft, view }: { draft: AgentDraft; view: AgentView }) {
  const d = draft.draft;
  const choices: [string, string][] = [["default", "Workspace default"]];
  for (const key of usableKeys(view.keys)) {
    for (const model of modelsFor(key, view.catalog)) {
      choices.push([`${key.id}|${model.id}`, `${model.display_name} · ${key.name}`]);
    }
  }
  const value = d.answer_key_id && d.answer_model ? `${d.answer_key_id}|${d.answer_model}` : "default";

  return (
    <Section title="Model">
      <Row label="Answers with" description="Falls back to the workspace's fallback model when it fails.">
        <Pick
          key={`model-${draft.generation}`}
          label="Answers with"
          choices={choices}
          value={value}
          className={widePick}
          onChange={(picked) => {
            const [key, model] = picked === "default" ? [] : picked.split("|");
            draft.change({ answer_key_id: key ?? null, answer_model: model ?? null });
          }}
        />
      </Row>
    </Section>
  );
}

/*
 * Handles is what the agent is for, in a sentence or two. The agent
 * reads it as its own job, and every agent that may pass it a
 * conversation reads it to know when a request is this agent's. It is
 * the one thing the team writes about transfers; when to transfer is
 * the system's own instruction, not theirs to spell out in guidance.
 */
function Handles({ draft }: { draft: AgentDraft }) {
  const [text, setText] = useState(draft.draft.description);

  return (
    <Section
      title="What it handles"
      description="Who it is for and what they ask about. Other agents read this to know when to pass it a conversation."
    >
      <div className="py-2.5 pe-2.5">
        <Textarea
          aria-label="What it handles"
          placeholder="People who have not bought yet: prices, which plan fits, trials and demos."
          rows={2}
          autoResize
          maxRows={6}
          maxLength={400}
          value={text}
          onChange={(next) => {
            setText(next);
            draft.change({ description: next.trim() });
          }}
        />
      </div>
    </Section>
  );
}

export function AgentBuild({
  draft,
  view,
  workspace,
  sources,
}: {
  draft: AgentDraft;
  view: AgentView;
  workspace: string;
  sources: ChipSources;
}) {
  const reasons = draft.reasons.length > 0 ? draft.reasons : view.readiness_errors;

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <Issues reasons={reasons} />
      <Section title="Name" description="Shown on its replies, beside its avatar.">
        <div className="flex items-center gap-2 py-2.5 pe-2.5">
          <AgentMark agent={{ id: draft.id, name: draft.draft.name, is_default: view.agent.is_default }} />
          <Field
            key={`name-${draft.generation}`}
            label="Agent name"
            value={draft.draft.name}
            width="w-full"
            onChange={(name) => (name.trim() ? draft.change({ name: name.trim() }) : undefined)}
          />
        </div>
      </Section>
      <Handles key={`handles-${draft.generation}`} draft={draft} />
      <AgentGuidance
        key={`guidance-${draft.generation}`}
        markdown={draft.draft.guidance}
        sources={sources}
        onChange={(guidance) => draft.change({ guidance })}
      />
      <Scope draft={draft} workspace={workspace} sources={sources} />
      <WhereAndWhen draft={draft} view={view} workspace={workspace} />
      <Handover draft={draft} workspace={workspace} sources={sources} />
      <Model draft={draft} view={view} />
    </div>
  );
}
