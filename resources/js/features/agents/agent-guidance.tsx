/*
 * An agent's guidance: one document of how it talks and what it does,
 * everywhere it answers. An agent that should behave differently for
 * some customers is a second agent, not a rule here.
 */
import { Section } from "@/features/settings/layout";

import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";
import { GuidanceWriter } from "./guidance-writer";

export function AgentGuidance({
  markdown,
  sources,
  onChange,
}: {
  markdown: string;
  sources: ChipSources;
  onChange: (markdown: string) => void;
}) {
  return (
    <Section
      title="Guidance"
      description="How it talks and what it does, in your words. Type @ to name a procedure, article, action, detail or team, and / for headings and lists."
    >
      <div className="py-3 pe-3">
        <GuidanceWriter markdown={markdown} sources={sources} onChange={onChange} />
      </div>
    </Section>
  );
}
