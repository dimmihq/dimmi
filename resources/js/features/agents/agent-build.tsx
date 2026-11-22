/*
 * The Build canvas: the whole agent on one page, top to bottom, the way
 * a procedure's steps page reads. What still stops it from being
 * published comes first; then who it is, the guidance it follows, what
 * it may search, follow and call, where and when it answers, who takes
 * over, and the model and spend. Mockup: sample values, nothing saved.
 */
import { useState } from "react";

import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Switch } from "@/components/base/switch/switch";
import { Field } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";

import { MultiPick, Pick } from "../automation/pick";
import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";
import { AgentGuidance } from "./agent-guidance";
import { AgentMark } from "./agent-mark";
import { agents, type MockAgent } from "./mock";

/* Room to spell out several picked names; the control still hugs what it says. */
const widePick = "max-w-96 shrink-0";

function Issues({ agent }: { agent: MockAgent }) {
  if (agent.tests.passing === agent.tests.total) {
    return null;
  }

  return (
    <Section title="To fix before publishing">
      <ul className="flex flex-col gap-1.5 py-3 pe-3">
        <li className="text-body-2-regular text-text-error-primary">
          1 simulation fails on the draft: Hands an existing customer's bug to dimmi.
        </li>
      </ul>
    </Section>
  );
}

function Scope({ agent }: { agent: MockAgent }) {
  const [knowledge, setKnowledge] = useState("selected");
  const [sources, setSources] = useState(["site", "pricing", "cases"]);
  const [procedures, setProcedures] = useState(["qualify"]);
  const [actions, setActions] = useState(["book_meeting", "create_lead"]);

  return (
    <>
      <Section title="Knowledge" description="What it may search. Enforced in search, not only asked for." bare>
        <RadioGroup aria-label="Knowledge" value={knowledge} onChange={setKnowledge} className="grid grid-cols-2 gap-2">
          <RadioCard value="all" title="Everything" description="Every help center and source." />
          <RadioCard value="selected" title="Only what I pick" description="Sources, help centers or collections." />
        </RadioGroup>
      </Section>
      {knowledge === "selected" ? (
        <Section title="Sources">
          <Row label="Searches" description="Nothing else reaches its answers.">
            <MultiPick
              label="Sources"
              choices={[
                ["site", "Marketing site"],
                ["pricing", "Pricing & plans"],
                ["cases", "Case studies"],
                ["help", "Help center"],
                ["api", "API reference"],
              ]}
              value={sources}
              shown={3}
              className={widePick}
              onChange={setSources}
            />
          </Row>
        </Section>
      ) : null}
      <Section
        title="Procedures and actions"
        description={`What ${agent.name} may follow and call. Everything else stays out of reach.`}
      >
        <Row label="Procedures">
          <MultiPick
            label="Procedures"
            choices={[
              ["qualify", "Qualify & book demo"],
              ["refund", "Refund request"],
              ["reset", "Password reset"],
            ]}
            value={procedures}
            allLabel="All live procedures"
            className={widePick}
            onChange={setProcedures}
          />
        </Row>
        <Row label="Actions">
          <MultiPick
            label="Actions"
            choices={[
              ["book_meeting", "Book meeting"],
              ["create_lead", "Create lead"],
              ["refund_order", "Refund order"],
            ]}
            value={actions}
            allLabel="All published actions"
            className={widePick}
            onChange={setActions}
          />
        </Row>
      </Section>
    </>
  );
}

function WhereAndWhen() {
  const [channels, setChannels] = useState(["pricing"]);
  const [hours, setHours] = useState("always");
  const [assigned, setAssigned] = useState("stop");

  return (
    <Section
      title="Where it answers"
      description="New conversations here go to this agent, unless a workflow picks another."
    >
      <Row label="Channels and widgets">
        <MultiPick
          label="Channels and widgets"
          choices={[
            ["pricing", "Pricing page widget"],
            ["messenger", "Messenger"],
            ["email", "Support inbox"],
            ["whatsapp", "WhatsApp"],
            ["slack", "Slack #sales"],
          ]}
          value={channels}
          className={widePick}
          onChange={setChannels}
        />
      </Row>
      <Row label="Hours">
        <Pick
          label="Hours"
          choices={[
            ["always", "Always"],
            ["office", "Office hours only"],
            ["outside", "Outside office hours"],
          ]}
          value={hours}
          onChange={setHours}
        />
      </Row>
      <Row label="Once a teammate is assigned">
        <Pick
          label="Once a teammate is assigned"
          choices={[
            ["stop", "Stops replying"],
            ["keep", "Keeps replying"],
          ]}
          value={assigned}
          onChange={setAssigned}
        />
      </Row>
    </Section>
  );
}

function Handover({ agent }: { agent: MockAgent }) {
  const [team, setTeam] = useState("sales");
  const [transfers, setTransfers] = useState(["agt_support"]);

  return (
    <Section title="Handover and transfers" description="Who takes over when it stops.">
      <Row label="Hand over to" description="The team that picks up its conversations.">
        <Pick
          label="Hand over to"
          choices={[
            ["sales", "Sales team"],
            ["support", "Support team"],
          ]}
          value={team}
          onChange={setTeam}
        />
      </Row>
      <Row label="May transfer to" description="Other agents. The customer is not told; your team sees it.">
        <MultiPick
          label="May transfer to"
          choices={agents.filter((a) => a.id !== agent.id).map((a) => [a.id, a.name] as const)}
          value={transfers}
          allLabel="Any agent"
          className={widePick}
          onChange={setTransfers}
        />
      </Row>
    </Section>
  );
}

function ModelAndSpend() {
  return (
    <Section title="Model and spend">
      <Row label="Model" description="Writes its answers. Falls back to the workspace's fallback.">
        <Pick
          label="Model"
          choices={[
            ["default", "Workspace default"],
            ["sonnet", "Claude Sonnet 5.5"],
            ["haiku", "Claude Haiku 5.5"],
          ]}
          value="default"
          onChange={() => undefined}
        />
      </Row>
      <Row label="Monthly cap of its own" description="Off: it counts toward the workspace cap of $25.">
        <Switch aria-label="Monthly cap of its own" />
      </Row>
    </Section>
  );
}

export function AgentBuild({ agent, sources }: { agent: MockAgent; sources: ChipSources }) {
  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <Issues agent={agent} />
      <Section title="Name" description="Shown on its replies, beside its avatar.">
        <div className="flex items-center gap-2 py-2.5 pe-2.5">
          <button
            type="button"
            aria-label="Upload an avatar"
            className="flex shrink-0 cursor-pointer rounded-lg outline-none transition-opacity duration-150 hover:opacity-80 focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <AgentMark agent={agent} />
          </button>
          <Field label="Agent name" value={agent.name} width="w-full" />
        </div>
      </Section>
      <AgentGuidance sources={sources} />
      <Scope agent={agent} />
      <WhereAndWhen />
      <Handover agent={agent} />
      <ModelAndSpend />
    </div>
  );
}
