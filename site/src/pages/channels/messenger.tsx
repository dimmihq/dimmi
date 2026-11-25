import { ChatCircleText, ChatsCircle, Code, Microphone, Translate, UsersThree } from "@phosphor-icons/react";

import { ChannelLinks, FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Home screen",
    title: "Chat, help articles, and updates in one window.",
    body: "Customers can search your help center, read product updates, or start a chat without leaving your app. New updates show a small dot so people notice them.",
    visual: (
      <ShotVideo
        name="channel-messenger-home"
        poster="channel-messenger-home"
        alt="The dimmi web messenger Home screen showing help search, recent conversations, and product updates with an unread dot"
        maxWidth={520}
      />
    ),
  },
  {
    label: "Fast setup",
    title: "Add it to your site or app in 5 minutes.",
    body: "Paste one line of HTML on any website, or install a ready-made package for React, Next.js, Vue, Angular, or Svelte. Run separate chat windows for each site or brand.",
    visual: (
      <ShotStill
        name="channel-messenger-install"
        alt="Messenger install settings showing packages for HTML, Browser, React, Next.js, Vue, Angular, and Svelte with a one-line snippet"
      />
    ),
  },
  {
    label: "Customer context",
    title: "Customers never have to repeat who they are.",
    body: "When a signed-in customer opens chat, their name, email, plan, and past conversations load right next to the thread so your team can help on the first reply.",
    visual: (
      <ShotStill
        name="channel-messenger-identity"
        alt="Verified customer details, plan, and recent account activity loaded beside a live chat conversation"
      />
    ),
  },
  {
    label: "Make it yours",
    title: "Matches your colors, light or dark mode, and brand.",
    body: "Pick your accent color, choose light or dark mode, and write your own greeting. AI replies always show an AI badge and hand over to a teammate with a summary.",
    visual: (
      <ShotStill
        name="channel-messenger-brand"
        alt="Messenger look and brand settings showing custom accent colors, light and dark mode, and AI agent handover"
      />
    ),
  },
];

const more = [
  {
    icon: Microphone,
    title: "Voice notes, photos, and files",
    body: "Customers can send screenshots, logs, or quick voice notes right in chat, with live typing and Seen receipts.",
  },
  {
    icon: Code,
    title: "Open from your own buttons",
    body: "Hide the corner chat bubble and open chat from any Help link or button inside your app.",
  },
  {
    icon: ChatsCircle,
    title: "Live unread badges",
    body: "Show an unread message count inside your own menu bar so customers never miss a reply.",
  },
  {
    icon: UsersThree,
    title: "Choose who sees chat",
    body: "Show live chat to everyone on your site, or limit new chats to signed-in or paid customers.",
  },
  {
    icon: Translate,
    title: "Speaks your customers' language",
    body: "Translate your greeting, away message, and tabs into any language your customers use.",
  },
  {
    icon: ChatCircleText,
    title: "Start chats with context",
    body: "Pre-fill a starting message when a customer clicks Help from a billing error or checkout page.",
  },
];

const questions = [
  {
    question: "How fast can we add live chat to our website or app?",
    answer:
      "About 5 minutes. Paste one script tag on any website, or install our package for React, Next.js, Vue, Angular, or Svelte.",
  },
  {
    question: "Can our AI agent answer live chats automatically?",
    answer:
      "Yes. Your AI agent answers from your help center with linked sources, follows your support rules, and hands over to a teammate with a short summary whenever a person should step in.",
  },
  {
    question: "Can customers see who is AI and who is a human?",
    answer:
      "Always. Every AI reply carries a clear AI badge in the customer's language so people always know who they are talking to.",
  },
  {
    question: "Can we run different chat windows on our site and inside our app?",
    answer:
      "Create as many messengers as you need in one workspace. Each one can have its own colors, greeting, help articles, and visibility rules.",
  },
  {
    question: "How do we make sure nobody fakes another customer's identity?",
    answer:
      "Turn on signed customer login in settings. Your server signs a short token (or connects your auth provider like Clerk or Supabase) so dimmi confirms who is writing before loading account details.",
  },
  {
    question: "Does adding live chat cost extra per site or per teammate?",
    answer:
      "No. Unlimited seats and multiple messengers are included. dimmi Cloud gives you 100 free conversations every month, and self-hosting is $0.",
  },
];

export function MessengerChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Web messenger"
        breadcrumb={[
          { label: "Home", href: "/" },
          { label: "Channels", href: "/channels" },
        ]}
        title="Live chat that answers fast, right inside your app."
        body="Help customers the moment they get stuck. AI answers simple questions with linked docs, and your team steps in with full customer history."
        visual={
          <ShotVideo
            name="channel-messenger-hero"
            poster="channel-messenger-hero"
            alt="The dimmi web messenger open inside a customer app, showing live chat, AI answers with linked docs, voice notes, and teammate replies"
            bleed
          />
        }
      />
      <Rows title="Everything customers expect from modern live chat." rows={rows} />
      <FeatureGrid title="Built to feel like part of your product." items={more} />
      <ChannelLinks current="/channels/messenger" />
      <Questions body="What teams ask before adding live chat to their site or app." items={questions} />
      <FinalCta />
    </>
  );
}
