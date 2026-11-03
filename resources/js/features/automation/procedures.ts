/*
 * Procedures are step-by-step instructions Dimmi follows for one kind of
 * request, written in plain words: when to use the procedure, what to do
 * in order, which actions it may take, and when to hand over to a person.
 * Saved test conversations check a procedure still behaves after edits;
 * running them calls the workspace's model, so a person starts each run.
 * Until procedures run on the server, these are the example workspace's.
 */
export interface ProcedureStep {
  id: string;
  kind: "instruction" | "condition" | "action";
  text: string;
}

export interface ProcedureTest {
  id: string;
  name: string;
  result: "passed" | "failed" | "not run";
}

export interface Procedure {
  id: string;
  name: string;
  when: string;
  status: "live" | "draft";
  edited: string;
  used: string;
  actions: string[];
  steps: ProcedureStep[];
  handover: string[];
  tests: ProcedureTest[];
}

export const procedures: Procedure[] = [
  {
    id: "sign-in-code",
    name: "Send a sign-in code",
    when: "The customer cannot sign in because a password or reset link fails.",
    status: "live",
    edited: "Edited yesterday by Sam Okafor",
    used: "Used in 64 conversations this month",
    actions: ["Look up an account", "Send a sign-in code"],
    steps: [
      {
        id: "a1",
        kind: "instruction",
        text: "Ask which email address they sign in with, unless the conversation already shows it.",
      },
      { id: "a2", kind: "action", text: "Look up an account with that email." },
      {
        id: "a3",
        kind: "condition",
        text: "If no account matches, say so and offer to check another address. Stop after two tries.",
      },
      { id: "a4", kind: "action", text: "Send a sign-in code to the account's email." },
      { id: "a5", kind: "instruction", text: "Tell them the code arrives within a minute and works for 10 minutes." },
    ],
    handover: ["The customer says they no longer have access to that email.", "Two sign-in codes did not arrive."],
    tests: [
      { id: "t1", name: "Password manager keeps an old password", result: "passed" },
      { id: "t2", name: "Typo in the email address", result: "passed" },
      { id: "t3", name: "Lost access to the mailbox", result: "failed" },
    ],
  },
  {
    id: "duplicate-charge",
    name: "Refund a duplicate charge",
    when: "The customer was charged twice for the same invoice.",
    status: "draft",
    edited: "Edited today by you",
    used: "Not live yet",
    actions: ["Look up invoices", "Refund a payment"],
    steps: [
      { id: "b1", kind: "action", text: "Look up the customer's invoices from the last 60 days." },
      { id: "b2", kind: "condition", text: "If two payments share an invoice, refund the later one." },
      { id: "b3", kind: "instruction", text: "Confirm the refund and say it takes 5 to 10 days to appear." },
    ],
    handover: ["The refund is over $500.", "The customer disputes more than one invoice."],
    tests: [{ id: "t4", name: "Two charges for October", result: "not run" }],
  },
  {
    id: "verify-identity",
    name: "Verify identity before account changes",
    when: "The customer asks to change their email, owner or billing details.",
    status: "live",
    edited: "Edited 2 weeks ago by Lea Martin",
    used: "Used in 9 conversations this month",
    actions: ["Look up an account"],
    steps: [
      {
        id: "c1",
        kind: "condition",
        text: "If the customer is signed in through the messenger, continue. Otherwise hand over.",
      },
      { id: "c2", kind: "instruction", text: "Repeat the change back to them and ask them to confirm." },
    ],
    handover: ["The customer is not signed in.", "The change is to the workspace owner."],
    tests: [],
  },
];

export function findProcedure(id: string): Procedure | undefined {
  return procedures.find((procedure) => procedure.id === id);
}
