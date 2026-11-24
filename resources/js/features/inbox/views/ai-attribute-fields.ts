/*
 * The workspace's AI attributes as fields of a saved view: each reads
 * as conversation.ai.<key>, the value the AI found on the conversation.
 * A choice picks one of its values, yes or no is a switch, a number is
 * typed as one, and text is typed as written. Every one can also ask
 * whether the AI found anything yet.
 */
import { useQuery } from "@tanstack/react-query";

import { type AIAttribute, aiAttributesQuery } from "@/lib/api/ai-attributes";

import { type FieldSpec, fieldSpecs, type ValueKind } from "./fields";

const prefix = "conversation.ai.";

/*
 * kinds are the value each attribute type takes in a view.
 */
const kinds: Record<AIAttribute["data_type"], ValueKind> = {
  text: "text",
  number: "number",
  date: "text",
  boolean: "flag",
  choice: "choice",
};

/*
 * aiFieldSpec is one attribute as a field of a view.
 */
export function aiFieldSpec(attribute: AIAttribute): FieldSpec {
  return {
    field: prefix + attribute.key,
    label: `AI: ${attribute.label}`,
    ops: [
      { op: "eq", label: "is", kind: kinds[attribute.data_type] },
      { op: "exists", label: "is known" },
    ],
    choices: attribute.options.map((option) => ({ id: option, label: option })),
  };
}

/*
 * offerAIFields puts the attributes among the fields a view offers, in
 * place of the ones offered before. Archived ones stay out.
 */
export function offerAIFields(attributes: AIAttribute[]) {
  for (let i = fieldSpecs.length - 1; i >= 0; i--) {
    if (fieldSpecs[i].field.startsWith(prefix)) {
      fieldSpecs.splice(i, 1);
    }
  }
  for (const attribute of attributes.filter((a) => a.status === "active")) {
    fieldSpecs.push(aiFieldSpec(attribute));
  }
}

/*
 * useAIViewFields reads the workspace's AI attributes and offers them.
 */
export function useAIViewFields(workspace: string) {
  const attributes = useQuery({ ...aiAttributesQuery(workspace), retry: false }).data;
  if (attributes) {
    offerAIFields(attributes);
  }
}
