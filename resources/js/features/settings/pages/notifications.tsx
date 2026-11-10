/*
 * The member's notification settings: which notifications reach them in
 * the dashboard, by email, by push and in Slack, this browser's push and the other
 * devices it is on, quiet hours on their own clock, a pause, and whether
 * previews and conversations the agent is answering come through. Every
 * change saves at once. A cell that cannot be turned off says so.
 */
import { RiComputerLine, RiDeleteBinLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Notification } from "@/components/base/notification/notification";
import { Switch } from "@/components/base/switch/switch";
import { toast } from "@/components/base/toast/toast";
import {
  type NotificationChannel,
  type NotificationPreferences,
  type NotificationSettings,
  notificationPreferencesQuery,
  pushConfigQuery,
  pushDevicesQuery,
  removePushDevice,
  resetNotificationPreferences,
  sendTestNotification,
  updateNotificationPreferences,
  updateNotificationSettings,
} from "@/lib/api/notifications";
import { formatDateTime, formatRelative } from "@/lib/i18n/format";

import { enablePush, pushSupport } from "../../notifications/push";
import { Action, Choice } from "../controls";
import { Empty, Glyph, Item, Row, RowsSkeleton, Section, SettingsPage } from "../layout";
import { useAccount } from "../use-account";
import { SlackDirectSection } from "./slack-direct";

const channelLabels: Record<NotificationChannel, string> = {
  in_app: "In app",
  email: "Email",
  push: "Push",
  slack: "Slack",
};

const days = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

const halfHours = Array.from({ length: 48 }, (_, i) => {
  const h = String(Math.floor(i / 2)).padStart(2, "0");
  return `${h}:${i % 2 === 0 ? "00" : "30"}`;
});

const pauses: { label: string; hours: number }[] = [
  { label: "1 hour", hours: 1 },
  { label: "4 hours", hours: 4 },
  { label: "Until tomorrow", hours: 24 },
  { label: "1 week", hours: 24 * 7 },
];

export function NotificationsSettings() {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const query = notificationPreferencesQuery(workspace);
  const { data, isPending } = useQuery({ ...query, enabled: workspace !== "" });
  const save = (next: NotificationPreferences) => queryClient.setQueryData(query.queryKey, next);
  const cells = useMutation({
    mutationFn: (change: { trigger: string; channel: NotificationChannel; enabled: boolean }) =>
      updateNotificationPreferences(workspace, [change]),
    onSuccess: save,
  });
  const reset = useMutation({ mutationFn: () => resetNotificationPreferences(workspace), onSuccess: save });
  const settings = useMutation({
    mutationFn: (change: Partial<NotificationSettings> & { clear_pause?: boolean }) =>
      updateNotificationSettings(workspace, change),
    onSuccess: (next) => data && save({ ...data, settings: next }),
  });
  const failure = cells.error ?? settings.error ?? reset.error;

  return (
    <SettingsPage title="Notifications" description="What reaches you, where, and when to stay quiet.">
      {failure ? (
        <Notification status="error" title={failure.message} dismissible={false} role="alert" className="w-full" />
      ) : null}
      <Section
        title="What reaches you"
        description="The bell in the dashboard, email, push on the browsers you enable, and your Slack DMs."
        action={<Action onPress={() => reset.mutate()}>Reset to defaults</Action>}
      >
        {isPending || !data ? <RowsSkeleton rows={4} /> : <Matrix data={data} onChange={(c) => cells.mutate(c)} />}
      </Section>
      <PushSection workspace={workspace} />
      <SlackDirectSection workspace={workspace} />
      {data ? <QuietSection settings={data.settings} onChange={(c) => settings.mutate(c)} /> : null}
    </SettingsPage>
  );
}

