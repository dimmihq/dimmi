/*
 * The workflows page: every workflow with its trigger, channels, how
 * often it ran and whether it is live, plus templates to start from. A
 * new workflow begins by picking its trigger.
 * Workflows run in order of this list when two share a trigger.
 */
import { RiAddLine, RiLayoutGridLine } from "@remixicon/react";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Action } from "@/features/settings/controls";
import { Item, Section } from "@/features/settings/layout";

import { KindMark, ListPage, OpenRow, StatusChip } from "./parts";
import { TriggerPicker } from "./trigger-picker";
import { tintOf, triggers } from "./triggers";
import { templates, workflows } from "./workflows";

export function WorkflowList() {
  const [picking, setPicking] = useState(false);
  const navigate = useNavigate();

  return (
    <ListPage
      title="Workflows"
      description="Route conversations, ask the right questions and decide when Dimmi takes a conversation."
      action={
        <Button size="xs" leadingIcon={RiAddLine} onClick={() => setPicking(true)}>
          New workflow
        </Button>
      }
    >
      <TriggerPicker
        isOpen={picking}
        onOpenChange={setPicking}
        onPick={(trigger) => navigate({ to: "/d/workflows/new", search: { trigger } })}
      />
      <Section title="Workflows" description="When two workflows share a trigger, the higher one runs first.">
        {workflows.map((workflow) => {
          const trigger = triggers[workflow.trigger];

          return (
            <OpenRow
              key={workflow.id}
              target={{ to: "/d/workflows/$workflow", params: { workflow: workflow.id } }}
              mark={<KindMark icon={trigger.icon} tint={tintOf(workflow.trigger)} />}
              title={workflow.name}
              detail={`${trigger.name} · ${workflow.channels} · ${workflow.runs}`}
            >
              <StatusChip status={workflow.status} />
            </OpenRow>
          );
        })}
      </Section>
      <Section title="Start from a template">
        {templates.map((template) => (
          <Item
            key={template.id}
            mark={<KindMark icon={RiLayoutGridLine} tint="neutral" />}
            title={template.name}
            detail={template.detail}
          >
            <Action size="xs">Use template</Action>
          </Item>
        ))}
      </Section>
    </ListPage>
  );
}
