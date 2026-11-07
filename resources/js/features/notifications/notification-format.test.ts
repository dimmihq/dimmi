import type { MemberNotification } from "@/lib/api/notifications";

import { centerItem, isPendingApproval, pathOf } from "./notification-format";

const base: MemberNotification = {
  id: "ntf_1",
  object: "notification",
  trigger: "customer_replied",
  title: "Ada replied",
  body: "Any news?",
  link: "https://app.example.com/d/acme/inbox/all/conv_1",
  actor_name: "",
  conversation_id: "conv_1",
  action_call_id: null,
  count: 3,
  read: false,
  seen: false,
  created_at: "2026-11-07T10:00:00Z",
  updated_at: "2026-11-07T10:00:00Z",
};

describe("notification format", () => {
  it("shows how often a thing happened and that it is unread", () => {
    const item = centerItem(base, new Date("2026-11-07T10:05:00Z"));
    expect(item.timestamp).toContain("3 times");
    expect(item.unread).toBe(true);
    expect(item.actions).toBeUndefined();
  });

  it("offers approve and decline only while nobody decided", () => {
    const waiting = { ...base, trigger: "ai_action_approval_requested", action_call_id: "aax_1" };
    expect(isPendingApproval(waiting)).toBe(true);
    expect(centerItem(waiting, new Date(), "approve").actions?.[0]).toMatchObject({ id: "approve", loading: true });
    const decided = centerItem({ ...waiting, resolved: "declined" }, new Date());
    expect(decided.actions).toBeUndefined();
    expect(decided.note).toBe("Declined");
  });

  it("turns a link into an address inside the app", () => {
    expect(pathOf(base.link)).toBe("/d/acme/inbox/all/conv_1");
    expect(pathOf("/d/x")).toBe("/d/x");
  });
});
