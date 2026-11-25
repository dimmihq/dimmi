import { Buildings, Checks, CurrencyDollar, Files, Megaphone, Sparkle } from "@phosphor-icons/react";

import { ChannelLinks, FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Connect Meta",
    title: "Connect your numbers in minutes and pay Meta directly.",
    body: "Link your WhatsApp Business number through Meta and keep using the phone app if you want. Meta bills you directly for template messages, and dimmi adds $0 markup.",
    visual: (
      <ShotStill
        name="channel-whatsapp-signup"
        alt="WhatsApp channel setup using Meta Embedded Signup to connect a WhatsApp Business number to a dimmi inbox"
      />
    ),
  },
  {
    label: "Templates",
    title: "Write and send Meta-approved templates from dimmi.",
    body: "Create templates with headers, customer names, quick reply buttons, and links. Type / in the reply box to pick a template and preview it before you send.",
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
    label: "24-hour clock",
    title: "See how much time is left to reply.",
    body: "WhatsApp closes normal replies 24 hours after a customer writes. dimmi shows the countdown in your reply box and switches to approved templates when time runs out.",
    visual: (
      <ShotStill
        name="channel-whatsapp-window"
        alt="Inbox composer showing the remaining 24-hour WhatsApp reply window countdown and switching to approved templates when the window closes"
      />
    ),
  },
  {
    label: "Photos and audio",
    title: "Send photos, voice notes, and files in the same chat.",
    body: "Receive and send photos, videos, voice notes, and PDFs right in the thread. Route multiple numbers to separate teams, add AI agents, and send template updates.",
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
    title: "Multiple phone numbers",
    body: "Connect numbers for sales, support, or different regions and route each one to its own team.",
  },
  {
    icon: Files,
    title: "Templates stay in sync",
    body: "Templates you already made in Meta pull into dimmi automatically, and you can refresh them in one click.",
  },
  {
    icon: Sparkle,
    title: "AI agents on WhatsApp",
    body: "Agents answer from your help center, check orders, and hand over to your team with a short summary.",
  },
  {
    icon: Megaphone,
    title: "Two-way broadcasts",
    body: "Send approved updates to customer lists and answer every reply right inside your shared inbox.",
  },
  {
    icon: Checks,
    title: "Read receipts and rich messages",
    body: "Replying marks messages as read, and shared locations, contacts, and quick replies show cleanly.",
  },
  {
    icon: CurrencyDollar,
    title: "Zero message markup",
    body: "Connect your own Meta account and pay Meta's standard rates with $0 added by dimmi.",
  },
];

const questions = [
  {
    question: "Does dimmi charge extra per WhatsApp message or phone number?",
    answer:
      "No. You pay Meta directly for template messages with $0 markup from dimmi. On dimmi Cloud, WhatsApp chats use normal conversation pricing after 100 free conversations each month.",
  },
  {
    question: "Can we still use the WhatsApp Business app on our phone?",
    answer:
      "Yes. Connect your existing WhatsApp Business app number through Meta and keep using your phone while your team replies from dimmi.",
  },
  {
    question: "What happens if we reply after 24 hours?",
    answer:
      "Meta blocks normal messages once 24 hours pass. dimmi shows the countdown and switches the reply box to your approved templates. Once the customer writes back, normal chat reopens.",
  },
  {
    question: "Which WhatsApp templates can we send from dimmi?",
    answer:
      "You can write and send templates with a text header, formatted body, footer, variables, and quick reply or link buttons. Media-header and carousel templates sync to your list, though you cannot edit or send them here yet.",
  },
  {
    question: "Does dimmi support WhatsApp voice calls or ad campaigns?",
    answer:
      "Not today. dimmi focuses on two-way support chats and template updates. Pick a marketing or phone tool if you need click-to-WhatsApp ads or WhatsApp voice calls.",
  },
  {
    question: "Does WhatsApp work if we self-host dimmi?",
    answer:
      "Yes. The same WhatsApp connection ships in the free MIT self-hosted build with unlimited seats and numbers.",
  },
];

export function WhatsAppChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="WhatsApp"
        breadcrumb={[
          { label: "Home", href: "/" },
          { label: "Channels", href: "/channels" },
        ]}
        title="Answer WhatsApp Business messages as a team."
        body="Connect your WhatsApp Business number straight through Meta. Your team answers from one inbox, sends approved templates, and pays Meta directly with $0 markup."
        visual={
          <ShotVideo
            name="channel-whatsapp-hero"
            poster="channel-whatsapp-hero"
            alt="A customer WhatsApp chat on the left synced live with the dimmi shared inbox on the right, showing the 24-hour reply timer and agent reply"
            bleed
          />
        }
      />
      <Rows title="Manage WhatsApp support without sharing one phone." rows={rows} />
      <FeatureGrid title="Everything you need to run support on WhatsApp." items={more} />
      <ChannelLinks current="/channels/whatsapp" />
      <Questions body="What teams ask before connecting WhatsApp Business." items={questions} />
      <FinalCta />
    </>
  );
}
