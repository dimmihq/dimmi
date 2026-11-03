/*
 * Workflows as the builder shows them, and the example workspace's
 * workflows. A workflow starts from one trigger and runs its steps in
 * order; steps with paths (reply buttons, branches, handing to Dimmi)
 * continue down whichever path applies. Dimmi joins a conversation only
 * through a "Let Dimmi handle" step, whose outcomes are paths of their
 * own. Until workflows run on the server these live in the browser.
 */
export type StepKind =
  | "message"
  | "buttons"
  | "collect"
  | "branch"
  | "dimmi"
  | "assign"
  | "tag"
  | "priority"
  | "wait"
  | "reply-time"
  | "note"
  | "close";

export interface Path {
  id: string;
  label: string;
  steps: Step[];
}

export interface Step {
  id: string;
  kind: StepKind;
  /* What the step does, in the words the card shows. */
  summary: string;
  /* The settings chosen in the inspector, once any were changed. */
  fields?: Record<string, string>;
  paths?: Path[];
}

export type TriggerKind = "opened" | "first-message" | "customer-quiet" | "teammate-quiet" | "state" | "reusable";

export interface Workflow {
  id: string;
  name: string;
  trigger: TriggerKind;
  channels: string;
  status: "live" | "draft" | "paused";
  edited: string;
  runs: string;
  version: number;
  steps: Step[];
}

export const triggerLabel: Record<TriggerKind, string> = {
  opened: "Customer opens the messenger",
  "first-message": "Customer sends their first message",
  "customer-quiet": "Customer stops replying",
  "teammate-quiet": "Teammate stops replying",
  state: "Conversation state changes",
  reusable: "Run from another workflow or the inbox",
};

export const workflows: Workflow[] = [
  {
    id: "first-message",
    name: "Let Dimmi answer first messages",
    trigger: "first-message",
    channels: "Messenger, email",
    status: "live",
    edited: "Edited 2 days ago by Sam Okafor",
    runs: "1,204 runs this month",
    version: 3,
    steps: [
      {
        id: "s1",
        kind: "buttons",
        summary: "What can we help you with?",
        paths: [
          {
            id: "p1",
            label: "Billing",
            steps: [
              { id: "s2", kind: "assign", summary: "Assign to the Billing team" },
              { id: "s3", kind: "reply-time", summary: "Show the team's reply time" },
            ],
          },
          {
            id: "p2",
            label: "Something else",
            steps: [
              {
                id: "s4",
                kind: "dimmi",
                summary: "Dimmi answers from your knowledge and procedures",
                paths: [
                  {
                    id: "p3",
                    label: "Resolved",
                    steps: [{ id: "s5", kind: "close", summary: "Close the conversation" }],
                  },
                  {
                    id: "p4",
                    label: "Hands over",
                    steps: [
                      { id: "s6", kind: "collect", summary: 'Ask "What\'s the best email to reach you?" → Email' },
                      { id: "s7", kind: "assign", summary: "Assign to the Support team" },
                      { id: "s8", kind: "reply-time", summary: "Show the team's reply time" },
                    ],
                  },
                  {
                    id: "p5",
                    label: "No answer",
                    steps: [{ id: "s9", kind: "assign", summary: "Assign to the Support team" }],
                  },
                ],
              },
            ],
          },
        ],
      },
    ],
  },
  {
    id: "vip",
    name: "Fast lane for VIP customers",
    trigger: "first-message",
    channels: "All channels",
    status: "draft",
    edited: "Edited today by you",
    runs: "Never run",
    version: 1,
    steps: [
      {
        id: "v1",
        kind: "branch",
        summary: "Company plan",
        paths: [
          {
            id: "v2",
            label: "Plan is Enterprise",
            steps: [
              { id: "v3", kind: "tag", summary: "Tag VIP" },
              { id: "v4", kind: "priority", summary: "Mark as priority" },
              { id: "v5", kind: "assign", summary: "Assign to Sam Okafor" },
            ],
          },
          {
            id: "v6",
            label: "Everyone else",
            steps: [{ id: "v7", kind: "assign", summary: "Assign to the Support team" }],
          },
        ],
      },
    ],
  },
  {
    id: "nudge",
    name: "Nudge quiet customers",
    trigger: "customer-quiet",
    channels: "Messenger",
    status: "paused",
    edited: "Edited last week by Lea Martin",
    runs: "312 runs this month",
    version: 2,
    steps: [
      { id: "n1", kind: "wait", summary: "Wait 10 minutes" },
      { id: "n2", kind: "message", summary: "Still there? Reply and we'll pick up where we left off." },
      { id: "n3", kind: "wait", summary: "Wait 1 day" },
      { id: "n4", kind: "close", summary: "Close the conversation" },
    ],
  },
  {
    id: "handover-note",
    name: "Brief the team on handover",
    trigger: "reusable",
    channels: "All channels",
    status: "live",
    edited: "Edited 3 weeks ago by Sam Okafor",
    runs: "88 runs this month",
    version: 1,
    steps: [
      { id: "h1", kind: "note", summary: "Add Operator's summary as an internal note" },
      { id: "h2", kind: "assign", summary: "Assign to the Support team" },
    ],
  },
];

/*
 * templates are workflows a workspace can start from.
 */
export const templates = [
  { id: "t1", name: "Route by topic", detail: "Ask what the customer needs and send them to the right team" },
  { id: "t2", name: "Dimmi first, then the team", detail: "Let Dimmi answer, hand over with the details collected" },
  { id: "t3", name: "Close inactive conversations", detail: "Nudge, then close after a quiet day" },
];

/*
 * triggerStep names the trigger card when it is the selected thing in the
 * builder, next to the steps' own ids.
 */
export const triggerStep = "trigger";

export function findWorkflow(id: string): Workflow | undefined {
  return workflows.find((workflow) => workflow.id === id);
}

/*
 * findStep returns the step with an id anywhere in a list of steps,
 * including down every path.
 */
export function findStep(steps: Step[], id: string): Step | undefined {
  for (const step of steps) {
    if (step.id === id) {
      return step;
    }
    for (const path of step.paths ?? []) {
      const inner = findStep(path.steps, id);
      if (inner) {
        return inner;
      }
    }
  }

  return undefined;
}
