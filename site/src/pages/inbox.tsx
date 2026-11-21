import {
  ArrowCounterClockwise,
  At,
  BookOpenText,
  ChartPieSlice,
  Checks,
  DeviceMobile,
  Files,
  FlowArrow,
  GitMerge,
  Keyboard,
  Smiley,
  Sparkle,
  Ticket,
  Timer,
  Translate,
} from "@phosphor-icons/react";
import {
  RiChat3Line,
  RiCodeSSlashLine,
  RiDiscordLine,
  RiMailLine,
  RiMessage2Line,
  RiMicrosoftLine,
  RiPhoneLine,
  RiSlackLine,
  RiWhatsappLine,
} from "@remixicon/react";

import {
  ChannelStrip,
  type Feature,
  FeatureCards,
  FeatureGrid,
  ProductHero,
  Questions,
} from "../components/product-page";
import { FinalCta } from "../sections/final-cta";

const features: Feature[] = [
  {
    title: "Assign with one click",
    body: "Hand a chat to a person or team, or let round robin split new threads fairly.",
    flow: "assign",
    alt: "Assigning a conversation to a teammate",
    anchor: "top-right",
  },
  {
    title: "Snooze until it matters",
    body: "Clear your queue for now. The thread comes back on time, or sooner if they reply.",
    flow: "snooze",
    alt: "Snoozing a conversation until tomorrow",
    anchor: "top-right",
  },
  {
    title: "Scan the whole queue fast",
    body: "See the channel, owner, and wait time on every row without clicking in.",
    flow: "queue",
    alt: "Switching between open, snoozed and closed conversations",
    anchor: "top",
  },
  {
    title: "Customer info next to every chat",
    body: "Plan, tags, and priority sit up top, with past chats and app events right below.",
    flow: "details",
    alt: "Setting a conversation's priority from the details panel",
    anchor: "top",
  },
  {
    title: "Operator sums up long threads",
    body: "Skip reading 40 messages. Ask Operator for a quick brief or a draft reply.",
    flow: "operator",
    alt: "Asking Operator about the customer",
    anchor: "bottom",
  },
];

const more = [
  {
    icon: Keyboard,
    title: "Keyboard shortcuts",
    body: "Every action has a shortcut, and the command menu finds the ones you forget.",
  },
  {
    icon: ArrowCounterClockwise,
    title: "Undo send",
    body: "Catch a typo and pull a reply back before it goes out.",
  },
  {
    icon: Files,
    title: "Saved replies",
    body: "Send a common answer, then tag, assign, or close the chat in one move.",
  },
  {
    icon: Checks,
    title: "Bulk actions",
    body: "Assign, tag, or close dozens of conversations in one shot.",
  },
  {
    icon: GitMerge,
    title: "Merge threads",
    body: "Same customer on chat and email? Combine both into one thread.",
  },
  {
    icon: Smiley,
    title: "Customer ratings",
    body: "Ask how the chat went right when you close it, in chat or email.",
  },
  {
    icon: Timer,
    title: "SLAs",
    body: "Set reply targets by plan or priority and get warned before one slips.",
  },
  {
    icon: Ticket,
    title: "Tickets",
    body: "Turn a support chat into tracked work linked to Linear, Jira, GitHub, and more.",
  },
  {
    icon: ChartPieSlice,
    title: "Reports",
    body: "Track volume, reply times, and CSAT by team, channel, and tag.",
  },
  {
    icon: At,
    title: "Internal notes",
    body: "Tag a teammate right inside the thread. Customers never see notes.",
  },
  {
    icon: Translate,
    title: "Live translation",
    body: "Read and write in your language while customers read in theirs.",
  },
  {
    icon: DeviceMobile,
    title: "Mobile apps",
    body: "Answer from iOS and Android, with push alerts for threads assigned to you.",
  },
];

const related = [
  {
    icon: Sparkle,
    title: "AI agent",
    body: "Takes the first reply from your docs and hands over with a summary.",
    link: { href: "/agent", label: "Explore the AI agent" },
  },
  {
    icon: FlowArrow,
    title: "Workflows",
    body: "Route chats, trigger SLAs, and send follow-ups without manual work.",
    link: { href: "/workflows", label: "Explore workflows" },
  },
  {
    icon: BookOpenText,
    title: "Help center",
    body: "Publish articles on your domain that help both customers and the AI.",
    link: { href: "/help-center", label: "Explore the help center" },
  },
];

const questions = [
  {
    question: "Which channels land in the inbox?",
    answer:
      "All of them: the messenger on your site or app, email, Slack, Microsoft Teams, Discord, WhatsApp, SMS, Instagram, Facebook Messenger, phone calls, and the REST API.",
  },
  {
    question: "Do you charge extra for WhatsApp, SMS, or phone?",
    answer:
      "Not a cent from us. Connect your own Twilio number or WhatsApp Business account and pay your carrier yourself. Calls arrive with a recording and transcript.",
  },
  {
    question: "Can I run a call center on dimmi?",
    answer:
      "If you run a 200-seat phone floor, pick a tool built for call centers. Calls land in dimmi, but we do not ship power dialers, wallboards, or shift schedules.",
  },
  {
    question: "Do engineers and founders need paid seats to read a thread?",
    answer:
      "Never. Seats are free and unlimited on dimmi Cloud, so anyone on your team can read a bug report or leave an internal note without raising your bill.",
  },
  {
    question: "Is the self-hosted inbox missing features?",
    answer:
      "It is the exact same inbox. You get every channel, rule, and shortcut on your own server, with all your data in your own Postgres database.",
  },
  {
    question: "How do I bring my existing customers over?",
    answer:
      "Upload a CSV of customers and companies, map your columns, run a dry run to check the results, and import them in minutes.",
  },
];

export function InboxPage() {
  return (
    <>
      <ProductHero
        eyebrow="Inbox"
        title="One inbox your whole company can afford to open."
        body="Chat, email, Slack, WhatsApp, SMS, and phone land in one queue. Seats are free, so any engineer can jump into a thread."
        shot="inbox"
        shotAlt="The dimmi inbox: conversation list, thread and details"
      />
      <ChannelStrip
        label="Channels"
        items={[
          { icon: RiChat3Line, name: "Messenger" },
          { icon: RiMailLine, name: "Email" },
          { icon: RiSlackLine, name: "Slack" },
          { icon: RiMicrosoftLine, name: "Teams" },
          { icon: RiDiscordLine, name: "Discord" },
          { icon: RiWhatsappLine, name: "WhatsApp" },
          { icon: RiMessage2Line, name: "SMS" },
          { icon: RiPhoneLine, name: "Phone" },
          { icon: RiCodeSSlashLine, name: "API" },
        ]}
      />
      <FeatureCards
        title="Answer without jumping across five tabs."
        body="Triage, customer info, and teamwork live in one view. Nobody opens another tool to find an order."
        features={features}
      />
      <FeatureGrid title="Everything your team uses all day." items={more} />
      <FeatureGrid title="Works with the rest of dimmi." items={related} />
      <Questions body="What teams ask before moving their inbox over." items={questions} />
      <FinalCta />
    </>
  );
}
