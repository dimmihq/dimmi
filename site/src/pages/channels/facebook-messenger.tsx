import { Alarm, Buildings, ChatText, Files, Hourglass, ShieldCheck } from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Meta OAuth setup",
    title: "Official Meta OAuth setup with live connection health checks.",
    body: "Sign in with Meta, pick your business portfolio, and attach the Pages you want in dimmi. Health checks watch each Page so you spot expired permissions before messages drop.",
    visual: (
      <ShotStill
        name="channel-facebook-connect"
        alt="Facebook Messenger integration settings in dimmi showing official Meta OAuth setup, attached Facebook Pages, and live connection health status"
      />
    ),
  },
  {
    label: "Shared inbox",
    title: "Skip Meta Business Suite and answer beside email and WhatsApp.",
    body: "Incoming Page messages open in your shared inbox next to email, WhatsApp, and past tickets. Teammates see the customer's history, leave internal notes, and reply straight back to Messenger.",
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
    label: "Routing, files, and windows",
    title: "One inbox per brand, any file type, and a live 24-hour clock.",
    body: "Attach multiple Facebook Pages and route each brand to its own inbox. Send photos, video, audio, or files while the composer tracks Meta's 24-hour and 7-day reply windows.",
    visual: (
      <ShotStill
        name="channel-facebook-routing"
        alt="Multi-brand Facebook Page inbox routing, image and file attachments, and the Meta 24-hour reply window countdown inside the dimmi composer"
      />
    ),
  },
  {
    label: "Agents and SLAs",
    title: "Support agents answer Page messages around the clock on your rules.",
    body: "Assign specialized support agents to answer Page messages from your docs and follow procedures in plain English. Pair them with workflows and SLAs so edge cases reach teammates fast.",
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
    title: "Multiple Pages in one workspace",
    body: "Run separate Facebook Pages for each brand, store, or region and route each Page into its own inbox and team.",
  },
  {
    icon: Hourglass,
    title: "Live Meta reply clock",
    body: "See how much time remains in Meta's 24-hour agent window and 7-day teammate window right inside the composer.",
  },
  {
    icon: Files,
    title: "Media and file attachments",
    body: "Receive customer photos, videos, shared links, and edited messages, and send back images, audio, video, or files like invoices.",
  },
  {
    icon: ChatText,
    title: "Automatic reply splitting",
    body: "Write a full answer once without chopping text by hand. dimmi splits long replies into multiple Messenger messages in order.",
  },
  {
    icon: Alarm,
    title: "Workflows and SLA targets",
    body: "Tag incoming Page threads, open linked engineering tickets, and set reply targets that respect your workspace office hours.",
  },
  {
    icon: ShieldCheck,
    title: "Connection health checks",
    body: "Automatic token and permission checks flag broken Facebook Page connections before customer messages go missing.",
  },
];

const questions = [
  {
    question: "Can we start a new Facebook Messenger conversation from dimmi?",
    answer:
      "Meta blocks businesses from starting cold Messenger threads. A customer has to message your Facebook Page first, and once they do, your team and support agents can reply right from dimmi.",
  },
  {
    question: "What happens after Meta's 24-hour reply window closes?",
    answer:
      "During the first 24 hours after a customer's message, both teammates and support agents can reply. Between 24 hours and 7 days, only human teammates can reply. Past 7 days, Meta locks the thread until the customer writes again.",
  },
  {
    question: "Can I connect a personal Facebook profile or multiple Pages?",
    answer:
      "Only Facebook Pages in your Meta business portfolio can connect, since Meta does not expose personal profiles to helpdesk apps. Attach as many Pages as you run and route each to its own inbox.",
  },
  {
    question: "What kinds of customer messages and files work in Messenger?",
    answer:
      "Text, photos, videos, shared posts, links, and edited messages all arrive in the thread. Your team can reply with text, images, video, audio, or any file attachment, and long replies split across messages for you.",
  },
  {
    question: "Will our support agents keep replying if a teammate jumps in?",
    answer:
      "Support agents step aside as soon as a teammate replies or takes the thread, or whenever a handover rule triggers. Inside the 24-hour window, they answer from your docs and follow procedures you write in plain English.",
  },
  {
    question: "Do you charge extra for Facebook Messenger?",
    answer:
      "dimmi includes Facebook Messenger on both dimmi Cloud and the MIT self-hosted build with zero extra channel fees.",
  },
];

export function FacebookMessengerChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Facebook Messenger"
        title="Answer Facebook Page messages without living inside Meta Business Suite."
        body="Connect your Facebook Pages to dimmi through Meta OAuth. Answer Page messages beside email and past tickets, and let support agents reply around the clock."
        visual={
          <ShotVideo
            name="channel-facebook-hero"
            poster="channel-facebook-hero"
            alt="A Facebook Messenger conversation inside the dimmi inbox alongside email and WhatsApp threads, customer history, and a live reply window timer"
            bleed
          />
        }
      />
      <Rows title="Handle Facebook Page messages inside your real support queue." rows={rows} />
      <FeatureGrid title="The details that keep Facebook Messenger support moving." items={more} />
      <Questions body="What teams ask before connecting their Facebook Pages." items={questions} />
      <FinalCta />
    </>
  );
}
