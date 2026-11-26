/*
 * The workspace's conversation attributes as fields of a saved view:
 * each reads as conversation.ai.<key>, the value a teammate or the AI
 * set on the conversation. A list picks one of its values, yes or no is
 * a switch, a number is typed as one, and text is typed as written.
 * Multi-selects, files, contacts and companies only ask whether a value
 * is set, as every attribute can.
 */
import { useQuery } from "@tanstack/react-query";

import { type ConversationAttribute, conversationAttributesQuery } from "@/lib/api/conversation-attributes";

import { type FieldSpec, fieldSpecs, type ValueKind } from "./fields";

const prefix = "conversation.ai.";

/*
 * kinds are the value each attribute type takes in a view.
 */
const kinds: Partial<Record<ConversationAttribute["data_type"], ValueKind>> = {
  text: "text",
  long_text: "text",
  number: "number",
  decimal: "number",
  date: "text",
  datetime: "text",
  boolean: "flag",
  choice: "choice",
};

/*
 * aiFieldSpec is one attribute as a field of a view.
 */
export function aiFieldSpec(attribute: ConversationAttribute): FieldSpec {
  const kind = kinds[attribute.data_type];

  return {
    field: prefix + attribute.key,
    label: attribute.label,
    ops: [...(kind ? [{ op: "eq", label: "is", kind }] : []), { op: "exists", label: "is set" }],
    choices: attribute.options.map((option) => ({ id: option, label: option })),
  };
}

/*
 * offerAIFields puts the attributes among the fields a view offers, in
 * place of the ones offered before. Archived ones stay out.
 */
export function offerAIFields(attributes: ConversationAttribute[]) {
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
  const attributes = useQuery({ ...conversationAttributesQuery(workspace), retry: false }).data;
  if (attributes) {
    offerAIFields(attributes);
  }
}
