/*
 * How the agent answers and what it is allowed to see. Answers: how many
 * replies it gives before a teammate takes over, what it does when it
 * does not know, whether every answer cites its sources, and the sites
 * it may link to. Privacy: whether personal data is hidden from the
 * model and how much of the customer's record it reads.
 */
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Row, Section } from "@/features/settings/layout";
import type { AiAgent } from "@/lib/api/ai";

import type { AgentChange } from "../format";
import { SavedTextarea, StackedRow } from "./fields";
import { contactContexts, domains, unknownBehaviors } from "./options";

const replyLimits = [3, 5, 10, 20];

export function AnswersSection({
  agent,
  canWrite,
  save,
}: {
  agent: AiAgent;
  canWrite: boolean;
  save: (change: AgentChange) => void;
}) {
  const limits = replyLimits.includes(agent.max_ai_replies)
    ? replyLimits
    : [...replyLimits, agent.max_ai_replies].sort((a, b) => a - b);
  const allowlist = agent.link_allowlist.join("\n");

  return (
    <Section title="Answers" description="How far it goes on its own.">
      <Row label="Replies per conversation" description="After this many answers it hands the conversation over.">
        <Select
          aria-label="Replies per conversation"
          size="sm"
          isDisabled={!canWrite}
          selectedKey={String(agent.max_ai_replies)}
          onSelectionChange={(key) => (key ? save({ max_ai_replies: Number(key) }) : undefined)}
          popoverClassName="w-40"
        >
          {limits.map((limit) => (
            <SelectItem key={limit} id={String(limit)} textValue={`${limit} replies`}>
              {limit} replies
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="When it doesn't know" description="It never guesses; it says so and does this.">
        <Select
          aria-label="When it doesn't know"
          size="sm"
          isDisabled={!canWrite}
          selectedKey={agent.unknown_behavior}
          onSelectionChange={(key) => (key ? save({ unknown_behavior: String(key) }) : undefined)}
          popoverClassName="w-60"
        >
          {unknownBehaviors.map((option) => (
            <SelectItem key={option.id} id={option.id} textValue={option.label}>
              {option.label}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="Cite sources" description="Each answer links the articles it came from.">
        <Switch
          aria-label="Cite sources"
          size="sm"
          isDisabled={!canWrite}
          isSelected={agent.require_citations}
          onChange={(require_citations) => save({ require_citations })}
        />
      </Row>
      <StackedRow
        label="Links it may share"
        description="Sites beyond your help center it may link to, one per line. Leave empty for your help center only."
      >
        <SavedTextarea
          label="Links it may share"
          value={allowlist}
          placeholder={"example.com\nstatus.example.com"}
          maxLength={2000}
          disabled={!canWrite}
          onSave={(text) => save({ link_allowlist: domains(text) })}
        />
      </StackedRow>
    </Section>
  );
}

export function PrivacySection({
  agent,
  canWrite,
  save,
}: {
  agent: AiAgent;
  canWrite: boolean;
  save: (change: AgentChange) => void;
}) {
  const context = contactContexts.find((c) => c.id === agent.include_contact_context) ?? contactContexts[0];

  return (
    <Section title="Privacy" description="What reaches the model provider.">
      <Row label="Hide personal data" description="Emails, phone numbers and card numbers are masked before sending.">
        <Switch
          aria-label="Hide personal data"
          size="sm"
          isDisabled={!canWrite}
          isSelected={agent.pii_redaction}
          onChange={(pii_redaction) => save({ pii_redaction })}
        />
      </Row>
      <Row label="Customer details" description={context.line}>
        <SegmentedControl
          aria-label="Customer details"
          selectedKeys={[context.id]}
          isDisabled={!canWrite}
          onSelectionChange={(keys) => {
            const next = contactContexts.find((c) => keys.has(c.id));
            if (next && next.id !== agent.include_contact_context) {
              save({ include_contact_context: next.id });
            }
          }}
        >
          {contactContexts.map((c) => (
            <SegmentedControlItem key={c.id} id={c.id}>
              {c.label}
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
      </Row>
    </Section>
  );
}
