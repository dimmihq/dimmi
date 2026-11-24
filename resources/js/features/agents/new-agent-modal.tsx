/*
 * New agent, in two steps that slide one into the other, like a new
 * workflow. First the starting point: one narrow dialog lists every
 * template group after group, each with its icon in the group's
 * colour, what it does and a chevron; searching keeps only the matches
 * and Enter picks the first. Then the name, what the template brings,
 * and whether the agent gets a messenger of its own, for a page such as
 * pricing. Create makes the agent as a draft, with its messenger placed
 * on it, and opens it.
 */
import {
  RiAddLine,
  RiArrowLeftSLine,
  RiArrowRightSLine,
  RiCodeSSlashLine,
  RiCustomerService2Line,
  RiHandCoinLine,
  RiRocketLine,
  RiSearchLine,
  RiShoppingBag3Line,
  RiTeamLine,
  RiUserStarLine,
} from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type ComponentType, useState } from "react";

import type { ChipColor } from "@/components/base/badges/chip";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { CheckboxCard } from "@/components/base/checkbox/checkbox-card";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Swap } from "@/features/inbox/motion";
import { type AgentTemplate, type AgentView, agentTemplatesQuery, createAgent, saveAgent } from "@/lib/api/agents";
import { ApiError } from "@/lib/api/client";
import { inboxesQuery } from "@/lib/api/inboxes";
import { createWidget } from "@/lib/api/widget-settings";

import { KindMark } from "../automation/parts";

type Group = AgentTemplate["group"];

const groups: { id: Group; name: string; detail: string }[] = [
  { id: "support", name: "Support", detail: "Answer customers and fix what they ask about." },
  { id: "growth", name: "Sales and onboarding", detail: "Turn visitors into customers and get them set up." },
  { id: "internal", name: "Internal", detail: "Answer your own team, from docs customers never see." },
  { id: "blank", name: "Blank", detail: "Write it yourself." },
];

const icons: Record<string, ComponentType<{ className?: string }>> = {
  support: RiCustomerService2Line,
  ecommerce: RiShoppingBag3Line,
  billing: RiHandCoinLine,
  developer: RiCodeSSlashLine,
  sales: RiUserStarLine,
  onboarding: RiRocketLine,
  internal: RiTeamLine,
};

const tints: Record<Group, ChipColor> = {
  support: "lime",
  growth: "purple",
  internal: "blue",
  blank: "neutral",
};

function matches(t: AgentTemplate, query: string): boolean {
  const q = query.toLowerCase();

  return q === "" || t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q);
}

function Choice({ template, onPick }: { template: AgentTemplate; onPick: (t: AgentTemplate) => void }) {
  return (
    <button
      type="button"
      onClick={() => onPick(template)}
      className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <KindMark icon={icons[template.key] ?? RiAddLine} tint={tints[template.group]} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-body-regular text-text-primary">{template.name}</span>
        <span className="text-body-2-regular text-text-secondary">{template.description}</span>
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </button>
  );
}

