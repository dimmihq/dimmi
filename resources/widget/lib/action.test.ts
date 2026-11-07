import { describe, expect, it } from "vitest";

import { Store } from "../client/store";
import { ActionBlock, Thread } from "../components/thread";
import type { ActionCard } from "../content";
import { actionTexts } from "../text";
import { toAction, toMessage } from "./map";

const card = (state: ActionCard["state"]): ActionCard => ({
  id: "aax_1",
  card: true,
  title: "Refund an invoice",
  text: "Refund €144.00 for invoice inv_1001?",
  details: [{ label: "Invoice", value: "inv_1001" }],
  state,
});

function render(markup: unknown): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = String(markup);
  return root;
}

describe("action cards", () => {
  it("reads the card or call an answer carries in either spelling", () => {
    expect(toAction(null)).toBeUndefined();
    expect(
      toAction({ confirmation: { action_call_id: "aax_1", text: "Refund?", status: "pending_confirmation" } }),
    ).toMatchObject({ id: "aax_1", card: true, text: "Refund?", state: "ask" });
    expect(toAction({ confirmation: { action_call_id: "aax_1", status: "canceled" } })?.state).toBe("canceled");
    expect(toAction({ action_call: { id: "aax_2", state: "pending_approval", summary: "Refund" } })).toMatchObject({
      card: false,
      state: "waiting",
      text: "Refund",
    });
    expect(toAction({ action_call: { id: "aax_2", state: "odd" } })?.state).toBe("waiting");
  });

  it("asks the visitor to confirm or cancel", () => {
    const root = render(ActionBlock({ action: card("ask"), texts: actionTexts }));
    const buttons = [...root.querySelectorAll<HTMLButtonElement>("[data-call]")];

    expect(root.querySelector(".action-text")?.textContent).toBe("Refund €144.00 for invoice inv_1001?");
    expect(root.querySelector("dd")?.textContent).toBe("inv_1001");
    expect(buttons.map((b) => [b.dataset.decision, b.disabled])).toEqual([
      ["confirmed", false],
      ["canceled", false],
    ]);
  });

  it("waits with a spinner while the choice is sending", () => {
    const root = render(ActionBlock({ action: card("sending"), texts: actionTexts }));

    expect(root.querySelector(".spinner")).not.toBeNull();
    expect([...root.querySelectorAll("button")].every((b) => b.disabled)).toBe(true);
  });

  it("says where the call stands once decided", () => {
    const waiting = render(ActionBlock({ action: card("waiting"), texts: actionTexts }));
    expect(waiting.querySelector("button")).toBeNull();
    expect(waiting.querySelector(".action-state")?.textContent).toBe("Waiting for the team to review this");

    const line = render(ActionBlock({ action: { ...card("declined"), card: false }, texts: actionTexts }));
    expect(line.querySelector(".action-card")).toBeNull();
    expect(line.textContent).toBe("Not approved");
    expect(String(ActionBlock({ action: { ...card("ask"), card: false }, texts: actionTexts }))).toBe("");
  });

  it("does not repeat the card's question in the bubble", () => {
    const message = toMessage(
      {
        id: "msg_1",
        conversation_id: "conv_1",
        author: { type: "agent", name: "dimmi", avatar_url: null },
        body: "I can do that.\n\nRefund €144.00 for invoice inv_1001?",
        attachments: [],
        created_at: "2026-11-07T10:00:00Z",
        metadata: {
          ai: {
            confirmation: {
              action_call_id: "aax_1",
              text: "Refund €144.00 for invoice inv_1001?",
              status: "pending_confirmation",
            },
          },
        },
      },
      Date.parse("2026-11-07T10:01:00Z"),
    );
    const root = render(Thread({ messages: [message] }));

    expect(root.querySelector(".bubble")?.textContent?.trim()).toBe("I can do that.");
    expect(root.querySelectorAll(".action-text")).toHaveLength(1);
  });

  it("moves a message's call to its new status", () => {
    const store = new Store(() => 0);
    store.setMessages("conv_1", [
      {
        id: "msg_1",
        conversation_id: "conv_1",
        author: { type: "agent", name: "dimmi", avatar_url: null },
        body: "Refund?",
        attachments: [],
        created_at: "2026-11-07T10:00:00Z",
        metadata: { ai: { confirmation: { action_call_id: "aax_1", status: "pending_confirmation" } } },
      },
    ]);

    store.setCallStatus("conv_1", "aax_1", "pending_approval");
    store.setCallStatus("conv_1", "aax_9", "confirmed");

    const message = store.messages.get("conv_1")?.[0];
    expect(message?.metadata?.ai?.confirmation?.status).toBe("pending_approval");
    expect(toAction(message?.ai)?.state).toBe("waiting");
  });
});
