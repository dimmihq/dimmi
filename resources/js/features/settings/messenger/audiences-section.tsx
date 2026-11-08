/*
 * Who sees a messenger: the launcher, starting a conversation, and the
 * help and what's new tabs, each open to everyone or to one saved
 * audience, made under Customers, Audiences. A choice saves at once. The
 * server decides every rule; a browser learns only yes, no, or the page
 * conditions to check, so rules on customer data stay private. Starting a
 * conversation can only use audiences of what the team's own systems
 * said, never what a browser could claim; the server says so when one
 * does not fit. A rule written through the API shows as custom and stays
 * until someone picks again.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Select, SelectItem } from "@/components/base/select/select";

import { audienceChoice, audienceFor, custom, everyone } from "@/features/automation/audience-choice";
import { audiencesQuery } from "@/lib/api/audiences";
import { ApiError } from "@/lib/api/client";
import { type MessengerAudiences, saveMessengerAudiences } from "@/lib/api/widget-settings";

import { Row, Section } from "../layout";
import type { TabProps } from "./parts";

type Slot = "launcher" | "start" | "help" | "changelog";

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

function ruleOf(rules: MessengerAudiences, slot: Slot) {
  return (slot === "launcher" || slot === "start" ? rules[slot] : rules.tabs?.[slot]) ?? undefined;
}

/*
 * change is what to send for one choice: the slot's rule, null for
 * everyone.
 */
function change(slot: Slot, choice: string): MessengerAudiences {
  const rule = audienceFor(choice) ?? null;

  return slot === "launcher" || slot === "start" ? { [slot]: rule } : { tabs: { [slot]: rule } };
}

function problem(error: unknown): string | null {
  if (!error) {
    return null;
  }
  const list = error instanceof ApiError ? (error.details.errors as { message: string }[] | undefined) : undefined;

  return list?.[0]?.message ?? (error instanceof Error ? error.message : "Not saved.");
}

export function AudiencesSection({
  workspace,
  value,
  locked,
}: { workspace: string } & Pick<TabProps, "value" | "locked">) {
  const queryClient = useQueryClient();
  const [rules, setRules] = useState<MessengerAudiences>(value.audiences ?? {});
  const saved = useQuery(audiencesQuery(workspace)).data ?? [];
  const save = useMutation({
    mutationFn: (next: MessengerAudiences) => saveMessengerAudiences(workspace, value.id, next),
    onSuccess: (answer) => {
      setRules(answer.rules);
      void queryClient.invalidateQueries({
        queryKey: ["workspace", workspace, "widget-settings"],
        refetchType: "none",
      });
    },
  });
  const failed = problem(save.error);

  return (
    <Section title="Who sees it" description="Limit parts of the messenger to a saved audience. Changes apply at once.">
      {slots.map(({ slot, label, description }) => {
        const current = audienceChoice(ruleOf(rules, slot));
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
                    save.mutate(change(slot, String(choice)));
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
      {failed ? <Row label={<span className="text-text-error-primary">{failed}</span>} /> : null}
    </Section>
  );
}
