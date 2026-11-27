import { ArrowsClockwise, Broadcast, Code, Key, Plugs, ShieldCheck } from "@phosphor-icons/react";

import { ChannelLinks, FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { SDK_COUNT } from "../../sdks";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Custom chat",
    title: "Build your own web or mobile chat with the Messenger API.",
    body: "Design the chat screen yourself in your web, iOS, or Android app without iframes or cookies. Sign a short token on your server so dimmi knows which customer is writing.",
    visual: (
      <ShotStill
        name="channel-api-messenger"
        alt="Messenger API settings showing the app ID, allowed sites, native app toggle, and identity verification signing secrets"
      />
    ),
  },
  {
    label: "Backend API",
    title: "Open and answer conversations straight from your backend.",
    body: "Every inbox includes an API channel. Call /api/v1 from your server to open chats, send replies, update customer details, and log product events.",
    visual: (
      <ShotVideo
        name="channel-api-rest"
        poster="channel-api-rest"
        alt="A backend service opening a conversation over /api/v1 and a teammate replying from the built-in API channel in the inbox"
        maxWidth={520}
      />
    ),
  },
  {
    label: "Webhooks",
    title: "Get real-time updates in your app when messages arrive.",
    body: "dimmi signs every webhook so you can trust it, and retries failed deliveries up to 12 times over 24 hours. Pick the events you want and replay missed calls anytime.",
    visual: (
      <ShotStill
        name="channel-api-webhooks"
        alt="Webhook endpoint view showing signing secret rotation, subscribed event patterns, and delivery retry history"
      />
    ),
  },
  {
    label: "SDKs and tools",
    title: "Ready-made SDKs for your stack, plus OAuth and AI tool access.",
    body: "Install typed packages for TypeScript, Go, PHP, Laravel, Python, Browser, React, Next.js, Vue, Angular, and Svelte. Create OAuth apps, or connect Claude and Cursor over MCP.",
    visual: (
      <ShotStill
        name="channel-api-sdks"
        alt={`Developer tools overview showing the ${SDK_COUNT} typed SDKs, OAuth 2 app settings, and remote MCP server URL`}
      />
    ),
  },
];

const more = [
  {
    icon: ShieldCheck,
    title: "Signed customer login",
    body: "Sign customer tokens on your server or point dimmi at your auth provider so nobody can fake an identity.",
  },
  {
    icon: Broadcast,
    title: "Live streams or polling",
    body: "Stream new messages to your custom chat app in real time, or poll for updates with a cursor.",
  },
  {
    icon: Key,
    title: "Scoped API tokens",
    body: "Give each API token only the read or write access it needs, matching your team permissions.",
  },
  {
    icon: ArrowsClockwise,
    title: "Zero-downtime key rotation",
    body: "Keep your old and new webhook or login keys active together while you update your servers.",
  },
  {
    icon: Code,
    title: "Updates never break old code",
    body: "We only add new fields and endpoints. Your existing API calls and SDK code keep working.",
  },
  {
    icon: Plugs,
    title: "Connect Claude or Cursor",
    body: "Plug AI coding tools into dimmi over /mcp using OAuth, with a confirmation check before edits run.",
  },
];

const questions = [
  {
    question: "Should we build our own chat or use the hosted web messenger?",
    answer:
      "Start with the hosted web messenger unless you need a custom native mobile screen or full control over the UI. The hosted messenger takes 5 minutes and includes docs search and updates.",
  },
  {
    question: "Do AI agents, workflows, and reply timers work on API chats?",
    answer:
      "Yes. Conversations from /widget/v1 and /api/v1 land in your inbox like any other channel, with the same routing, reply timers, tickets, and AI agents.",
  },
  {
    question: "Can we connect our own chat system to dimmi?",
    answer:
      "Send customer messages into any inbox with /api/v1, then listen to message.created webhooks to push teammate and AI replies back to your app.",
  },
  {
    question: "Do we need a higher plan for the API, webhooks, or SDKs?",
    answer: `No. The API, webhooks, all ${SDK_COUNT} SDKs, OAuth apps, and MCP are included on dimmi Cloud (with 100 free conversations a month) and in the $0 self-hosted build.`,
  },
  {
    question: "Can iOS and Android apps use the Messenger API?",
    answer:
      "Yes. Turn on native app access in settings and call /widget/v1 from iOS, Android, or React Native without browser cookies.",
  },
  {
    question: "Will future dimmi updates break our integration?",
    answer:
      "No. Our public API and webhook formats only add new fields, never rename or remove old ones, and every SDK is generated straight from that contract.",
  },
];

export function ApiChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Headless & API"
        breadcrumb={[
          { label: "Home", href: "/" },
          { label: "Channels", href: "/channels" },
        ]}
        title="Build your own chat UI on top of dimmi."
        body="Build custom chat for your web or mobile app with our Messenger API, or open threads from your backend. Every message lands in your dimmi inbox."
        visual={
          <ShotVideo
            name="channel-api-hero"
            poster="channel-api-hero"
            alt="A custom in-app chat client on the left sending messages over /widget/v1 while a teammate and support agent reply in the dimmi inbox on the right"
            bleed
          />
        }
      />
      <Rows title="Connect your app and backend straight to your inbox." rows={rows} />
      <FeatureGrid title="Simple tools for connecting custom apps." items={more} />
      <ChannelLinks current="/channels/api" />
      <Questions body="What developers ask before building on the dimmi API." items={questions} />
      <FinalCta />
    </>
  );
}
