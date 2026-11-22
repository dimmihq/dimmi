/*
 * The agents page: every agent the workspace runs, the default one
 * first, with where each answers, how it did in the last 30 days and
 * how its tests went. New agent opens the template picker. Mockup: the
 * rows come from sample data.
 */
import { RiAddLine } from "@remixicon/react";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Section } from "@/features/settings/layout";

import { ListPage, OpenRow, StatusChip } from "../automation/parts";
import { AgentMark } from "./agent-mark";
import { agents, type MockAgent } from "./mock";
import { NewAgentModal } from "./new-agent-modal";

function detailOf(a: MockAgent): string {
  const where = a.where.length === 0 ? "Not placed on a channel yet" : a.where.join(", ");
  if (a.resolved === null) {
    return `${a.role} · ${where}`;
  }

  return `${a.role} · ${where} · ${a.resolved}% resolved, ${a.conversations} conversations in 30 days`;
}

/*
 * SimulationsChip says how many simulations passed on their last run,
 * green when all pass and red when one fails, as procedures say it.
 */
export function SimulationsChip({ tests }: { tests: MockAgent["tests"] }) {
  if (!tests.ran) {
    return null;
  }
  const color = tests.passing === tests.total ? "lime" : "rose";

  return (
    <Chip variant="caption" color={color} className="py-0.5 tabular-nums">
      {tests.passing}/{tests.total}
    </Chip>
  );
}

export function AgentList() {
  const [creating, setCreating] = useState(false);
  const [first, ...rest] = agents;

  return (
    <ListPage
      title="Agents"
      description="Each agent answers its own channels with its own guidance, knowledge and tools."
      action={
        <Button size="small" leadingIcon={RiAddLine} onClick={() => setCreating(true)}>
          New agent
        </Button>
      }
    >
      <Section
        title="Default agent"
        description="Answers every conversation no other agent takes. It cannot be deleted."
      >
        <AgentRow agent={first} />
      </Section>
      <Section title="Other agents" description="Unlimited on every plan.">
        {rest.map((a) => (
          <AgentRow key={a.id} agent={a} />
        ))}
      </Section>
      {creating ? <NewAgentModal onClose={() => setCreating(false)} /> : null}
    </ListPage>
  );
}

function AgentRow({ agent }: { agent: MockAgent }) {
  return (
    <OpenRow
      target={{ to: "/d/agents/$agent", params: { agent: agent.id } }}
      mark={<AgentMark agent={agent} />}
      title={agent.name}
      detail={detailOf(agent)}
    >
      {agent.status === "live" && agent.unpublished ? (
        <Chip variant="caption" color="yellow" className="py-0.5">
          Unpublished changes
        </Chip>
      ) : null}
      <SimulationsChip tests={agent.tests} />
      <StatusChip status={agent.status} />
    </OpenRow>
  );
}
