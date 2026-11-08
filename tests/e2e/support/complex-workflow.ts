/*
 * A workflow with many paths, nested branches and fixed outcomes, for
 * checking that the builder lays it out the same way every time. Team
 * and tag ids come from the workspace it is made in.
 */
export function complexWorkflow(team: string, tags: string[]) {
  const step = (id: string, kind: string, settings: Record<string, unknown> = {}, paths?: unknown[]) => ({
    id,
    kind,
    settings,
    ...(paths ? { paths } : {}),
  });
  const path = (id: string, label: string, steps: unknown[], condition?: unknown) => ({
    id,
    label,
    steps,
    ...(condition ? { condition } : {}),
  });
  const fixed = (id: string, outcome: string, steps: unknown[]) => ({ id, outcome, steps });

  return {
    schema: 1,
    trigger: "conversation.first_message",
    schedule: { mode: "any" },
    reentry: { mode: "once_per_conversation" },
    exclusive: false,
    steps: [
      step("s_greet00001", "message", { text: "Hi {{contact.first_name}}, thanks for writing in." }),
      step("s_menu000001", "buttons", { text: "What can we help with?" }, [
        path("p_billing001", "Billing", [
          step("s_plan000001", "branch", {}, [
            path(
              "p_enterpr001",
              "Enterprise",
              [
                step("s_tagvip0001", "tag", { tag_ids: tags.slice(0, 1) }),
                step("s_prio000001", "priority", { value: "urgent" }),
                step("s_asgbill001", "assign", { mode: "team", team_id: team }),
              ],
              { field: "company.name", op: "contains_any", value: ["Acme"] },
            ),
            path(
              "p_refund0001",
              "Refund",
              [
                step("s_email00001", "collect", {
                  text: "What's the best email to reach you?",
                  field: "contact.email",
                  type: "email",
                }),
                step("s_note000001", "note", { text: "Refund asked for by {{contact.email}}" }),
              ],
              { field: "message.body", op: "contains_any", value: ["refund", "money back"] },
            ),
            fixed("p_else000001", "else", [
              step("s_asgbill002", "assign", { mode: "team_round_robin", team_id: team }),
            ]),
          ]),
        ]),
        path("p_bug0000001", "Bug report", [
          step("s_bugname001", "collect", { text: "What's your name?", field: "contact.name", type: "text" }),
          step("s_wait000001", "wait", { for: "1h" }),
          step("s_tagbug0001", "tag", { tag_ids: tags.slice(1, 3) }),
        ]),
        path("p_sales00001", "Sales", [
          step("s_sales00001", "message", { text: "A sales teammate will reply shortly." }),
          step("s_asgsale001", "assign", { mode: "team_balanced", team_id: team }),
        ]),
        path("p_other00001", "Something else", [
          step("s_vip0000001", "branch", {}, [
            path("p_vipdom0001", "Acme domain", [step("s_snooze0001", "snooze", { for: "1d" })], {
              field: "contact.email_domain",
              op: "eq",
              value: "acme.com",
            }),
            fixed("p_else000002", "else", [step("s_close00001", "close", {})]),
          ]),
        ]),
        fixed("p_typed00001", "typed", [
          step("s_note000002", "note", { text: "The customer typed their own answer." }),
        ]),
      ]),
    ],
  };
}
