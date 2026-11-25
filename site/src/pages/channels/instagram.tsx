import { At, Buildings, ChatsCircle, CurrencyDollar, GitMerge, Timer } from "@phosphor-icons/react";

import { ChannelLinks, FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Connect",
    title: "Connect one or more Instagram accounts through Meta.",
    body: "Sign in with Meta and pick your Instagram Business or Creator accounts. Send each brand handle to its own inbox and team.",
    visual: (
      <ShotStill
        name="channel-instagram-connect"
        alt="Instagram integration settings showing Meta OAuth connection and multiple brand handles routed to separate inboxes"
      />
    ),
  },
  {
    label: "Shared inbox",
    title: "See past emails and orders beside every DM.",
    body: "Instagram DMs land next to past emails, live chats, and order history. Teammates can tag coworkers and leave private notes before replying.",
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
    label: "Photos and replies",
    title: "View photos, story replies, and reply timers in one place.",
    body: "See customer photos, videos, and story replies right in the thread, and send images, audio, or PDFs back. The reply box shows how much time Meta allows to answer.",
    visual: (
      <ShotStill
        name="channel-instagram-media"
        alt="Instagram DM thread displaying customer product photos, a story reply link, PDF attachment, and the Meta 24-hour reply window timer in the composer"
      />
    ),
  },
  {
    label: "AI agents",
    title: "Let AI agents check orders and handle simple returns.",
    body: "Assign an AI support agent to your Instagram inbox to look up orders and walk customers through returns, while tricky cases go straight to your team.",
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
    title: "Story replies and reels",
    body: "Story mentions, story replies, and shared posts open with a link in the thread, and edited messages update in place.",
  },
  {
    icon: Buildings,
    title: "Multiple brand accounts",
    body: "Connect as many Instagram accounts as you run and route each handle to its own team and saved replies.",
  },
  {
    icon: GitMerge,
    title: "Merge DMs with email",
    body: "Link a customer's Instagram handle to their email profile so your team sees past orders beside every DM.",
  },
  {
    icon: Timer,
    title: "Clear reply timers",
    body: "See Meta's 24-hour AI window and 7-day human reply window right in the reply box.",
  },
  {
    icon: At,
    title: "Private team notes",
    body: "Tag warehouse staff or founders inside a DM thread to check an order before answering the customer.",
  },
  {
    icon: CurrencyDollar,
    title: "No social add-on fee",
    body: "Instagram DMs are included in your 100 free monthly conversations on dimmi Cloud and cost $0 self-hosted.",
  },
];

const questions = [
  {
    question: "Does dimmi handle public Instagram comments or personal accounts?",
    answer:
      "Only direct messages for Business and Creator accounts. Keep a social media tool if you need post scheduling or public comment moderation.",
  },
  {
    question: "How long do we have to reply to an Instagram DM?",
    answer:
      "Both AI agents and teammates can reply for 24 hours after a customer's last message. Teammates can reply for up to 7 days. After 7 days, Meta locks the thread until the customer writes again.",
  },
  {
    question: "Why did a message from a non-follower stay in Instagram Requests?",
    answer:
      "Meta holds first-time DMs from accounts you do not follow inside Instagram's Requests folder until you accept them in the app. After that, messages flow into dimmi.",
  },
  {
    question: "What setting do we need to turn on in Instagram?",
    answer:
      "Connect your account in dimmi, then turn on Allow access to messages under Connected tools in the Instagram app so Meta sends DMs to dimmi.",
  },
  {
    question: "Can we connect multiple Instagram accounts for different brands?",
    answer:
      "Yes. Link as many Business or Creator accounts as you run and route each one to its own inbox and team for $0 extra.",
  },
  {
    question: "Do you charge extra for Instagram DMs?",
    answer:
      "No. Instagram DMs use standard conversation pricing on dimmi Cloud and are completely free in the self-hosted MIT build.",
  },
];

export function InstagramChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Instagram"
        breadcrumb={[
          { label: "Home", href: "/" },
          { label: "Channels", href: "/channels" },
        ]}
        title="Answer Instagram DMs from one shared inbox."
        body="Connect your Instagram Business accounts to dimmi. Reply to DMs and story replies beside email and order history without passing one phone around."
        visual={
          <ShotVideo
            name="channel-instagram-hero"
            poster="channel-instagram-hero"
            alt="An Instagram DM conversation on the left synced live with the dimmi inbox on the right, showing order history and a support agent reply"
            bleed
          />
        }
      />
      <Rows title="Turn Instagram DMs into a clean support queue." rows={rows} />
      <FeatureGrid title="Built for how customers message brands on Instagram." items={more} />
      <ChannelLinks current="/channels/instagram" />
      <Questions body="What teams ask before moving Instagram DMs into dimmi." items={questions} />
      <FinalCta />
    </>
  );
}
