/*
 * How the integrations pages talk about tools, their features and Slack
 * rooms, in plain words. Each provider has a look and a line on what it
 * does, so a provider the server starts offering, such as GitHub or
 * Linear, gets its card without a new page. A room's settings page
 * sends only what changed, and a setting put back to its default is
 * cleared rather than stored.
 */

import {
  type RemixiconComponentType,
  RiAlarmWarningLine,
  RiGithubFill,
  RiPlugLine,
  RiSlackFill,
} from "@remixicon/react";
import type { ChipColor } from "@/components/base/badges/chip";

import type { Integration, RoomChange, RoomMode, RoomSettings, RoomTrigger, SlackRoom } from "@/lib/api/integrations";

export interface ProviderLook {
  label: string;
  icon: RemixiconComponentType;
  brand: string;
  detail: string;
}

/*
 * providerLooks are the providers this page knows how to draw. Brand
 * colours are the providers' own, behind a white logo.
 */
const providerLooks: Record<string, ProviderLook> = {
  slack: {
    label: "Slack",
    icon: RiSlackFill,
    brand: "#4A154B",
    detail: "Answer customers in Slack channels, alert your team and ask the operator.",
  },
  github: {
    label: "GitHub",
    icon: RiGithubFill,
    brand: "#181717",
    detail: "Link conversations to issues and pull requests.",
  },
  linear: {
    label: "Linear",
    icon: RiPlugLine,
    brand: "#5E6AD2",
    detail: "Link tickets to Linear issues; their status moves the ticket.",
  },
  jira: {
    label: "Jira",
    icon: RiPlugLine,
    brand: "#0052CC",
    detail: "Link tickets to Jira issues; their status moves the ticket.",
  },
  incident_io: {
    label: "incident.io",
    icon: RiAlarmWarningLine,
    brand: "#F25533",
    detail: "Link incidents to tickets; resolving one closes the loop.",
  },
  rootly: {
    label: "Rootly",
    icon: RiAlarmWarningLine,
    brand: "#6E38F7",
    detail: "Link incidents to tickets; resolving one closes the loop.",
  },
  pagerduty: {
    label: "PagerDuty",
    icon: RiAlarmWarningLine,
    brand: "#06AC38",
    detail: "Page on-call from an incident; acknowledged and resolved come back.",
  },
};

export function providerLook(name: string): ProviderLook {
  return (
    providerLooks[name] ?? { label: name, icon: RiPlugLine, brand: "#525252", detail: "Connect it to your workspace." }
  );
}

/*
 * featureLabels names each feature an installation serves and says what
 * it does.
 */
const featureLabels: Record<string, { label: string; detail: string }> = {
  channels: { label: "Customer channels", detail: "Turn messages in Slack channels and DMs into conversations." },
  notifications: { label: "Notifications", detail: "Post alerts to channels and to teammates' DMs." },
  operator: { label: "Operator", detail: "Ask dimmi's operator questions from your team's Slack." },
  issues: { label: "Linked issues", detail: "Create and link issues from tickets; their status moves the ticket." },
  incidents: { label: "Linked incidents", detail: "Declare and link incidents from tickets and follow their status." },
};

export function featureLabel(name: string): { label: string; detail: string } {
  return featureLabels[name] ?? { label: name, detail: "" };
}

/*
 * IntegrationHealth is how an installation stands, in one word for its
 * card: working, missing permissions a feature needs, or broken.
 */
export type IntegrationHealth = "connected" | "needs_permissions" | "broken";

export function integrationHealth(integration: Integration): IntegrationHealth {
  if (integration.status !== "active") {
    return "broken";
  }

  if (integration.features.some((feature) => feature.missing_scopes.length > 0)) {
    return "needs_permissions";
  }

  return "connected";
}

export const healthColor: Record<IntegrationHealth, ChipColor> = {
  connected: "lime",
  needs_permissions: "yellow",
  broken: "rose",
};

export const healthLabels: Record<IntegrationHealth, string> = {
  connected: "Connected",
  needs_permissions: "Needs permissions",
  broken: "Not working",
};

/*
 * landingErrors explain each code a provider's return can carry.
 */
const landingErrors: Record<string, string> = {
  denied: "The connection was declined, so nothing changed.",
  enterprise_install_unsupported: "Install the app in each Slack workspace rather than across the whole organization.",
  integration_account_taken: "This account is already connected to another dimmi workspace.",
  integration_account_mismatch: "That account is not the one this workspace is connected to.",
  integration_provider_error: "The service did not finish connecting. Try again.",
  integration_not_active: "Reconnect the app first, then try again.",
};

/*
 * Landing is what a provider's return says: the installation it
 * connected, or what went wrong, or nothing for a page not reached
 * from a provider.
 */
export type Landing = { kind: "connected"; id: string } | { kind: "error"; message: string } | null;

export function landingOf(search: string): Landing {
  const params = new URLSearchParams(search);
  const connected = params.get("connected");
  const error = params.get("error");

  if (connected) {
    return { kind: "connected", id: connected };
  }

  if (error) {
    return { kind: "error", message: landingErrors[error] ?? "Something went wrong while connecting. Try again." };
  }

  return null;
}

interface Option<T extends string> {
  id: T;
  label: string;
  detail: string;
}

