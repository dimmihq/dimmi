/*
 * The settings of a step that runs another workflow: which one, picked
 * from the workspace's live workflows made to run from other workflows.
 * The workflow carries on once that one is done.
 */
import { Row, Section } from "@/features/settings/layout";

import { type EditorProps, Note } from "./field-parts";
import { Pick } from "./pick";
import { text, useSet } from "./step-settings";

export function RunWorkflowFields({ step, lists, onEdit }: EditorProps) {
  const set = useSet(onEdit);

  return (
    <Section title="Workflow to run" description="This workflow carries on once that one is done.">
      {lists.workflows.length === 0 ? (
        <Note>Create a workflow with the trigger Run from other workflows and publish it.</Note>
      ) : (
        <Row label="Run">
          <Pick
            key={`workflows-${lists.workflows.length}`}
            label="Workflow to run"
            choices={lists.workflows}
            value={text(step.settings, "workflow_id")}
            onChange={(id) => set({ workflow_id: id })}
          />
        </Row>
      )}
    </Section>
  );
}
