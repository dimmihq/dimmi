/*
 * The settings of a step that sorts a conversation by topic. Its
 * categories are its paths: each has a name and, to help the AI tell
 * them apart, a description. A conversation that fits none, or that
 * meets no AI to sort it, takes the none matched path. The step has a
 * name, required here, under which later steps read the category it
 * picked, such as {{ run.vars.topic }}; a new step starts with topic,
 * or topic_2 when another sort step has it. It sorts only when the AI
 * is fairly sure, set when the step is made and not shown here.
 */
import { Input } from "@/components/base/input/input";
import { Action, Danger, Field } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";

import { type EditorProps, Note, Words } from "./field-parts";
import { text, useSet, withNewPath, withoutPath, withPathChange, withPathLabel } from "./step-settings";

const fewest = 2;

const prefix = "run.vars.";

/*
 * keyOf writes a name the way a variable keeps it: lowercase, words
 * joined by underscores, starting with a letter.
 */
function keyOf(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9_]+/g, "_")
    .replace(/^[^a-z]+/, "")
    .slice(0, 64);
}
const most = 10;

export function ClassifyFields({ step, onEdit }: EditorProps) {
  const set = useSet(onEdit);
  const categories = (step.paths ?? []).filter((path) => !path.outcome);

  const saved = text(step.settings, "save_to");
  const name = saved.startsWith(prefix) ? saved.slice(prefix.length) : saved.split(".").pop() || "";

  return (
    <>
      <Section title="Name" description="Later steps use the topic it picks by this name.">
        <Row label="Name">
          <Input
            aria-label="Name"
            size="small"
            className="w-40"
            value={name}
            placeholder="topic"
            isInvalid={name === ""}
            hint={name === "" ? "Give it a name." : undefined}
            onChange={(value) => set({ save_to: keyOf(value) ? `${prefix}${keyOf(value)}` : "" })}
          />
        </Row>
      </Section>
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
    </>
  );
}