export const roomModes: Option<RoomMode>[] = [
  {
    id: "shared",
    label: "Shared with a customer",
    detail: "A Slack Connect channel: the other company's people are customers.",
  },
  { id: "community", label: "Community", detail: "Anyone who is not on your team is a customer." },
  { id: "helpdesk", label: "Help desk", detail: "Your own colleagues ask for help here, as customers." },
  {
    id: "internal",
    label: "Team only",
    detail: "Your team's own channel, for triage and discussion. Never opens conversations.",
  },
];

export const roomTriggers: Option<RoomTrigger>[] = [
  {
    id: "author_window",
    label: "Each new question",
    detail: "A person's messages close together become one conversation.",
  },
  { id: "every_message", label: "Every message", detail: "Each top-level message opens its own conversation." },
  {
    id: "ai",
    label: "Requests the AI spots",
    detail: "AI tells real requests from chatter. Without an AI key, each new question.",
  },
  {
    id: "emoji",
    label: "When someone reacts",
    detail: "Only messages that get the ticket emoji become conversations.",
  },
  {
    id: "mention",
    label: "When the app is mentioned",
    detail: "Only messages that mention the app become conversations.",
  },
  { id: "auto", label: "Every post", detail: "Every post becomes a conversation." },
  { id: "off", label: "Never", detail: "Watch the channel without opening conversations." },
];

export const agentModes: Option<NonNullable<RoomSettings["agent_mode"]>>[] = [
  { id: "auto", label: "Answer in the thread", detail: "The agent replies to the customer itself." },
  { id: "draft", label: "Draft for review", detail: "The agent drafts its answer and a teammate sends it." },
];

export const delayWhens: Option<NonNullable<RoomSettings["agent_delay_when"]>>[] = [
  { id: "always", label: "Always", detail: "Wait before every answer." },
  { id: "inside_hours", label: "During office hours", detail: "Wait only while your team is around to answer first." },
];

export const sensitivities: Option<NonNullable<RoomSettings["request_sensitivity"]>>[] = [
  { id: "conservative", label: "Only clear requests", detail: "Fewer conversations; casual questions may be missed." },
  { id: "aggressive", label: "Anything that might be one", detail: "More conversations; some chatter may slip in." },
];

export function optionLabel<T extends string>(options: Option<T>[], id: T): string {
  return options.find((option) => option.id === id)?.label ?? id;
}

/*
 * RoomDraft is a room's settings as its form holds them: every value
 * filled, with the default where the room has none. An automatic reply
 * of null is off.
 */
export interface RoomDraft {
  mode: RoomMode;
  trigger: RoomTrigger;
  agent_mode: NonNullable<RoomSettings["agent_mode"]>;
  agent_delay_seconds: number;
  agent_delay_when: NonNullable<RoomSettings["agent_delay_when"]>;
  triage_channel_id: string | null;
  auto_reply_after_seconds: number | null;
  auto_join: boolean;
  request_sensitivity: NonNullable<RoomSettings["request_sensitivity"]>;
}

export function draftOf(room: SlackRoom): RoomDraft {
  const s = room.settings;

  return {
    mode: room.mode,
    trigger: room.trigger,
    agent_mode: s.agent_mode ?? "auto",
    agent_delay_seconds: s.agent_delay_seconds ?? 0,
    agent_delay_when: s.agent_delay_when ?? "always",
    triage_channel_id: s.triage_channel_id ?? null,
    auto_reply_after_seconds: s.auto_reply_after_seconds ?? null,
    auto_join: s.auto_join ?? false,
    request_sensitivity: s.request_sensitivity ?? "conservative",
  };
}

type SettingKey = Exclude<keyof RoomDraft, "mode" | "trigger">;

/*
 * defaults are the settings a room follows when it holds none; a value
 * put back to one is cleared.
 */
const defaults: Record<SettingKey, unknown> = {
  agent_mode: "auto",
  agent_delay_seconds: 0,
  agent_delay_when: "always",
  triage_channel_id: null,
  auto_reply_after_seconds: null,
  auto_join: false,
  request_sensitivity: "conservative",
};

/*
 * roomChange is what saving the draft sends: the mode and trigger when
 * they changed, and each changed setting, null when it went back to its
 * default. Nothing changed gives null.
 */
export function roomChange(room: SlackRoom, draft: RoomDraft): RoomChange | null {
  const before = draftOf(room);
  const change: RoomChange = {};
  const settings: Record<string, unknown> = {};

  if (draft.mode !== before.mode) {
    change.mode = draft.mode;
  }

  if (draft.trigger !== before.trigger && draft.mode !== "internal") {
    change.trigger = draft.trigger;
  }

  for (const key of Object.keys(defaults) as SettingKey[]) {
    if (draft[key] !== before[key]) {
      settings[key] = draft[key] === defaults[key] ? null : draft[key];
    }
  }

  if (Object.keys(settings).length > 0) {
    change.settings = settings as RoomChange["settings"];
  }

  return Object.keys(change).length > 0 ? change : null;
}

/*
 * roomLine is the line under a room's name: who its customers are and
 * when a message becomes a conversation, or how to bring it back once
 * the app was removed from it.
 */
export function roomLine(room: SlackRoom): string {
  if (room.status === "removed") {
    return "Invite @dimmi to the channel again to keep watching it.";
  }

  return `${optionLabel(roomModes, room.mode)} · ${optionLabel(roomTriggers, room.trigger)}`;
}
