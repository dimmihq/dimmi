/*
 * The choices the agent settings offer, each in the words a person
 * reads, with the value the server stores, and the small rules that go
 * with them: the order channels are kept in, how a typed allowlist is
 * read, and which model a connection suggests.
 */
import type { AiModelEntry } from "@/lib/api/ai";

/*
 * tones are the voices the agent can take, each with what it sounds
 * like.
 */
export const tones = [
  { id: "friendly", label: "Friendly", line: "Warm and plain-spoken, like a helpful colleague." },
  { id: "professional", label: "Professional", line: "Polite and precise, without small talk." },
  { id: "concise", label: "Concise", line: "The answer first, in as few words as it takes." },
] as const;

export const channels = [
  { id: "widget", label: "Messenger", line: "The chat on your site and in your app." },
  { id: "email", label: "Email", line: "Replies to mail sent to your support addresses." },
  { id: "api", label: "API", line: "Conversations your own systems start." },
] as const;

export const schedules = [
  { id: "always", label: "Always", line: "Day and night, every day." },
  { id: "inside_office_hours", label: "During office hours", line: "While the team is in, to take the first reply." },
  { id: "outside_office_hours", label: "Outside office hours", line: "Only while the team is away." },
] as const;

export const assignedBehaviors = [
  { id: "skip_when_human_active", label: "Step back while a teammate is active" },
  { id: "never_when_assigned", label: "Stop once someone is assigned" },
  { id: "always", label: "Keep answering" },
] as const;

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

export const unknownBehaviors = [
  { id: "offer_handoff", label: "Offer a teammate" },
  { id: "handoff", label: "Hand over straight away" },
] as const;

export const contactContexts = [
  { id: "basic", label: "Basic", line: "Name, plan and company, so answers can be personal." },
  { id: "none", label: "None", line: "Nothing about the customer beyond the conversation." },
] as const;

/*
 * domains reads the allowlist as typed: one site per line or between
 * commas, without the scheme.
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