function Picker({ templates, onPick }: { templates: AgentTemplate[]; onPick: (t: AgentTemplate) => void }) {
  const [query, setQuery] = useState("");
  const q = query.trim();
  const found = groups
    .map((group) => ({ ...group, items: templates.filter((t) => t.group === group.id && matches(t, q)) }))
    .filter((group) => group.items.length > 0);
  const first = found[0]?.items[0];

  return (
    <>
      <ModalHeader title="New agent" description="Each template brings guidance written for the job." />
      <div className="shrink-0 px-6">
        <Input
          aria-label="Search templates"
          placeholder="Search templates"
          leadingIcon={RiSearchLine}
          value={query}
          onChange={setQuery}
          autoFocus
          onKeyDown={(event) => {
            if (event.key === "Enter" && first) {
              onPick(first);
            }
          }}
        />
      </div>
      <ModalBody>
        {templates.length === 0 ? null : found.length === 0 ? (
          <p className="py-8 text-center text-body-regular text-text-secondary">No template matches “{q}”</p>
        ) : (
          <div className="flex flex-col gap-5 pb-2">
            {found.map((group) => (
              <section key={group.id} aria-label={group.name} className="flex flex-col gap-2">
                <div className="flex flex-col">
                  <h3 className="text-body-2-medium text-text-primary">{group.name}</h3>
                  <p className="text-body-2-regular text-text-secondary">{group.detail}</p>
                </div>
                <div className="flex flex-col gap-0.5 rounded-2xl bg-background-secondary-default p-1">
                  {group.items.map((t) => (
                    <Choice key={t.key || "blank"} template={t} onPick={onPick} />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </ModalBody>
    </>
  );
}

function Details({
  workspace,
  template,
  onBack,
  onClose,
}: {
  workspace: string;
  template: AgentTemplate;
  onBack: () => void;
  onClose: () => void;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [name, setName] = useState(template.agent_name);
  const [problem, setProblem] = useState("");
  const [saving, setSaving] = useState(false);
  const [widget, setWidget] = useState(template.widget);
  const inbox = useQuery(inboxesQuery(workspace)).data?.[0];

  /* withWidget gives the new agent a messenger of its own, placed on it in the draft. */
  const withWidget = async (made: AgentView): Promise<AgentView> => {
    if (!widget || !inbox) {
      return made;
    }
    const messenger = await createWidget(workspace, `${made.agent.name} messenger`, inbox.id);
    void queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "widget-settings"] });

    return saveAgent(workspace, made.agent.id, {
      placement: { channel_connection_ids: [messenger.channel_connection_id] },
      lock_version: made.agent.lock_version,
    });
  };

  const create = async () => {
    setSaving(true);
    try {
      const made = await withWidget(await createAgent(workspace, name.trim(), template.key));
      void queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "agents"] });
      queryClient.setQueryData(["workspace", workspace, "agents", made.agent.id], made);
      onClose();
      void navigate({ to: "/d/agents/$agent", params: { agent: made.agent.id } });
    } catch (error) {
      if (error instanceof ApiError && error.fields.name) {
        setProblem(error.fields.name);
      } else {
        toast.error(error instanceof Error ? error.message : "The agent was not made.");
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <ModalHeader title={template.name} description={template.description} />
      <ModalBody className="flex flex-col gap-5">
        <Input
          label="Name"
          placeholder="Sales"
          value={name}
          onChange={(next) => {
            setName(next);
            setProblem("");
          }}
          isInvalid={problem !== ""}
          errorMessage={problem || undefined}
          autoFocus
          onKeyDown={(event) => {
            if (event.key === "Enter" && name.trim()) {
              void create();
            }
          }}
        />
        {template.key ? (
          <div className="flex flex-col gap-2">
            <span className="text-body-2-medium text-text-primary">Comes with</span>
            <span className="flex flex-wrap gap-1.5">
              <Chip variant="caption" color="soft" className="py-0.5">
                Guidance for {template.name.toLowerCase()}
              </Chip>
            </span>
          </div>
        ) : null}
        <CheckboxCard
          title="Make a widget for it"
          description="Its own messenger, for a pricing page or docs site."
          isSelected={widget}
          onChange={setWidget}
        />
        <p className="text-body-2-regular text-text-tertiary">
          Starts as a draft. Nothing reaches customers until you publish.
        </p>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" leadingIcon={RiArrowLeftSLine} className="me-auto" onClick={onBack}>
          Back
        </Button>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button size="small" disabled={name.trim() === ""} loading={saving} onClick={() => void create()}>
          Create agent
        </Button>
      </ModalFooter>
    </>
  );
}

export function NewAgentModal({ workspace, onClose }: { workspace: string; onClose: () => void }) {
  const [picked, setPicked] = useState<AgentTemplate | null>(null);
  const templates = useQuery(agentTemplatesQuery(workspace)).data ?? [];

  return (
    <Modal
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      size="md"
      className="h-[min(640px,calc(100dvh-3rem))]"
    >
      <Swap
        view={picked ? picked.key || "blank" : "pick"}
        forward={picked !== null}
        className="flex min-h-0 flex-1 flex-col"
      >
        {picked ? (
          <Details workspace={workspace} template={picked} onBack={() => setPicked(null)} onClose={onClose} />
        ) : (
          <Picker templates={templates} onPick={setPicked} />
        )}
      </Swap>
    </Modal>
  );
}
