/*
 * Who sees a messenger: the launcher, starting a conversation, and the
 * help and what's new tabs, each open to everyone or to one saved
 * audience, made under Customers, Audiences. A choice waits in the
 * page's draft and goes with the page's Save. The
 * server decides every rule; a browser learns only yes, no, or the page
 * conditions to check, so rules on customer data stay private. Starting a
 * conversation can only use audiences of what the team's own systems
 * said, never what a browser could claim; the server says so when one
 * does not fit. A rule written through the API shows as custom and stays
 * until someone picks again.
 */
import { useQuery } from "@tanstack/react-query";

import { Select, SelectItem } from "@/components/base/select/select";

import { custom, everyone } from "@/features/automation/audience-choice";
import { audiencesQuery } from "@/lib/api/audiences";

import { Row, Section } from "../layout";
import { type AudienceChoices, type Slot, savedChoice } from "./audience-choices";
import type { TabProps } from "./parts";

const slots: { slot: Slot; label: string; description: string }[] = [
  {
    slot: "launcher",
    label: "Launcher",
    description: "Hidden from everyone else. Replies and announcements still reach them.",
  },
  {
    slot: "start",
    label: "Starting a conversation",
    description: "Others can still read and answer their conversations.",
  },
  { slot: "help", label: "Help tab", description: "Shown only to these people." },
  { slot: "changelog", label: "What's new tab", description: "Shown only to these people." },
];

export function AudiencesSection({
  workspace,
  value,
  locked,
  choices: picked,
  onPick,
}: { workspace: string; choices: AudienceChoices; onPick: (slot: Slot, choice: string) => void } & Pick<
  TabProps,
  "value" | "locked"
>) {
  const saved = useQuery(audiencesQuery(workspace)).data ?? [];

  return (
    <Section title="Who sees it" description="Limit parts of the messenger to a saved audience.">
      {slots.map(({ slot, label, description }) => {
        const current = picked[slot] ?? savedChoice(value.audiences, slot);
        const choices: [string, string][] = [
          [everyone, "Everyone"],
          ...saved.map((a): [string, string] => [a.id, a.name]),
          ...(current === custom ? [[custom, "A custom rule"] as [string, string]] : []),
        ];

        return (
          <Row key={slot} label={label} description={description}>
            {locked ? null : (
              <Select
                aria-label={label}
                size="sm"
                popoverClassName="w-56"
                selectedKey={current}
                onSelectionChange={(choice) => {
                  if (choice !== null && choice !== custom) {
                    onPick(slot, String(choice));
                  }
                }}
              >
                {choices.map(([id, name]) => (
                  <SelectItem key={id} id={id} textValue={name}>
                    {name}
                  </SelectItem>
                ))}
              </Select>
            )}
          </Row>
        );
      })}
    </Section>
  );
}
