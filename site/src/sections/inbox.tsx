import { ChartLineUp, ChatsCircle, FlowArrow, Megaphone, SquaresFour, UserFocus } from "@phosphor-icons/react";

import { Features, Intro, Section } from "../components/container";
import { ProductShot } from "../components/product-shot";

const features = [
  {
    icon: ChatsCircle,
    title: "Every channel in one queue",
    body: "Messenger, email, Slack, Teams, Discord, WhatsApp, SMS, and phone land in one list, with one reply box.",
    link: { href: "/inbox", label: "See the inbox" },
  },
  {
    icon: UserFocus,
    title: "The customer beside the thread",
    body: "Company, plan, product events, and past conversations sit next to every reply. No second tab.",
    link: { href: "/inbox", label: "See customer context" },
  },
  {
    icon: FlowArrow,
    title: "Workflows you draw",
    body: "Routing, SLAs, and follow-ups on a canvas. Start from a template or a blank one.",
    link: { href: "/workflows", label: "See workflows" },
  },
  {
    icon: Megaphone,
    title: "Broadcasts",
    body: "Tell customers about incidents and releases. Replies come back as conversations, not into a void.",
  },
  {
    icon: SquaresFour,
    title: "Support inside your product",
    body: "Drop in the messenger, or build your own UI on the headless SDK.",
    link: { href: "/#developers", label: "See the SDKs" },
  },
  {
    icon: ChartLineUp,
    title: "Know what people ask about",
    body: "Conversations grouped by topic, so product knows what to build and what to fix.",
  },
];

export function Inbox() {
  return (
    <Section anchor="product" frameClassName="flex flex-col gap-16 md:gap-20">
      <Intro
        title="One queue for every conversation."
        body="Your team and the AI agent work the same threads, with the customer and their history beside every reply."
      />
      <ProductShot name="inbox" alt="The dimmi inbox: conversation list, thread and customer details" />
      <Features items={features} />
    </Section>
  );
}
