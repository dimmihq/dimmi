/*
 * An agent's guidance: one document of how it talks and what it does,
 * everywhere it answers. An agent that should behave differently for
 * some customers is a second agent, not a rule here. Mockup: sample
 * guidance, nothing saved.
 */
import { Section } from "@/features/settings/layout";

import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";
import { GuidanceWriter } from "./guidance-writer";
import { guidanceHtml } from "./mock";

export function AgentGuidance({ sources }: { sources: ChipSources }) {
  return (
    <Section
      title="Guidance"
      description="How it talks and what it does, in your words. Type @ to name a procedure, article, action, detail or team, and / for headings and lists."
    >
      <div className="py-3 pe-3">
        <GuidanceWriter html={guidanceHtml} sources={sources} />
      </div>
    </Section>
  );
}
