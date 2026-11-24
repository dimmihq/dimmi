/*
 * The workspace's AI attributes as condition fields: each one reads as
 * conversation.ai.<key>, a choice offering its values, yes or no for a
 * boolean, and typed words for the rest. Under the trigger for an AI
 * attribute changing, conditions can also read which one changed and
 * its new value. They join the fields the builder offers once the
 * workspace's attributes are read.
 */
import { useQuery } from "@tanstack/react-query";

import { useAccount } from "@/features/settings/use-account";
import { type AIAttribute, aiAttributesQuery } from "@/lib/api/ai-attributes";

import { type ConditionField, conditionFields } from "./conditions";

/*
 * aiField is one attribute as a condition field.
 */
export function aiField(attribute: AIAttribute): ConditionField {
  const name = `AI: ${attribute.label}`;
  if (attribute.data_type === "choice") {
    return { name, type: "choice", options: Object.fromEntries(attribute.options.map((o) => [o, o])) };
  }

  return { name, type: attribute.data_type === "boolean" ? "yes_no" : "text" };
}

/*
 * addAIFields offers the attributes as condition fields, and replaces
 * the ones offered before.
 */
export function addAIFields(attributes: AIAttribute[]) {
  for (const key of Object.keys(conditionFields)) {
    if (key.startsWith("conversation.ai.")) {
      delete conditionFields[key];
    }
  }
  const active = attributes.filter((attribute) => attribute.status === "active");
  for (const attribute of active) {
    conditionFields[`conversation.ai.${attribute.key}`] = aiField(attribute);
  }
  conditionFields["ai_attribute.key"] = {
    name: "Changed AI attribute",
    type: "choice",
    options: Object.fromEntries(active.map((attribute) => [attribute.key, attribute.label])),
    triggers: ["conversation.ai_attribute_changed"],
  };
  conditionFields["ai_attribute.value"] = {
    name: "New AI value",
    type: "text",
    triggers: ["conversation.ai_attribute_changed"],
  };
}

/*
 * useAIConditionFields reads the workspace's AI attributes and offers
 * them as condition fields.
 */
export function useAIConditionFields() {
  const workspace = useAccount().workspace.id;
  const attributes = useQuery({ ...aiAttributesQuery(workspace), retry: false }).data;
  if (attributes) {
    addAIFields(attributes);
  }
}
