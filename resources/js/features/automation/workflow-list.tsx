/*
 * The workflows page: every workflow with its trigger, channels, how
 * often it ran and whether it is live, plus templates to start from.
 * Workflows run in order of this list when two share a trigger.
 */
import { RiAddLine, RiLayoutGridLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { Action } from "@/features/settings/controls";
import { Item, Section } from "@/features/settings/layout";

import { KindMark, ListPage, OpenRow, StatusChip } from "./parts";
import { triggerLook } from "./step-kinds";
import { templates, triggerLabel, workflows } from "./workflows";

export function WorkflowList() {
  return (
    <ListPage
      title="Workflows"
      description="Route conversations, ask the right questions and decide when Dimmi takes a conversation."
      action={
        <Button size="small" leadingIcon={RiAddLine}>
          New workflow
        </Button>
      }
    >
      <Section title="Workflows" description="When two workflows share a trigger, the higher one runs first.">
        {workflows.map((workflow) => {
          const look = triggerLook[workflow.trigger];

          return (
            <OpenRow
              key={workflow.id}
              target={{ to: "/d/workflows/$workflow", params: { workflow: workflow.id } }}
              mark={<KindMark icon={look.icon} tint={look.tint} />}
              title={workflow.name}
              detail={`${triggerLabel[workflow.trigger]} · ${workflow.channels} · ${workflow.runs}`}
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
            <Action>Use template</Action>
          </Item>
        ))}
      </Section>
    </ListPage>
  );
}
