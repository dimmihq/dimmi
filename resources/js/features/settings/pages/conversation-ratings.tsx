/*
 * Whether and how the workspace asks customers to rate a conversation
 * once it closes. The messenger shows five faces in the thread; an
 * email conversation's customer gets a short email whose faces open a
 * page that keeps the score. The ask goes out straight away or after a
 * delay, so a customer who writes again in the meantime is not asked to
 * rate a conversation that opened again. Only people the server lets
 * change settings see working controls.
 */
import { RiTimeLine, RiTimerFlashLine, RiTimerLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Switch } from "@/components/base/switch/switch";
import { type RatingSettings, ratingSettingsQuery, updateRatingSettings } from "@/lib/api/ratings";
import { callerQuery } from "@/lib/api/workspace";

import { Row, Section, SettingsPage } from "../layout";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

function useSave(workspace: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (change: Partial<RatingSettings>) => updateRatingSettings(workspace, change),
    onSuccess: (saved) => client.setQueryData(ratingSettingsQuery(workspace).queryKey, saved),
  });
}

const delays = [
  { value: 0, title: "Straight away", description: "As soon as the conversation closes.", icon: RiTimerFlashLine },
  { value: 10, title: "After 10 minutes", description: "Gives the customer a moment to reply.", icon: RiTimerLine },
  { value: 60, title: "After an hour", description: "Asks once things have settled.", icon: RiTimeLine },
];

function Asking({ settings, canManage }: { settings: RatingSettings; canManage: boolean }) {
  const workspace = useAccount().workspace.id;
  const save = useSave(workspace);

  return (
    <>
      <Section title="Asking" description="Customers pick one of five faces and can add a comment.">
        <Row
          label="Ask for a rating when a conversation closes"
          description="Customers can change their rating for a week."
        >
          <Switch
            aria-label="Ask for a rating when a conversation closes"
            size="sm"
            isSelected={settings.enabled}
            isDisabled={!canManage}
            onChange={(on) => save.mutate({ enabled: on })}
          />
        </Row>
        <Row
          label="Include conversations closed by the agent"
          description="Also ask when dimmi resolved and closed the conversation."
        >
          <Switch
            aria-label="Include conversations closed by the agent"
            size="sm"
            isSelected={settings.include_agent_closed}
            isDisabled={!canManage || !settings.enabled}
            onChange={(on) => save.mutate({ include_agent_closed: on })}
          />
        </Row>
      </Section>
      <Section title="When to ask" description="Nothing is sent if the conversation opens again before then.">
        <div className="py-3 pe-3">
          <RadioGroup
            aria-label="When to ask"
            value={String(settings.delay_minutes)}
            isDisabled={!canManage || !settings.enabled}
            onChange={(value) => save.mutate({ delay_minutes: Number(value) })}
          >
            {delays.map((delay) => (
              <RadioCard
                key={delay.value}
                value={String(delay.value)}
                icon={delay.icon}
                title={delay.title}
                description={delay.description}
              />
            ))}
          </RadioGroup>
        </div>
      </Section>
      <Section title="Channels" description="Where customers are asked.">
        <Row label="Messenger" description="A card with five faces at the end of the thread.">
          <Switch
            aria-label="Messenger"
            size="sm"
            isSelected={settings.messenger}
            isDisabled={!canManage || !settings.enabled}
            onChange={(on) => save.mutate({ messenger: on })}
          />
        </Row>
        <Row label="Email" description="A short email whose faces open a rating page.">
          <Switch
            aria-label="Email"
            size="sm"
            isSelected={settings.email}
            isDisabled={!canManage || !settings.enabled}
            onChange={(on) => save.mutate({ email: on })}
          />
        </Row>
      </Section>
      {save.error ? <p className="text-body-2-regular text-text-error-primary">{failureText(save.error)}</p> : null}
    </>
  );
}

export function ConversationRatingsSettings() {
  const workspace = useAccount().workspace.id;
  const { data } = useQuery(ratingSettingsQuery(workspace));
  const { data: caller } = useQuery(callerQuery(workspace));
  const canManage = (caller?.member?.permissions ?? []).includes("settings:write");

  return (
    <SettingsPage title="Conversation ratings" description="Ask customers how it went when a conversation closes.">
      {data ? (
        <Asking settings={data} canManage={canManage} />
      ) : (
        <>
          <Section title="Asking" loading rows={2} />
          <Section title="Channels" loading rows={2} />
        </>
      )}
    </SettingsPage>
  );
}
