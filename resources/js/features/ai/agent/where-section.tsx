/*
 * Where and when the agent answers: the channels it takes, the hours it
 * works, read from each inbox's office hours, and what it does once a
 * teammate holds the conversation. At
 * least one channel stays chosen, since an agent on no channel never
 * answers anyone.
 */
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Radio, RadioGroup } from "@/components/base/radio/radio";
import { Select, SelectItem } from "@/components/base/select/select";
import { Row, Section } from "@/features/settings/layout";
import type { AiAgent } from "@/lib/api/ai";

import type { AgentChange } from "../format";
import { StackedRow } from "./fields";
import { assignedBehaviors, channels, schedules, toggled } from "./options";

export function WhereSection({
  agent,
  canWrite,
  save,
}: {
  agent: AiAgent;
  canWrite: boolean;
  save: (change: AgentChange) => void;
}) {
  return (
    <Section title="Where & when" description="The conversations it takes and the hours it works.">
      <StackedRow label="Channels" description="It answers new conversations on the channels you tick.">
        <fieldset aria-label="Channels" className="m-0 flex min-w-0 flex-col gap-3 border-0 p-0">
          {channels.map((channel) => {
            const on = agent.enabled_channels.includes(channel.id);
            const last = on && agent.enabled_channels.length === 1;

            return (
              <Checkbox
                key={channel.id}
                isSelected={on}
                isDisabled={!canWrite || last}
                onChange={(next) => save({ enabled_channels: toggled(agent.enabled_channels, channel.id, next) })}
              >
                <span className="flex flex-col">
                  <span className="text-body-regular text-text-primary">{channel.label}</span>
                  <span className="text-body-2-regular text-text-secondary">{channel.line}</span>
                </span>
              </Checkbox>
            );
          })}
        </fieldset>
      </StackedRow>
      <StackedRow
        label="Hours"
        description="It follows the office hours of each conversation's inbox, set on the inbox's page."
      >
        <RadioGroup
          aria-label="Hours"
          value={agent.schedule}
          isDisabled={!canWrite}
          onChange={(schedule) => save({ schedule })}
          className="gap-3"
        >
          {schedules.map((schedule) => (
            <Radio key={schedule.id} value={schedule.id}>
              <span className="flex flex-col">
                <span className="text-body-regular text-text-primary">{schedule.label}</span>
                <span className="text-body-2-regular text-text-secondary">{schedule.line}</span>
              </span>
            </Radio>
          ))}
        </RadioGroup>
      </StackedRow>
      <Row label="Once a teammate is assigned" description="Whether it keeps replying when a person takes over.">
        <Select
          aria-label="Once a teammate is assigned"
          size="sm"
          isDisabled={!canWrite}
          selectedKey={agent.assigned_behavior}
          onSelectionChange={(key) => (key ? save({ assigned_behavior: String(key) }) : undefined)}
          popoverClassName="w-72"
        >
          {assignedBehaviors.map((option) => (
            <SelectItem key={option.id} id={option.id} textValue={option.label}>
              {option.label}
            </SelectItem>
          ))}
        </Select>
      </Row>
    </Section>
  );
}
