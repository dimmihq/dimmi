/*
 * New agent, in two steps that slide one into the other, like a new
 * workflow. First the starting point: one narrow dialog lists every
 * template group after group, each with its icon in the group's
 * colour, what it does and a chevron; searching keeps only the matches
 * and Enter picks the first. Then the name, what the template brings,
 * and whether the agent gets a widget of its own. Mockup: Create opens
 * the sample Sales agent.
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
import { useNavigate } from "@tanstack/react-router";
import { type ComponentType, useState } from "react";

import type { ChipColor } from "@/components/base/badges/chip";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { CheckboxCard } from "@/components/base/checkbox/checkbox-card";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Swap } from "@/features/inbox/motion";

import { KindMark } from "../automation/parts";
import { type Template, type TemplateGroup, templateGroups, templates } from "./mock";

const icons: Record<string, ComponentType<{ className?: string }>> = {
  support: RiCustomerService2Line,
  ecommerce: RiShoppingBag3Line,
  billing: RiHandCoinLine,
  developer: RiCodeSSlashLine,
  sales: RiUserStarLine,
  onboarding: RiRocketLine,
  internal: RiTeamLine,
  blank: RiAddLine,
};

const tints: Record<TemplateGroup, ChipColor> = {
  support: "lime",
  growth: "purple",
  internal: "blue",
  blank: "neutral",
};

function matches(t: Template, query: string): boolean {
  const q = query.toLowerCase();

  return q === "" || t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q);
}

function Choice({ template, onPick }: { template: Template; onPick: (t: Template) => void }) {
  return (
    <button
      type="button"
      onClick={() => onPick(template)}
      className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <KindMark icon={icons[template.id]} tint={tints[template.group]} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-body-regular text-text-primary">{template.name}</span>
        <span className="text-body-2-regular text-text-secondary">{template.description}</span>
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </button>
  );
}

function Picker({ onPick }: { onPick: (t: Template) => void }) {
  const [query, setQuery] = useState("");
  const q = query.trim();
  const found = templateGroups
    .map((group) => ({ ...group, items: templates.filter((t) => t.group === group.id && matches(t, q)) }))
    .filter((group) => group.items.length > 0);
  const first = found[0]?.items[0];

  return (
    <>
      <ModalHeader title="New agent" description="Each template brings guidance, procedures and simulations." />
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
        {found.length === 0 ? (
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
                    <Choice key={t.id} template={t} onPick={onPick} />
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

function Details({ template, onBack, onClose }: { template: Template; onBack: () => void; onClose: () => void }) {
  const navigate = useNavigate();
  const [name, setName] = useState(template.agentName);
  const [widget, setWidget] = useState(template.widget);

  return (
    <>
      <ModalHeader title={template.name} description={template.description} />
      <ModalBody className="flex flex-col gap-5">
        <Input label="Name" placeholder="Sales" value={name} onChange={setName} autoFocus />
        {template.kit.length > 0 ? (
          <div className="flex flex-col gap-2">
            <span className="text-body-2-medium text-text-primary">Comes with</span>
            <span className="flex flex-wrap gap-1.5">
              {template.kit.map((k) => (
                <Chip key={k} variant="caption" color="soft" className="py-0.5">
                  {k}
                </Chip>
              ))}
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
        <Button
          size="small"
          disabled={name.trim() === ""}
          onClick={() => void navigate({ to: "/d/agents/$agent", params: { agent: "agt_sales" } })}
        >
          Create agent
        </Button>
      </ModalFooter>
    </>
  );
}

export function NewAgentModal({ onClose }: { onClose: () => void }) {
  const [picked, setPicked] = useState<Template | null>(null);

  return (
    <Modal
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      size="md"
      className="h-[min(640px,calc(100dvh-3rem))]"
    >
      <Swap view={picked ? picked.id : "pick"} forward={picked !== null} className="flex min-h-0 flex-1 flex-col">
        {picked ? (
          <Details template={picked} onBack={() => setPicked(null)} onClose={onClose} />
        ) : (
          <Picker onPick={setPicked} />
        )}
      </Swap>
    </Modal>
  );
}
