/*
 * What the AI read off the conversation: each active AI attribute
 * beside the value it found, "Not found yet" where it found none, and
 * when it last read the conversation. A value an archived attribute
 * still holds stays listed. The section is left out when the workspace
 * has no active attribute, and refetches when the AI changes a value.
 */
import { useQuery } from "@tanstack/react-query";

import { ago } from "@/features/inbox/conversations";
import { aiValueLabel } from "@/lib/ai-values";
import { conversationAIAttributesQuery } from "@/lib/api/ai-attributes";

import { Row, Rows, Section } from "./rows";

export function AIAttributesSection({ workspace, conversation }: { workspace: string; conversation: string }) {
  const { data } = useQuery(conversationAIAttributesQuery(workspace, conversation));
  if (!data?.values.some((v) => v.attribute.status === "active")) {
    return null;
  }

  return (
    <Section title="AI attributes" aside={data.checked_at ? `Read ${ago(data.checked_at).toLowerCase()}` : undefined}>
      <Rows
        rows={data.values.map(({ attribute, value }) => {
          const shown = aiValueLabel(attribute, value);

          return (
            <Row key={attribute.id} label={<span className="truncate">{attribute.label}</span>}>
              {shown === null ? (
                <span className="text-text-tertiary">Not found yet</span>
              ) : (
                <span dir="auto" className="min-w-0 truncate" title={shown}>
                  {shown}
                </span>
              )}
            </Row>
          );
        })}
      />
    </Section>
  );
}
