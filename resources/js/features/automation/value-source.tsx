/*
 * Where an action's input comes from, picked rather than typed: a detail
 * of the contact or the conversation, one of the workspace's AI
 * attributes, a variable an earlier step saved, a contact attribute, or
 * a fixed value. The setting stores what the server fills, such as
 * {{contact.email}}, or the fixed text as it is.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Field } from "@/features/settings/controls";
import { useAccount } from "@/features/settings/use-account";
import { conversationAttributesQuery } from "@/lib/api/conversation-attributes";

import type { Choices } from "./choices";
import { Pick } from "./pick";
import { aiAttr, contactAttr, details, fixed, runVar, sourceOf, storedValue } from "./step-values";

export function ValueSource({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const workspace = useAccount().workspace.id;
  const attributes = useQuery({ ...conversationAttributesQuery(workspace), retry: false }).data ?? [];
  const [saved, savedName] = sourceOf(value);
  const [source, setSource] = useState(saved);
  const [name, setName] = useState(savedName);
  const choices: Choices = [
    ...Object.entries(details),
    ...attributes
      .filter((attribute) => attribute.status === "active")
      .map((attribute) => [aiAttr + attribute.key, `AI: ${attribute.label}`] as const),
    [runVar, "Run variable"],
    [contactAttr, "Contact attribute"],
    [fixed, "Fixed value"],
  ];
  const named = source === fixed || source === runVar || source === contactAttr;

  return (
    <span className="flex flex-wrap items-center gap-2">
      <Pick
        label={`${label} from`}
        choices={choices}
        value={source}
        onChange={(next) => {
          setSource(next);
          onChange(storedValue(next, name));
        }}
      />
      {named ? (
        <Field
          label={`${label} ${source === fixed ? "value" : "name"}`}
          placeholder={source === fixed ? "value" : "name"}
          value={name}
          width="w-32"
          onChange={(next) => {
            setName(next);
            onChange(storedValue(source, next));
          }}
        />
      ) : null}
    </span>
  );
}
