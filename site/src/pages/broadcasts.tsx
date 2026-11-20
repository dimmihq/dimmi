import {
  ChartLineUp,
  Clock,
  Flag,
  GitFork,
  HandPalm,
  ListChecks,
  Megaphone,
  ShieldCheck,
  Sparkle,
} from "@phosphor-icons/react";

import { FeatureGrid, HowItWorks, ProductHero, Questions, Rows, Steps } from "../components/product-page";
import {
  AudienceFragment,
  BroadcastReportFragment,
  ChannelFragment,
  DeliveryFragment,
  FunnelFragment,
  ReplyFragment,
  SeriesStack,
  WriteFragment,
} from "../fragments/broadcast-fragments";
import { FinalCta } from "../sections/final-cta";

const steps = [
  {
    title: "Pick who it's for",
    body: "A saved audience, a rule written for this send, or a list you upload.",
    visual: <AudienceFragment />,
  },
  {
    title: "Write it once",
    body: "Name each person with variables, then preview it the way they'll read it.",
    visual: <WriteFragment />,
  },
  {
    title: "Send from your own inbox",
    body: "From a teammate, on an address you connected. Replies come back to your team.",
    visual: <DeliveryFragment />,
  },
];

const rows = [
  {
    label: "Channels",
    title: "Six ways to reach people.",
    body: "Email, SMS, WhatsApp, a chat message or an announcement card in the messenger, or a post in each customer's shared Slack channel.",
    visual: <ChannelFragment />,
  },
  {
    label: "Replies",
    title: "Every reply is a conversation.",
    body: "Replies open in the inbox, assigned to the team you picked. The agent can answer them first, or you can leave them all to your team.",
    visual: <ReplyFragment />,
  },
  {
    label: "Reports",
    title: "See who got there, not just who opened.",
    body: "Follow each broadcast from sent to the goal you set, with the people, links, and accounts behind every number.",
    visual: <FunnelFragment />,
  },
];

const series = [
  {
    title: "Pick who enters",
    body: "People who match a rule, join an audience, or do an event. A teammate or the API can add them too.",
  },
  {
    title: "Send, wait, and check",
    body: "Email, chat, posts, and Slack, with waits between them, or a wait for something they do.",
  },
  {
    title: "Follow up, or step aside",
    body: "Nudge the people who need it. Anyone who reaches the goal, replies, or unsubscribes leaves at once.",
  },
];

const more = [
  {
    icon: ListChecks,
    title: "Subscription topics",
    body: "People choose what they hear about, and unsubscribe in one click.",
  },
  { icon: HandPalm, title: "A frequency cap", body: "Nobody gets one message right after another, across every send." },
  { icon: Clock, title: "Their own time zone", body: "Schedule a send for 9:00 wherever each person is." },
  {
    icon: ShieldCheck,
    title: "Pauses on its own",
    body: "Too many bounces or spam reports and sending stops, to protect your domain.",
  },
  { icon: Flag, title: "Goals", body: "Name the event that counts as working, and the window to reach it." },
  {
    icon: Megaphone,
    title: "Important notices",
    body: "Account and service news still reaches people who unsubscribed.",
  },
  { icon: GitFork, title: "Split tests", body: "Send part of a series down another path and compare." },
  { icon: Sparkle, title: "AI paths", body: "Describe the paths in plain words. The AI picks one for each person." },
  {
    icon: ChartLineUp,
    title: "Results on every step",
    body: "See how many reached, left, or finished each step of a series.",
  },
];

const questions = [
  {
    question: "Do our emails go out from some shared domain?",
    answer:
      "No. They go from an address you connected, on a domain you verified. dimmi never sends your broadcasts from a shared domain, so your reputation stays your own.",
  },
  {
    question: "Does it send SMS or WhatsApp?",
    answer:
      "Yes, both. Connect your own Twilio number or WhatsApp Business account. Your carrier bills you for each message, and replies land in the inbox like any other.",
  },
  {
    question: "Can this replace our marketing automation tool?",
    answer:
      "No. There are no landing pages, no lead scoring, and no ad audiences. Broadcasts are for telling customers things and hearing back. Keep your marketing tool for the funnel.",
  },
  {
    question: "Broadcast or series: which one do I want?",
    answer:
      "A broadcast, if it's one message. It goes out once, now or at a set time. A series runs over days, with waits, checks, and paths.",
  },
  {
    question: "Can people drop one kind of email and keep the rest?",
    answer:
      "Yes. Each broadcast can belong to a topic, and people pick their topics on a preferences page linked from every email.",
  },
  {
    question: "Can our own product start a series?",
    answer:
      "Yes. Send events to dimmi and a series can start when someone does one. You can also add people through the API.",
  },
];

export function BroadcastsPage() {
  return (
    <>
      <ProductHero
        eyebrow="Broadcasts"
        title="Messages people can reply to."
        body="Send announcements and multi-step series by email, SMS, WhatsApp, messenger, and Slack, from your own address. Replies land in your inbox, not a no-reply void."
        visual={<BroadcastReportFragment />}
      />
      <Steps title="Pick, write, send." items={steps} />
      <Rows title="The send is the easy part." rows={rows} />
      <HowItWorks
        label="Series"
        title="Messages that follow what people do."
        body="A series sends the next message when it fits, and knows when to stop."
        steps={series}
        visual={(step) => <SeriesStack step={step} />}
      />
      <FeatureGrid title="How not to become spam." items={more} />
      <Questions body="What people ask before their first send." items={questions} />
      <FinalCta />
    </>
  );
}
