/*
 * The workspace's conversation attributes as condition fields: each one
 * reads as conversation.ai.<key>, a list offering its values, yes or no
 * for a boolean, numbers for numbers, and typed words for the rest.
 * Under the triggers for an attribute changing, conditions can also read
 * which one changed, its new value and, for any change, who set it.
 * They join the fields the builder offers once the workspace's
 * attributes are read.
 */
import { useQuery } from "@tanstack/react-query";

import { useAccount } from "@/features/settings/use-account";
import { type ConversationAttribute, conversationAttributesQuery } from "@/lib/api/conversation-attributes";

import { type ConditionField, conditionFields } from "./conditions";

/*
 * aiField is one attribute as a condition field.
 */
export function aiField(attribute: ConversationAttribute): ConditionField {
  const name = attribute.label;
  if (attribute.data_type === "choice") {
    return { name, type: "choice", options: Object.fromEntries(attribute.options.map((o) => [o, o])) };
  }
  if (attribute.data_type === "number" || attribute.data_type === "decimal") {
    return { name, type: "number" };
  }

  return { name, type: attribute.data_type === "boolean" ? "yes_no" : "text" };
}

/*
 * addAIFields offers the attributes as condition fields, and replaces
 * the ones offered before.
 */
export function addAIFields(attributes: ConversationAttribute[]) {
  for (const key of Object.keys(conditionFields)) {
    if (key.startsWith("conversation.ai.")) {
      delete conditionFields[key];
    }
  }
  const active = attributes.filter((attribute) => attribute.status === "active");
  for (const attribute of active) {
    conditionFields[`conversation.ai.${attribute.key}`] = aiField(attribute);
  }
  const keys = Object.fromEntries(active.map((attribute) => [attribute.key, attribute.label]));
  conditionFields["attribute.key"] = {
    name: "Changed attribute",
    type: "choice",
    options: keys,
    triggers: ["conversation.attribute_changed"],
  };
  conditionFields["attribute.value"] = {
    name: "New attribute value",
    type: "text",
    triggers: ["conversation.attribute_changed"],
  };
  conditionFields["attribute.by"] = {
    name: "Changed by",
    type: "choice",
    options: { member: "A teammate", ai: "The AI", workflow: "A workflow", import: "An import" },
    triggers: ["conversation.attribute_changed"],
  };
  conditionFields["ai_attribute.key"] = {
    name: "Attribute the AI set",
    type: "choice",
    options: Object.fromEntries(active.map((attribute) => [attribute.key, attribute.label])),
    triggers: ["conversation.ai_attribute_changed"],
  };
  conditionFields["ai_attribute.value"] = {
    name: "Value the AI set",
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
  const attributes = useQuery({ ...conversationAttributesQuery(workspace), retry: false }).data;
  if (attributes) {
    addAIFields(attributes);
  }
}
