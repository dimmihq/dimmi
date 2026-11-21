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
    title: "Pick who gets it",
    body: "Choose a saved audience, filter customers for this send, or upload a CSV list.",
    visual: <AudienceFragment />,
  },
  {
    title: "Write and preview",
    body: "Personalize with customer fields and preview the exact message before sending.",
    visual: <WriteFragment />,
  },
  {
    title: "Send from your address",
    body: "Send from a real teammate on your own domain so replies land right in your inbox.",
    visual: <DeliveryFragment />,
  },
];

const rows = [
  {
    label: "Channels",
    title: "Reach customers where they read.",
    body: "Send by email, SMS, WhatsApp, chat message, messenger card, or straight into each customer's shared Slack channel.",
    visual: <ChannelFragment />,
  },
  {
    label: "Replies",
    title: "Every reply opens a real conversation.",
    body: "Replies arrive in your shared inbox assigned to the team you picked. Let the AI agent take the first pass, or route straight to humans.",
    visual: <ReplyFragment />,
  },
  {
    label: "Reports",
    title: "Track real goals, not vanity opens.",
    body: "Follow each broadcast from delivery to the goal event you set, and click any number to inspect the people behind it.",
    visual: <FunnelFragment />,
  },
];

const series = [
  {
    title: "Choose who enters",
    body: "Start a series when someone joins an audience, fires an app event, or gets added via the API.",
  },
  {
    title: "Send, wait, and branch",
    body: "Combine email, chat, posts, and Slack with timed waits or checks on what customers did.",
  },
  {
    title: "Stop at the right moment",
    body: "Nudge people who still need help, and exit anyone who hits the goal, replies, or unsubscribes.",
  },
];

const more = [
  {
    icon: ListChecks,
    title: "Subscription topics",
    body: "Let customers pick which updates they want and opt out in one click.",
  },
  {
    icon: HandPalm,
    title: "Frequency caps",
    body: "Stop customers from getting flooded when multiple sends overlap.",
  },
  {
    icon: Clock,
    title: "Local time zones",
    body: "Schedule sends for 9:00 AM wherever each customer lives.",
  },
  {
    icon: ShieldCheck,
    title: "Auto pause on bounces",
    body: "dimmi pauses sending if bounce or spam rates spike to protect your domain.",
  },
  {
    icon: Flag,
    title: "Goal tracking",
    body: "Pick the product event that counts as success and the window to hit it.",
  },
  {
    icon: Megaphone,
    title: "Service notices",
    body: "Deliver urgent billing or outage alerts even to people who opted out of news.",
  },
  {
    icon: GitFork,
    title: "Split tests",
    body: "Test two messages in a series and see which branch hits your goal faster.",
  },
  {
    icon: Sparkle,
    title: "AI paths",
    body: "Describe each branch in plain English and let AI route every person.",
  },
  {
    icon: ChartLineUp,
    title: "Step metrics",
    body: "See how many people entered, exited, or finished every step of a series.",
  },
];

const questions = [
  {
    question: "Do our emails send from a shared dimmi domain?",
    answer:
      "Never. Broadcasts go out from your own verified domain and connected address, so your sender reputation stays yours.",
  },
  {
    question: "Can we send SMS or WhatsApp broadcasts too?",
    answer:
      "Yes. Connect your own Twilio number or WhatsApp Business account. You pay your carrier yourself, and replies open in your inbox.",
  },
  {
    question: "Can this replace HubSpot or Customer.io for marketing?",
    answer:
      "Keep your marketing tool for ad funnels and campaigns. dimmi has no landing pages, lead scoring, or ad audiences. Broadcasts are built for talking to customers and getting replies.",
  },
  {
    question: "When should I use a broadcast vs. a series?",
    answer:
      "Use a broadcast for a single send like an outage notice or release update. Use a series for onboarding or check-ins that run over days with waits and branches.",
  },
  {
    question: "Can people unsubscribe from one topic without dropping all emails?",
    answer:
      "Yes. Assign each broadcast to a topic, and every email links to a preferences page where customers toggle what they want.",
  },
  {
    question: "Can our app trigger a series when a user signs up or upgrades?",
    answer:
      "Yes. Track product events from your app or add customers through the REST API to start a series automatically.",
  },
];

export function BroadcastsPage() {
  return (
    <>
      <ProductHero
        eyebrow="Broadcasts"
        title="Send updates your customers can reply to."
        body="Reach customers by email, SMS, WhatsApp, chat, and Slack from your own address. Every reply opens right in your inbox."
        visual={<BroadcastReportFragment />}
      />
      <Steps title="Pick your audience, write once, hit send." items={steps} />
      <Rows title="Sending is half the job. Replies are the rest." rows={rows} />
      <HowItWorks
        label="Series"
        title="Follow-up messages that react to what people do."
        body="Send the next message at the right moment, and stop automatically when a customer replies."
        steps={series}
        visual={(step) => <SeriesStack step={step} />}
      />
      <FeatureGrid title="Built to protect your domain reputation." items={more} />
      <Questions body="What teams ask before sending their first broadcast." items={questions} />
      <FinalCta />
    </>
  );
}
