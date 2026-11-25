import { Buildings, Checks, CurrencyDollar, Files, Megaphone, Sparkle } from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Direct Meta signup",
    title: "Connect your numbers in minutes and pay Meta yourself.",
    body: "Link a WhatsApp Business number through Meta Embedded Signup and keep the phone app running if you want. Meta bills you for template messages, and dimmi adds $0 markup.",
    visual: (
      <ShotStill
        name="channel-whatsapp-signup"
        alt="WhatsApp channel setup using Meta Embedded Signup to connect a WhatsApp Business number to a dimmi inbox"
      />
    ),
  },
  {
    label: "Approved templates",
    title: "Write, submit, and send Meta templates without leaving dimmi.",
    body: "Create templates with text headers, variables, quick replies, and links, with Meta approval status right beside each one. Type / in the composer to fill details and preview before sending.",
    visual: (
      <ShotVideo
        name="channel-whatsapp-templates"
        poster="channel-whatsapp-templates"
        alt="Creating and sending a Meta-approved WhatsApp template with text header, customer variables, quick replies, and a link button inside dimmi"
        maxWidth={520}
      />
    ),
  },
  {
    label: "24-hour window",
    title: "See the 24-hour clock right where you type.",
    body: "WhatsApp closes free-form replies 24 hours after a customer writes. Our composer shows the hours left and switches to approved templates once time runs out, so messages never fail without warning.",
    visual: (
      <ShotStill
        name="channel-whatsapp-window"
        alt="Inbox composer showing the remaining 24-hour WhatsApp reply window countdown and switching to approved templates when the window closes"
      />
    ),
  },
  {
    label: "Media, agents & broadcasts",
    title: "One channel for photos, multiple numbers, agents, and broadcasts.",
    body: "Share photos, videos, voice notes, and documents in the thread. Route multiple numbers to separate inboxes, turn on support agents, and send WhatsApp broadcasts whose replies land in your queue.",
    visual: (
      <ShotStill
        name="channel-whatsapp-media"
        alt="WhatsApp thread in dimmi with photo and voice note attachments, multi-number inbox routing, support agent reply, and broadcast history"
      />
    ),
  },
];

const more = [
  {
    icon: Buildings,
    title: "Multiple numbers, separate inboxes",
    body: "Connect numbers for sales, support, or regional brands and send each line to its own team and SLA.",
  },
  {
    icon: Files,
    title: "Two-way template sync",
    body: "Templates you build in WhatsApp Manager pull into dimmi on connect, and you can refresh them in one click.",
  },
  {
    icon: Sparkle,
    title: "Support agents on WhatsApp",
    body: "Agents answer from your help center, follow refund or order procedures, and hand over with a private summary.",
  },
  {
    icon: Megaphone,
    title: "Two-way WhatsApp broadcasts",
    body: "Send approved template updates to filtered customer lists and handle every reply inside your shared inbox.",
  },
  {
    icon: Checks,
    title: "Read receipts and rich cards",
    body: "Replying marks the customer's latest message as read, and shared locations, contact cards, and quick replies render right in the thread.",
  },
  {
    icon: CurrencyDollar,
    title: "Direct Meta billing",
    body: "Connect your own WhatsApp Business account and pay Meta directly, with zero per-message markup from dimmi.",
  },
];

const questions = [
  {
    question: "Does dimmi charge extra per WhatsApp message or per number?",
    answer:
      "Not a cent. You connect your own Meta account and pay Meta for template messages. On dimmi Cloud, WhatsApp threads count under your standard conversation pricing after your first 100 free conversations each month.",
  },
  {
    question: "Can we keep using the WhatsApp Business app on our phone?",
    answer:
      "You can. Connect a number that already runs on the WhatsApp Business app through Meta Embedded Signup and keep that app active while your team answers from dimmi.",
  },
  {
    question: "What happens when a customer writes and we reply after 24 hours?",
    answer:
      "Meta blocks free-form replies once 24 hours pass. dimmi shows the countdown in the composer and switches to your approved templates when it hits zero. As soon as the customer replies, a fresh 24-hour window opens.",
  },
  {
    question: "Which WhatsApp template types work in dimmi today?",
    answer:
      "dimmi creates and sends templates with a text header, formatted body, footer, variables, and buttons for quick replies or links. Templates with media headers, carousels, or dynamic buttons sync into your list, though you cannot edit or send them from dimmi yet.",
  },
  {
    question: "Is dimmi the right tool for WhatsApp ad funnels or voice calls?",
    answer:
      "Pick a marketing tool or call center for those. dimmi handles two-way support threads and template updates. We skip click-to-WhatsApp ad campaigns, emoji reactions, and WhatsApp voice or video calls.",
  },
  {
    question: "Does WhatsApp work on the self-hosted MIT build too?",
    answer:
      "The exact same integration runs on both dimmi Cloud and self-hosted servers. Connect your Meta app, point webhooks at your server, and run unlimited seats and numbers while paying $0 to dimmi.",
  },
];

export function WhatsAppChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="WhatsApp"
        title="Turn WhatsApp Business into a shared inbox for your whole team."
        body="Connect your official WhatsApp Business numbers through Meta. Manage approved templates, track 24-hour windows, and answer beside customer history with zero message markup."
        visual={
          <ShotVideo
            name="channel-whatsapp-hero"
            poster="channel-whatsapp-hero"
            alt="dimmi shared inbox showing a live WhatsApp customer conversation, 24-hour reply timer, customer details, and an agent reply"
            bleed
          />
        }
      />
      <Rows title="Built around Meta's real rules, without the middleman fees." rows={rows} />
      <FeatureGrid title="Everything your team needs on WhatsApp, with no add-on tier." items={more} />
      <Questions body="What teams ask before connecting their WhatsApp Business numbers." items={questions} />
      <FinalCta />
    </>
  );
}
