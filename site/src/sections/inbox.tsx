import { ChartLineUp, ChatsCircle, FlowArrow, Megaphone, SquaresFour, UserFocus } from "@phosphor-icons/react";

import { Features, Intro, Section } from "../components/container";
import { ProductShot } from "../components/product-shot";

const features = [
  {
    icon: ChatsCircle,
    title: "Every channel, one reply box",
    body: "Messenger, email, Slack, WhatsApp, SMS, and phone arrive in one queue instead of five tabs.",
    link: { href: "/inbox", label: "Explore the inbox" },
  },
  {
    icon: UserFocus,
    title: "Customer info beside the thread",
    body: "See their plan, recent app events, and past tickets right next to your reply.",
    link: { href: "/inbox", label: "See customer details" },
  },
  {
    icon: FlowArrow,
    title: "Visual workflows included",
    body: "Draw routing, SLA alerts, and follow-ups on a canvas, or start from a template.",
    link: { href: "/workflows", label: "Explore workflows" },
  },
  {
    icon: Megaphone,
    title: "Broadcasts people can reply to",
    body: "Send updates from your own domain. Replies land right back in your inbox.",
    link: { href: "/broadcasts", label: "See broadcasts" },
  },
  {
    icon: SquaresFour,
    title: "Support inside your app",
    body: "Add the chat script in a minute, use our framework packages, or build your own UI.",
    link: { href: "/#developers", label: "Explore the SDKs" },
  },
  {
    icon: ChartLineUp,
    title: "Topics that guide your roadmap",
    body: "Group chats by topic so your team sees what breaks most often.",
  },
];

export function Inbox() {
  return (
    <Section anchor="product" frameClassName="flex flex-col gap-16 md:gap-20">
      <Intro
        title="One inbox your whole company can afford to open."
        body="Every channel lands in one queue beside each customer's history. Seats are free, so any engineer can jump into a bug thread."
      />
      <ProductShot name="inbox" alt="The dimmi inbox: conversation list, thread and customer details" />
      <Features items={features} />
    </Section>
  );
}
