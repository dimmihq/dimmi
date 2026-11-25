import { Alarm, Buildings, ChatText, Files, Hourglass, ShieldCheck } from "@phosphor-icons/react";

import { ChannelLinks, FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Connect",
    title: "Connect your Facebook Pages and check their health.",
    body: "Sign in with Meta and pick the Facebook Pages you want in dimmi. Health checks watch each Page and warn you if permissions expire.",
    visual: (
      <ShotStill
        name="channel-facebook-connect"
        alt="Facebook Messenger integration settings in dimmi showing official Meta OAuth setup, attached Facebook Pages, and live connection health status"
      />
    ),
  },
  {
    label: "Shared inbox",
    title: "Reply beside past emails, WhatsApp chats, and team notes.",
    body: "Skip Meta Business Suite. Incoming Page messages land next to past emails and tickets so teammates can leave private notes and reply right from dimmi.",
    visual: (
      <ShotVideo
        name="channel-facebook-thread"
        poster="channel-facebook-thread"
        alt="A Facebook Messenger thread open in the dimmi inbox showing past email and WhatsApp conversations, internal notes, and a reply posting back to Messenger"
        maxWidth={520}
      />
    ),
  },
  {
    label: "Files and timers",
    title: "Route each Page to its own team and send any file.",
    body: "Connect multiple Facebook Pages and send each brand to its own inbox. Share photos, videos, audio, or PDFs while seeing how much time Meta allows to reply.",
    visual: (
      <ShotStill
        name="channel-facebook-routing"
        alt="Multi-brand Facebook Page inbox routing, image and file attachments, and the Meta 24-hour reply window countdown inside the dimmi composer"
      />
    ),
  },
  {
    label: "AI agents",
    title: "Let AI agents answer Page messages day and night.",
    body: "Put an AI support agent on your Facebook inbox to answer from your docs and follow your rules. Workflows and reply timers make sure urgent chats reach people fast.",
    visual: (
      <ShotStill
        name="channel-facebook-agents"
        alt="A specialized dimmi support agent answering a Facebook Messenger question using a procedure, with workflow routing and an active SLA clock"
      />
    ),
  },
];

const more = [
  {
    icon: Buildings,
    title: "Multiple Facebook Pages",
    body: "Connect Pages for different brands or stores and route each one to its own inbox and team.",
  },
  {
    icon: Hourglass,
    title: "Live Meta reply timer",
    body: "See how much time is left in Meta's 24-hour AI window and 7-day teammate window right in the reply box.",
  },
  {
    icon: Files,
    title: "Photos, videos, and PDFs",
    body: "Receive customer photos, videos, and links, and send back images, audio, or files like invoices.",
  },
  {
    icon: ChatText,
    title: "Long replies split cleanly",
    body: "Write a full answer once. dimmi splits long messages into ordered Messenger bubbles automatically.",
  },
  {
    icon: Alarm,
    title: "Workflows and reply timers",
    body: "Tag Page messages, link engineering tickets, and track reply targets during your office hours.",
  },
  {
    icon: ShieldCheck,
    title: "Connection health alerts",
    body: "Automatic checks spot broken Page permissions before customer messages go missing.",
  },
];

const questions = [
  {
    question: "Can we start a new Facebook Messenger chat with a customer?",
    answer:
      "No. Meta requires the customer to message your Facebook Page first. Once they write, your team and AI agents can reply from dimmi.",
  },
  {
    question: "What happens after 24 hours?",
    answer:
      "For the first 24 hours, both people and AI agents can reply. Between 24 hours and 7 days, only human teammates can reply. After 7 days, Meta locks the thread until the customer writes again.",
  },
  {
    question: "Can we connect a personal Facebook account or multiple Pages?",
    answer:
      "Only Facebook Pages can connect, because Meta does not allow apps on personal profiles. You can connect as many Pages as you run in one workspace.",
  },
  {
    question: "What file types work in Facebook Messenger?",
    answer:
      "You can receive text, photos, videos, shared posts, and links, and send back text, images, video, audio, or PDFs.",
  },
  {
    question: "Will the AI agent keep talking after a teammate steps in?",
    answer: "No. The agent steps back the moment a teammate replies or takes over the conversation.",
  },
  {
    question: "Does Facebook Messenger cost extra?",
    answer: "No. Facebook Messenger is included on dimmi Cloud and in the free self-hosted MIT build.",
  },
];

export function FacebookMessengerChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Facebook Messenger"
        breadcrumb={[
          { label: "Home", href: "/" },
          { label: "Channels", href: "/channels" },
        ]}
        title="Answer Facebook Page messages from your team inbox."
        body="Connect your Facebook Pages to dimmi through Meta. Read and reply to Page messages beside email and WhatsApp without opening Meta Business Suite."
        visual={
          <ShotVideo
            name="channel-facebook-hero"
            poster="channel-facebook-hero"
            alt="A Facebook Messenger chat on the left synced live with the dimmi inbox on the right, showing internal notes and a live reply"
            bleed
          />
        }
      />
      <Rows title="Keep every Facebook message in your main inbox." rows={rows} />
      <FeatureGrid title="Helpful tools for Facebook Page support." items={more} />
      <ChannelLinks current="/channels/facebook-messenger" />
      <Questions body="What teams ask before connecting their Facebook Pages." items={questions} />
      <FinalCta />
    </>
  );
}
