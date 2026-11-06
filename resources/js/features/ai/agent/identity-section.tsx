/*
 * Who the agent is to customers: the name it signs with, what it calls
 * your product, how it sounds, and the standing guidance it follows in
 * every answer.
 */
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Row, Section } from "@/features/settings/layout";
import type { AiAgent } from "@/lib/api/ai";

import type { AgentChange } from "../format";
import { SavedText, SavedTextarea, StackedRow } from "./fields";
import { tones } from "./options";

/*
 * guidanceLimit is the longest guidance the server keeps.
 */
const guidanceLimit = 2000;

export function IdentitySection({
  agent,
  canWrite,
  save,
}: {
  agent: AiAgent;
  canWrite: boolean;
  save: (change: AgentChange) => void;
}) {
  const tone = tones.find((t) => t.id === agent.tone) ?? tones[0];

  return (
    <Section title="Identity" description="How the agent introduces itself and sounds.">
      <Row label="Name" description="Shown on its replies in the messenger.">
        <SavedText
          label="Agent name"
          value={agent.name}
          maxLength={60}
          required
          disabled={!canWrite}
          onSave={(name) => save({ name })}
        />
      </Row>
      <Row label="Product name" description="What it calls your product in answers.">
        <SavedText
          label="Product name"
          value={agent.product_name ?? ""}
          placeholder="Your product"
          maxLength={80}
          disabled={!canWrite}
          onSave={(product_name) => save({ product_name })}
        />
      </Row>
      <StackedRow label="Tone" description={tone.line}>
        <SegmentedControl
          aria-label="Tone"
          selectedKeys={[tone.id]}
          isDisabled={!canWrite}
          onSelectionChange={(keys) => {
            const next = tones.find((t) => keys.has(t.id));
            if (next && next.id !== agent.tone) {
              save({ tone: next.id });
            }
          }}
          className="self-start"
        >
          {tones.map((t) => (
            <SegmentedControlItem key={t.id} id={t.id}>
              {t.label}
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
      </StackedRow>
      <StackedRow
        label="Guidance"
        description="Anything it should always know or do: policies, words to avoid, when to point to a page."
      >
        <SavedTextarea
          label="Guidance"
          value={agent.guidance}
          placeholder="For example: We ship worldwide except Antarctica. Never promise refunds; offer a teammate instead."
          maxLength={guidanceLimit}
          disabled={!canWrite}
          onSave={(guidance) => save({ guidance })}
        />
      </StackedRow>
    </Section>
  );
}
