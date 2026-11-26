/*
 * The workflows a new one can start from: the jobs support teams set up
 * most, across the tools they come from. Each names its trigger and
 * builds the steps it starts with; the server fills in the rest of the
 * definition from the trigger, so a template only says what differs.
 * What only your workspace knows, such as a team, an SLA policy or who
 * counts as a top customer, opens empty, to pick before publishing.
 * Jobs a setting already does, such as which channels an agent answers
 * on or the note it leaves when it hands over, are not templates.
 */
import type { Spec } from "@/lib/api/workflows";

import { freshStep } from "./edit-steps";
import type { TriggerKind } from "./triggers";
import { newId, type Path, type Step } from "./workflows";

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

/*
 * path is a labelled way out of a step, with the steps it runs.
 */
function path(label: string, steps: Step[], extra?: Partial<Path>): Path {
  return { id: newId("p_"), label, steps, ...extra };
}

/*
 * branch is a step whose paths are the ones given, then everyone else.
 */
function branch(kind: "branch" | "classify" | "buttons", paths: Path[], settings?: Record<string, unknown>): Step {
  const fresh = step(kind, settings);
  const rest = (fresh.paths ?? []).filter((p) => p.outcome);

  return { ...fresh, paths: [...paths, ...rest] };
}

const assignTeam = () => step("assign", { mode: "team" });

export const workflowTemplates: WorkflowTemplate[] = [
  {
    id: "nudge_then_close",
    name: "Nudge, then close quiet conversations",
    description: "When the customer goes quiet for 2 days, check in. Still nothing 3 days later, close it.",
    trigger: "conversation.customer_quiet",
    build: (spec) => ({
      ...spec,
      trigger_settings: { after: "2d" },
      steps: [
        step("message", { text: "Just checking in: did that sort it out? Reply here if you still need a hand." }),
        step("wait", { for: "3d", interrupt_on: ["customer_message"] }),
        step("message", { text: "We'll close this for now. Write back any time and it opens again." }),
        step("close", { only_if_no_teammate_reply: true }),
      ],
    }),
  },
  {
    id: "route_by_topic",
    name: "Route by topic",
    description: "Sort each new conversation by what it's about and send it to the team that owns it.",
    trigger: "conversation.first_message",
    build: (spec) => ({
      ...spec,
      steps: [
        branch(
          "classify",
          [
            path("Billing", [assignTeam()], { description: "Invoices, payments, refunds, plans and pricing." }),
            path("Bug", [assignTeam()], { description: "Something in the product is broken or not working." }),
            path("Sales", [assignTeam()], { description: "Buying, upgrading, demos and quotes." }),
          ],
          { question: "What is the customer writing about?", source: "last_message", min_confidence: "medium" },
        ),
      ],
    }),
  },
  {
    id: "ask_first",
    name: "Ask what they need, then route",
    description: "Offer a few buttons first. Questions go to your AI agent, the rest to the right team.",
    trigger: "conversation.first_message",
    build: (spec) => ({
      ...spec,
      channels: ["widget"],
      steps: [
        branch(
          "buttons",
          [
            path("Get help", [step("dimmi")]),
            path("Talk to sales", [assignTeam()]),
            path("Report a bug", [assignTeam()]),
          ],
          {
            text: "Hi! What can we help you with?",
          },
        ),
      ],
    }),
  },
  {
    id: "top_customers",
    name: "Fast lane for top customers",
    description: "Your biggest customers get urgent priority, a faster SLA and their own team.",
    trigger: "conversation.first_message",
    build: (spec) => ({
      ...spec,
      steps: [
        branch("branch", [
          path("Top customer", [step("priority", { value: "urgent" }), step("apply_sla"), assignTeam()]),
        ]),
      ],
    }),
  },
  {
    id: "team_quiet",
    name: "Escalate when the team goes quiet",
    description: "After 4 hours with no reply, tell the customer when to expect one and raise the priority.",
    trigger: "conversation.teammate_quiet",
    build: (spec) => ({
      ...spec,
      trigger_settings: { after: "4h" },
      steps: [
        step("reply_time", { text: "Sorry for the wait. You'll hear back {{reply_time}}." }),
        step("priority", { value: "high" }),
        step("note", { text: "The customer has waited 4 hours for a reply." }),
      ],
    }),
  },
  {
    id: "sla_warning",
    name: "Warn before an SLA breaches",
    description: "When a reply target is about to be missed, make it urgent and send it to a lead.",
    trigger: "sla.warning",
    build: (spec) => ({
      ...spec,
      steps: [
        step("priority", { value: "urgent" }),
        step("note", { text: "This conversation is about to miss its reply target." }),
        assignTeam(),
      ],
    }),
  },
  {
    id: "low_rating",
    name: "Follow up on a bad rating",
    description: "When a customer rates a conversation 2 or lower, send it to a lead to follow up.",
    trigger: "conversation.rated",
    build: (spec) => ({
      ...spec,
      steps: [
        branch("branch", [
          path(
            "Low rating",
            [
              step("priority", { value: "high" }),
              step("note", { text: "The customer rated this conversation 2 or lower. Follow up with them." }),
              assignTeam(),
            ],
            { condition: { field: "rating.score", op: "lte", value: 2 } },
          ),
        ]),
      ],
    }),
  },
  {
    id: "churn_risk",
    name: "Catch customers at risk of leaving",
    description: "When the AI flags an upset customer or one who wants to cancel, escalate it.",
    trigger: "conversation.ai_attribute_changed",
    build: (spec) => ({
      ...spec,
      steps: [
        branch("branch", [
          path("At risk", [
            step("priority", { value: "urgent" }),
            step("note", { text: "This customer may be about to leave. Reply personally." }),
            assignTeam(),
          ]),
        ]),
      ],
    }),
  },
];
