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
    title: "Assign it in one click",
    body: "Give it to a person or a team. Or let round robin deal out the new ones.",
    flow: "assign",
    alt: "Assigning a conversation to a teammate",
    anchor: "top-right",
  },
  {
    title: "Snooze it. It comes back",
    body: "It returns at the time you pick, or as soon as the customer replies.",
    flow: "snooze",
    alt: "Snoozing a conversation until tomorrow",
    anchor: "top-right",
  },
  {
    title: "A queue you can read at a glance",
    body: "Every row shows the channel, the owner, and the wait.",
    flow: "queue",
    alt: "Switching between open, snoozed and closed conversations",
    anchor: "top",
  },
  {
    title: "The customer, beside every reply",
    body: "Status, priority, and tags up top. Their history underneath.",
    flow: "details",
    alt: "Setting a conversation's priority from the details panel",
    anchor: "top",
  },
  {
    title: "Operator does the reading",
    body: "It briefs you, drafts the reply, and answers from your data.",
    flow: "operator",
    alt: "Asking Operator about the customer",
    anchor: "bottom",
  },
];

const more = [
  {
    icon: Keyboard,
    title: "Keyboard first",
    body: "Every action has a shortcut. The command menu finds the ones you forgot.",
  },
  { icon: ArrowCounterClockwise, title: "Undo send", body: "A few seconds to pull a reply back. We've all needed it." },
  { icon: Files, title: "Saved replies", body: "Send the answer, then close, tag, or assign in the same step." },
  { icon: Checks, title: "Bulk actions", body: "Assign, tag, or close a pile of conversations at once." },
  { icon: GitMerge, title: "Merge", body: "One customer, two threads? Make them one conversation." },
  { icon: Smiley, title: "Ratings", body: "When you close, the customer rates the help they got." },
  { icon: Timer, title: "SLAs", body: "Reply targets by plan or priority. You get a warning before one slips." },
  { icon: Ticket, title: "Tickets", body: "Turn a conversation into tracked work your engineers can follow." },
  { icon: ChartPieSlice, title: "Reports", body: "Volume, response times, and ratings by team, channel, and tag." },
  { icon: At, title: "Notes and mentions", body: "Ask a teammate inside the thread. The customer never sees it." },
  { icon: Translate, title: "Live translation", body: "Read and reply in your language. The customer reads theirs." },
  {
    icon: DeviceMobile,
    title: "Mobile apps",
    body: "The inbox on iOS and Android, with push for what's assigned to you.",
  },
];

const related = [
  {
    icon: Sparkle,
    title: "AI agent",
    body: "It answers first. Your team gets the rest, with a summary.",
    link: { href: "/agent", label: "Meet the AI agent" },
  },
  {
    icon: FlowArrow,
    title: "Workflows",
    body: "Sorting, routing, and follow-ups that nobody has to remember.",
    link: { href: "/workflows", label: "See workflows" },
  },
  {
    icon: BookOpenText,
    title: "Help center",
    body: "Answers customers find on their own, on your domain.",
    link: { href: "/help-center", label: "See the help center" },
  },
];

const questions = [
  {
    question: "Which channels actually land in the inbox?",
    answer:
      "All of them. The messenger on your site or app, email, Slack, Microsoft Teams, Discord, WhatsApp, SMS, Instagram, Facebook Messenger, phone calls, and anything you send through the REST API. One queue.",
  },
  {
    question: "Do you do phone, WhatsApp, and SMS?",
    answer:
      "Yes. Connect your own Twilio number or WhatsApp Business account. Calls arrive with a recording and a transcript. Your carrier bills you for minutes and messages. We don't add a markup.",
  },
  {
    question: "Can I run a call center on it?",
    answer:
      "No. Calls land in the inbox, but there's no power dialer, wallboard, or shift scheduling. If you run a 200-seat phone floor, pick a tool built for that.",
  },
  {
    question: "Will Operator bill me per answer?",
    answer:
      "No. It bills per thread. Each thread a teammate starts with Operator counts as one AI conversation, however many questions they ask in it.",
  },
  {
    question: "Is the self-hosted inbox a cut-down one?",
    answer:
      "No. The self-hosted edition has the same inbox. It runs on your server, with your data in your own database.",
  },
  {
    question: "Do I have to retype my customers?",
    answer:
      "No. Import customers and companies from a spreadsheet. Map the columns, try a dry run, then run it for real.",
  },
];

export function InboxPage() {
  return (
    <>
      <ProductHero
        eyebrow="Inbox"
        title="One inbox for every channel."
        body="Chat, email, Slack, and the rest land in one queue. The customer sits beside every reply."
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
        title="Answer without leaving the thread."
        body="Triage, context, and teamwork sit in one view. Nobody opens a sixth tab to answer a question."
        features={features}
      />
      <FeatureGrid title="Small things you use all day." items={more} />
      <FeatureGrid title="The rest of dimmi plugs in." items={related} />
      <Questions items={questions} />
      <FinalCta />
    </>
  );
}
