/*
 * The agents page: every agent the workspace runs, the default one
 * first, with its role and where it answers. New agent opens the
 * template picker.
 */
import { RiAddLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Section } from "@/features/settings/layout";
import { type Agent, agentsQuery, type PlacementTarget, placementsQuery } from "@/lib/api/agents";
import type { TestResult } from "@/lib/api/procedures";

import { ListPage, OpenRow, StatusChip } from "../automation/parts";
import { AgentMark } from "./agent-mark";
import { NewAgentModal } from "./new-agent-modal";

const roles: Record<string, string> = {
  support: "Customer support",
  ecommerce: "Orders and returns",
  billing: "Billing and retention",
  developer: "Developer docs",
  sales: "Sales",
  onboarding: "Onboarding",
  internal: "Internal helpdesk",
  custom: "Custom",
};

function roleOf(role: string): string {
  return roles[role] ?? "Custom";
}

/* whereOf names where an agent's live version takes new conversations. */
function whereOf(agent: Agent, places: PlacementTarget[]): string {
  const names = places.filter((p) => p.agent_id === agent.id).map((p) => (p.kind === "room" ? `#${p.name}` : p.name));
  if (agent.is_default) {
    return names.length > 0 ? `${names.join(", ")}, and everything else` : "Everything no other agent takes";
  }

  return names.length > 0 ? names.join(", ") : "Not placed on a channel yet";
}

/*
 * SimulationsChip says how many simulations passed on their last run,
 * green when all pass and red when one fails, as procedures say it.
 */
export function SimulationsChip({ results }: { results: (TestResult | null)[] }) {
  const ran = results.filter((r): r is TestResult => r !== null);
  if (ran.length === 0) {
    return null;
  }
  const passing = ran.filter((r) => r.passed).length;

  return (
    <Chip variant="caption" color={passing === results.length ? "lime" : "rose"} className="py-0.5 tabular-nums">
      {passing}/{results.length}
    </Chip>
  );
}

function AgentRow({ agent, places }: { agent: Agent; places: PlacementTarget[] }) {
  return (
    <OpenRow
      target={{ to: "/d/agents/$agent", params: { agent: agent.id } }}
      mark={<AgentMark agent={agent} />}
      title={agent.name}
      detail={`${roleOf(agent.role)} · ${whereOf(agent, places)}`}
    >
      {agent.status !== "draft" && agent.has_unpublished_changes ? (
        <Chip variant="caption" color="yellow" className="py-0.5">
          Unpublished changes
        </Chip>
      ) : null}
      <StatusChip status={agent.status} />
    </OpenRow>
  );
}

export function AgentList({ workspace }: { workspace: string }) {
  const [creating, setCreating] = useState(false);
  const { data: agents, isPending } = useQuery(agentsQuery(workspace));
  const places = useQuery(placementsQuery(workspace)).data ?? [];
  const first = agents?.find((a) => a.is_default);
  const rest = (agents ?? []).filter((a) => !a.is_default);

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
        loading={isPending}
        rows={1}
      >
        {first ? <AgentRow agent={first} places={places} /> : null}
      </Section>
      <Section title="Other agents" description="Unlimited on every plan." loading={isPending} rows={2}>
        {rest.length === 0 ? (
          <p className="px-3 py-3 text-body-2-regular text-text-secondary">
            No other agents yet. Make one for sales, onboarding or your own team.
          </p>
        ) : (
          rest.map((a) => <AgentRow key={a.id} agent={a} places={places} />)
        )}
      </Section>
      {creating ? <NewAgentModal workspace={workspace} onClose={() => setCreating(false)} /> : null}
    </ListPage>
  );
}
