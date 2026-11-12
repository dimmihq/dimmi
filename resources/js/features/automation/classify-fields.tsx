/*
 * The settings of a step that sorts a conversation by topic. Its
 * categories are its paths: each has a name and, to help the AI tell
 * them apart, a description. A conversation that fits none, or that
 * meets no AI to sort it, takes the none matched path. The category
 * picked can be saved for later steps.
 */
import { Action, Danger, Field } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";

import { choicesOf } from "./choices";
import { type EditorProps, Note, Words } from "./field-parts";
import { Pick } from "./pick";
import { SaveTarget } from "./save-target";
import { text, useSet, withNewPath, withoutPath, withPathChange, withPathLabel } from "./step-settings";

const fewest = 2;
const most = 10;

const confidences = { any: "Any", medium: "Medium", high: "High" };

export function ClassifyFields({ step, onEdit }: EditorProps) {
  const set = useSet(onEdit);
  const categories = (step.paths ?? []).filter((path) => !path.outcome);

  return (
    <>
      <Section title="AI">
        <Note>
          Sorting uses the AI provider set in Agent settings. Without one, every conversation takes the None matched
          path.
        </Note>
      </Section>
      <Section title="Extra instructions" description="Added to what the AI is told about the categories.">
        <Words
          label="Extra instructions"
          value={text(step.settings, "question")}
          placeholder="Look at the customer's latest message only."
          onChange={(value) => set({ question: value.trim() ? value : undefined })}
        />
      </Section>
      {categories.map((path, index) => (
        <Section
          key={path.id}
          title={`Category ${index + 1}`}
          action={
            categories.length > fewest ? (
              <Danger onPress={() => onEdit((current) => withoutPath(current, path.id))}>Remove</Danger>
            ) : undefined
          }
        >
          <Row label="Name">
            <Field
              label={`Category ${index + 1} name`}
              value={path.label}
              width="w-40"
              onChange={(label) => onEdit((current) => withPathLabel(current, path.id, label.slice(0, 40)))}
            />
          </Row>
          <Words
            label={`Category ${index + 1} description`}
            value={path.description ?? ""}
            placeholder="What belongs here, to help the AI. Optional."
            onChange={(value) =>
              onEdit((current) =>
                withPathChange(current, path.id, { description: value.trim() ? value.slice(0, 300) : undefined }),
              )
            }
          />
        </Section>
      ))}
      <Section title="More categories" description="Names must differ from one another.">
        {categories.length < most ? (
          <Row label="Another category">
            <Action onPress={() => onEdit((current) => withNewPath(current, `Category ${categories.length + 1}`))}>
              Add category
            </Action>
          </Row>
        ) : (
          <Note>A step sorts into 10 categories at most.</Note>
        )}
      </Section>
      <Section title="After sorting">
        <Row label="Save the category as" description="Later steps can use it.">
          <SaveTarget
            label="Save the category as"
            value={text(step.settings, "save_to")}
            nothing="Don't save it"
            onChange={(target) => set({ save_to: target })}
          />
        </Row>
        <Row label="Least confidence" description="Less sure than this takes the None matched path.">
          <Pick
            label="Least confidence"
            choices={choicesOf(confidences)}
            value={text(step.settings, "min_confidence") || "any"}
            onChange={(value) => set({ min_confidence: value === "any" ? undefined : value })}
          />
        </Row>
      </Section>
    </>
  );
}