function Matrix({
  data,
  onChange,
}: {
  data: NotificationPreferences;
  onChange: (change: { trigger: string; channel: NotificationChannel; enabled: boolean }) => void;
}) {
  return (
    <div className="overflow-x-auto py-2.5 pe-2.5">
      <table className="w-full min-w-[480px] text-start">
        <thead>
          <tr>
            <th className="pb-2 text-start text-caption-1-medium text-text-tertiary">Notification</th>
            {data.channels.map((channel) => (
              <th key={channel} className="w-20 pb-2 text-center text-caption-1-medium text-text-tertiary">
                {channelLabels[channel]}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.triggers.map((trigger) => (
            <tr key={trigger.key} className="border-t border-separator-border">
              <td className="py-2.5 pe-4">
                <span className="block text-body-regular text-text-primary">{trigger.label}</span>
                <span className="block text-body-2-regular text-text-tertiary">{trigger.description}</span>
              </td>
              {data.channels.map((channel) => {
                const cell = data.cells.find((c) => c.trigger === trigger.key && c.channel === channel);
                return (
                  <td key={channel} className="py-2.5 text-center">
                    <span className="inline-flex justify-center">
                      <Switch
                        size="sm"
                        aria-label={`${trigger.label} by ${channelLabels[channel]}`}
                        isSelected={cell?.enabled ?? false}
                        isDisabled={cell?.locked ?? false}
                        onChange={(enabled) => onChange({ trigger: trigger.key, channel, enabled })}
                      />
                    </span>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function PushSection({ workspace }: { workspace: string }) {
  const queryClient = useQueryClient();
  const { data: config } = useQuery(pushConfigQuery());
  const { data: devices = [], isPending } = useQuery(pushDevicesQuery());
  const [enabling, setEnabling] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const refresh = () => queryClient.invalidateQueries({ queryKey: pushDevicesQuery().queryKey });
  const remove = useMutation({ mutationFn: removePushDevice, onSettled: refresh });
  const test = useMutation({
    mutationFn: (channel: NotificationChannel) => sendTestNotification(workspace, channel),
    onSuccess: () => toast.success("Test sent"),
    onError: (error) => toast.error(error.message),
  });
  const support = pushSupport();

  const enable = async () => {
    if (!config) {
      return;
    }
    setEnabling(true);
    setProblem(null);
    try {
      await enablePush(config.vapid_public_key);
      await refresh();
      toast.success("Push is on for this browser");
    } catch (error) {
      setProblem(error instanceof Error ? error.message : "Push could not be turned on.");
    } finally {
      setEnabling(false);
    }
  };

  return (
    <Section
      title="Push"
      description="System notifications on this computer or phone, even when the dashboard is closed."
      action={
        <Action onPress={() => void enable()} loading={enabling}>
          Enable on this browser
        </Action>
      }
    >
      {support === "unsupported" ? <Empty>This browser cannot receive push notifications.</Empty> : null}
      {support === "denied" ? <Empty>Notifications are blocked for this site in your browser's settings.</Empty> : null}
      {problem ? (
        <div className="py-2.5 pe-2.5">
          <Notification status="error" title={problem} dismissible={false} role="alert" className="w-full" />
        </div>
      ) : null}
      {isPending ? <RowsSkeleton rows={1} /> : null}
      {!isPending && devices.length === 0 ? <Empty>No devices yet.</Empty> : null}
      {devices.map((device) => (
        <Item
          key={device.id}
          mark={<Glyph icon={RiComputerLine} />}
          title={device.device_label ?? "Browser"}
          detail={
            device.last_success_at
              ? `Last push ${formatRelative(device.last_success_at)}`
              : `Added ${formatDateTime(device.created_at)}`
          }
        >
          <Button
            variant="ghost"
            size="small"
            iconOnly
            leadingIcon={RiDeleteBinLine}
            aria-label={`Remove ${device.device_label ?? "this device"}`}
            onClick={() => remove.mutate(device.id)}
          />
        </Item>
      ))}
      <Row label="Send a test" description="Check each channel reaches you.">
        <div className="flex flex-wrap gap-2">
          {(["in_app", "email", "push", "slack"] as const).map((channel) => (
            <Action key={channel} size="xs" onPress={() => test.mutate(channel)}>
              {channelLabels[channel]}
            </Action>
          ))}
        </div>
      </Row>
    </Section>
  );
}

function QuietSection({
  settings,
  onChange,
}: {
  settings: NotificationSettings;
  onChange: (change: Partial<NotificationSettings> & { clear_pause?: boolean }) => void;
}) {
  const zone = settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone;
  const paused = settings.paused_until && new Date(settings.paused_until) > new Date();
  const toggleDay = (index: number, on: boolean) => {
    const bit = 1 << index;
    onChange({ quiet_hours_days: on ? settings.quiet_hours_days | bit : settings.quiet_hours_days & ~bit });
  };

  return (
    <Section title="When to stay quiet" description="Email and push hold off; the bell still keeps everything.">
      <Row label="Quiet hours" description={`Every night on your own clock (${zone}).`}>
        <Switch
          size="sm"
          aria-label="Quiet hours"
          isSelected={settings.quiet_hours_enabled}
          onChange={(on) => onChange({ quiet_hours_enabled: on, ...(settings.timezone ? {} : { timezone: zone }) })}
        />
      </Row>
      {settings.quiet_hours_enabled ? (
        <div className="flex flex-col gap-3 py-2.5 pe-2.5">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-body-regular text-text-secondary">From</span>
            <Choice
              label="Quiet from"
              options={halfHours}
              value={settings.quiet_hours_start}
              onChange={(v) => onChange({ quiet_hours_start: v })}
            />
            <span className="text-body-regular text-text-secondary">to</span>
            <Choice
              label="Quiet until"
              options={halfHours}
              value={settings.quiet_hours_end}
              onChange={(v) => onChange({ quiet_hours_end: v })}
            />
          </div>
          <div className="flex flex-wrap gap-3">
            {days.map((day, index) => (
              <Checkbox
                key={day}
                size="sm"
                isSelected={(settings.quiet_hours_days & (1 << index)) !== 0}
                onChange={(on) => toggleDay(index, on)}
              >
                {day}
              </Checkbox>
            ))}
          </div>
        </div>
      ) : null}
      <Row
        label="Pause notifications"
        description={
          paused && settings.paused_until ? `Paused until ${formatDateTime(settings.paused_until)}.` : "Take a break."
        }
      >
        {paused ? (
          <Action onPress={() => onChange({ clear_pause: true })}>Resume</Action>
        ) : (
          <Choice
            label="Pause for"
            options={["Not paused", ...pauses.map((p) => p.label)]}
            value="Not paused"
            onChange={(label) => {
              const pick = pauses.find((p) => p.label === label);
              if (pick) {
                onChange({ paused_until: new Date(Date.now() + pick.hours * 3_600_000).toISOString() });
              }
            }}
          />
        )}
      </Row>
      <Row label="Show message previews" description="The customer's words in email and push.">
        <Switch
          size="sm"
          aria-label="Show message previews"
          isSelected={settings.show_previews}
          onChange={(on) => onChange({ show_previews: on })}
        />
      </Row>
      <Row
        label="Conversations the agent is answering"
        description="Hear about new conversations and replies even while the dimmi agent handles them."
      >
        <Switch
          size="sm"
          aria-label="Conversations the agent is answering"
          isSelected={settings.include_ai_handled}
          onChange={(on) => onChange({ include_ai_handled: on })}
        />
      </Row>
    </Section>
  );
}
