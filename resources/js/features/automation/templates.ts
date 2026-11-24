/*
 * The workflows a new one can start from. Each names its trigger and
 * builds the steps it starts with; the server fills in the rest of the
 * definition from the trigger, so a template only says what differs.
 * Steps that need a choice only your workspace has, such as an agent,
 * open with that choice to make before publishing.
 */
import type { Spec } from "@/lib/api/workflows";

import { freshStep } from "./edit-steps";
import type { TriggerKind } from "./triggers";
import type { Step } from "./workflows";

export interface WorkflowTemplate {
  id: string;
  name: string;
  description: string;
  trigger: TriggerKind;
  build: (spec: Spec) => Spec;
}

function step(kind: Parameters<typeof freshStep>[0], settings?: Record<string, unknown>): Step {
  const fresh = freshStep(kind);

  return settings ? { ...fresh, settings: { ...fresh.settings, ...settings } } : fresh;
}

export const workflowTemplates: WorkflowTemplate[] = [
  {
    id: "greet_then_agent",
    name: "Greet, then hand to your AI agent",
    description: "Say hello on the first message, then let the agent placed on the channel answer.",
    trigger: "conversation.first_message",
    build: (spec) => ({
      ...spec,
      steps: [step("message", { text: "Hi, thanks for writing in. Someone will be with you shortly." }), step("dimmi")],
    }),
  },
  {
    id: "out_of_hours",
    name: "Take an email out of hours",
    description: "Outside office hours, say when you're back and ask where to reply.",
    trigger: "conversation.first_message",
    build: (spec) => ({
      ...spec,
      schedule: { mode: "closed" },
      steps: [step("reply_time", { text: "We're away right now. You'll hear back {{reply_time}}." }), step("collect")],
    }),
  },
  {
    id: "ai_off_email",
    name: "Keep AI off email",
    description: "Leave every email conversation to your team, while AI answers chat.",
    trigger: "conversation.first_message",
    build: (spec) => ({ ...spec, channels: ["email"], steps: [step("turn_off_ai")] }),
  },
  {
    id: "brief_after_handover",
    name: "Brief the team after a handover",
    description: "Raise the priority and leave a note when an AI agent hands over.",
    trigger: "dimmi.handed_over",
    build: (spec) => ({
      ...spec,
      steps: [
        step("priority", { value: "high" }),
        step("note", { text: "An AI agent handed this over. Its summary is in the thread." }),
      ],
    }),
  },
  {
    id: "thank_after_resolve",
    name: "Thank the customer when the agent resolves",
    description: "Send a short thank-you once the customer confirms the answer worked.",
    trigger: "dimmi.resolved",
    build: (spec) => ({
      ...spec,
      steps: [step("message", { text: "Glad that's sorted. Write back here any time." })],
    }),
  },
  {
    id: "sla_breach",
    name: "Escalate a breached SLA",
    description: "Mark the conversation urgent and leave a note for whoever picks it up.",
    trigger: "sla.breached",
    build: (spec) => ({
      ...spec,
      steps: [
        step("priority", { value: "urgent" }),
        step("note", { text: "This conversation missed its reply target." }),
      ],
    }),
  },
];
