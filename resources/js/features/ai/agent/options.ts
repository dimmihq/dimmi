/*
 * The choices the agent settings offer, one for every value the server
 * accepts, each in the words a person reads, and the small rules that go
 * with them: the order channels are kept in, how a typed allowlist is
 * read, and which model a connection suggests.
 */
import {
  AI_ASSIGNED_BEHAVIORS,
  AI_CHANNELS,
  AI_CONTACT_CONTEXTS,
  AI_SCHEDULES,
  AI_TONES,
  AI_UNKNOWN_BEHAVIORS,
  type AiAssignedBehavior,
  type AiChannel,
  type AiContactContext,
  type AiModelEntry,
  type AiSchedule,
  type AiTone,
  type AiUnknownBehavior,
} from "@/lib/api/ai";

/*
 * Choice is one value with its label and, where it helps, a line saying
 * what it means.
 */
export interface Choice<T extends string> {
  id: T;
  label: string;
  line: string;
}

function choices<T extends string>(values: readonly T[], words: Record<T, [string, string]>): Choice<T>[] {
  return values.map((id) => ({ id, label: words[id][0], line: words[id][1] }));
}

export const tones = choices<AiTone>(AI_TONES, {
  friendly: ["Friendly", "Warm and plain-spoken, like a helpful colleague."],
  professional: ["Professional", "Polite and precise, without small talk."],
  concise: ["Concise", "The answer first, in as few words as it takes."],
});

export const channels = choices<AiChannel>(AI_CHANNELS, {
  widget: ["Messenger", "The chat on your site and in your app."],
  email: ["Email", "Replies to mail sent to your support addresses."],
  api: ["API", "Conversations your own systems start."],
});

export const schedules = choices<AiSchedule>(AI_SCHEDULES, {
  always: ["Always", "Day and night, every day."],
  outside_office_hours: ["Outside office hours", "Only while the team is away."],
  inside_office_hours: ["During office hours", "While the team is in, to take the first reply."],
});

export const assignedBehaviors = choices<AiAssignedBehavior>(AI_ASSIGNED_BEHAVIORS, {
  skip_when_human_active: ["Step back while a teammate is active", ""],
  never_when_assigned: ["Stop once someone is assigned", ""],
  always: ["Keep answering", ""],
});

export const unknownBehaviors = choices<AiUnknownBehavior>(AI_UNKNOWN_BEHAVIORS, {
  offer_handoff: ["Offer a teammate", ""],
  handoff: ["Hand over straight away", ""],
  say_unknown: ["Just say it doesn't know", ""],
});

export const contactContexts = choices<AiContactContext>(AI_CONTACT_CONTEXTS, {
  none: ["None", "Nothing about the customer beyond the conversation."],
  basic: ["Basic", "Name, plan and company, so answers can be personal."],
});

/*
 * toggled is the channels with one switched on or off, in the order the
 * page lists them.
 */
export function toggled(current: string[], channel: string, on: boolean): string[] {
  const next = new Set(current);
  if (on) {
    next.add(channel);
  } else {
    next.delete(channel);
  }

  return channels.map((c) => c.id as string).filter((id) => next.has(id));
}

/*
 * domains reads the allowlist as typed: one site per line or between
 * commas, without the scheme. "*.example.com" keeps its star and covers
 * every subdomain.
 */
export function domains(text: string): string[] {
  return [
    ...new Set(
      text
        .split(/[\s,]+/)
        .map((d) =>
          d
            .trim()
            .replace(/^https?:\/\//, "")
            .replace(/\/.*$/, "")
            .toLowerCase(),
        )
        .filter(Boolean),
    ),
  ];
}

/*
 * preferred is the model a connection suggests for a role, or its first.
 */
export function preferred(models: AiModelEntry[], role: string): AiModelEntry | undefined {
  return models.find((model) => model.suggested.includes(role)) ?? models[0];
}
