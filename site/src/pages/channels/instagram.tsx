import { At, Buildings, ChatsCircle, CurrencyDollar, GitMerge, Timer } from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Meta OAuth",
    title: "Connect Instagram Business accounts through Meta and route each brand.",
    body: "Sign in with official Meta OAuth and pick which Business or Creator accounts to share. Send each brand handle to its own inbox and team.",
    visual: (
      <ShotStill
        name="channel-instagram-connect"
        alt="Instagram integration settings showing Meta OAuth connection and multiple brand handles routed to separate inboxes"
      />
    ),
  },
  {
    label: "Shared inbox",
    title: "Reply beside email, chat, and order history without touching a phone.",
    body: "Incoming DMs land beside past emails, chat threads, and order history. Teammates jump in, tag ops, and leave private internal notes from one queue.",
    visual: (
      <ShotVideo
        name="channel-instagram-thread"
        poster="channel-instagram-thread"
        alt="Teammate answering an Instagram DM in dimmi with past conversations, customer order history, and an internal note beside the composer"
        maxWidth={520}
      />
    ),
  },
  {
    label: "Media & reply windows",
    title: "See customer photos and story replies, plus your Meta reply countdown.",
    body: "View grouped photos, videos, and story replies in the thread, and send images, audio, or PDFs back. The composer shows Meta's 24-hour and 7-day reply countdown.",
    visual: (
      <ShotStill
        name="channel-instagram-media"
        alt="Instagram DM thread displaying customer product photos, a story reply link, PDF attachment, and the Meta 24-hour reply window timer in the composer"
      />
    ),
  },
  {
    label: "Agents & automation",
    title: "Let support agents look up orders and handle returns automatically.",
    body: "Assign a dedicated support agent to your Instagram inbox. It looks up orders and runs return procedures, while workflows and SLAs route escalations to the right teammate.",
    visual: (
      <ShotStill
        name="channel-instagram-agents"
        alt="Dedicated Orders and returns support agent assigned to an Instagram inbox with return procedures, routing workflows, and SLA policies"
      />
    ),
  },
];

const more = [
  {
    icon: ChatsCircle,
    title: "Story replies and shared reels",
    body: "Story mentions, replies, and shared posts open with a link in the thread, and edited messages update in place.",
  },
  {
    icon: Buildings,
    title: "One workspace for multiple brands",
    body: "Connect multiple Instagram accounts and route each brand handle to its own inbox, support agent, and saved replies.",
  },
  {
    icon: GitMerge,
    title: "Merge DMs with email threads",
    body: "Merge a customer's Instagram DM with their email profile so teammates see past orders and tickets beside each reply.",
  },
  {
    icon: Timer,
    title: "Clear 24-hour and 7-day timers",
    body: "Watch Meta's 24-hour agent window and 7-day human window in the composer, while long replies split without losing text.",
  },
  {
    icon: At,
    title: "Private internal notes",
    body: "Tag warehouse staff, founders, or social leads right inside a DM thread to check an order before replying.",
  },
  {
    icon: CurrencyDollar,
    title: "Zero social add-on fees",
    body: "Instagram DMs share your 100 free monthly conversations on dimmi Cloud and ship in the MIT self-hosted build for $0.",
  },
];

const questions = [
  {
    question: "Does dimmi handle public Instagram comments or personal accounts?",
    answer:
      "Instagram integration in dimmi covers direct messages for Business and Creator accounts only. If you need social media publishing or public comment moderation on posts and reels, keep a dedicated social tool alongside dimmi.",
  },
  {
    question: "How do Meta's reply windows work inside dimmi?",
    answer:
      "Support agents and teammates can reply for 24 hours after a customer's last DM, and teammates can reply for up to 7 days. After 7 days, Meta locks the thread until the customer writes again. You cannot start a cold DM thread from dimmi.",
  },
  {
    question: "Why did a first-time DM from a non-follower stay in Instagram Requests?",
    answer:
      "Meta holds first-time messages from accounts you do not follow inside Instagram's Requests folder until you accept them in the Instagram app. Once you accept it, future messages from that customer flow into dimmi.",
  },
  {
    question: "What do we need to turn on in the Instagram app?",
    answer:
      "Connect your Business or Creator account through Meta OAuth in dimmi, then enable Allow access to messages under Connected tools in the Instagram app. Without that toggle on, Meta sends zero messages to dimmi.",
  },
  {
    question: "Can we connect multiple Instagram accounts for different brands?",
    answer:
      "Link as many Business or Creator accounts as you run and route each handle to its own inbox, team, and support agent. Your workspace includes multi-brand routing with zero extra channel fees.",
  },
  {
    question: "Do you charge extra for Instagram DMs?",
    answer:
      "Not a cent extra. Instagram DMs use standard dimmi conversation pricing on dimmi Cloud, and ship in the MIT self-hosted build for $0.",
  },
];

export function InstagramChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Instagram"
        title="Answer Instagram DMs without passing one phone around the office."
        body="Connect your Instagram Business accounts beside email and chat. Your team and AI support agents answer order questions, story replies, and returns from one shared inbox."
        visual={
          <ShotVideo
            name="channel-instagram-hero"
            poster="channel-instagram-hero"
            alt="Instagram DM conversation in the dimmi inbox beside email and chat, showing customer order history and a support agent draft"
            bleed
          />
        }
      />
      <Rows title="Run Instagram support like a real queue, with full order context." rows={rows} />
      <FeatureGrid title="Built for how customers message brands on Instagram." items={more} />
      <Questions body="What ecommerce and brand teams ask before moving Instagram DMs into dimmi." items={questions} />
      <FinalCta />
    </>
  );
}
