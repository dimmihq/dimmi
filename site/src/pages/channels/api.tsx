import { ArrowsClockwise, Broadcast, Code, Key, Plugs, ShieldCheck } from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Messenger API",
    title: "Build native web and mobile chat on /widget/v1.",
    body: "Render chat inside your own web or mobile app with zero iframes or cookies. Sign a JWT on your server so dimmi checks each customer's identity and trusts their account attributes.",
    visual: (
      <ShotStill
        name="channel-api-messenger"
        alt="Messenger API settings showing the app ID, allowed sites, native app toggle, and identity verification signing secrets"
      />
    ),
  },
  {
    label: "REST API",
    title: "Open and answer conversations straight from your backend.",
    body: "Each inbox includes a built-in API channel. Call /api/v1 from your backend to open conversations, post messages, update customer records, and track product events with scoped API tokens.",
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
    label: "Signed webhooks",
    title: "Push real-time events into your app, CRM, or custom bridge.",
    body: "dimmi signs payloads with Standard Webhooks and retries failures up to 12 times over 24 hours. Subscribe with wildcards, skip repeats by event ID, and replay missed calls anytime.",
    visual: (
      <ShotStill
        name="channel-api-webhooks"
        alt="Webhook endpoint view showing Standard Webhooks signing secret rotation, subscribed event patterns, and delivery retry history"
      />
    ),
  },
  {
    label: "SDKs, OAuth & MCP",
    title: "11 typed SDKs, OAuth apps, MCP, and llms.txt included.",
    body: "Install typed SDKs for TypeScript, Go, PHP, Laravel, Python, Browser, React, Next.js, Vue, Angular, and Svelte. Register OAuth 2 apps, connect Claude or Cursor over /mcp, and feed docs to coding agents with llms.txt.",
    visual: (
      <ShotStill
        name="channel-api-sdks"
        alt="Developer tools overview showing the 11 typed SDKs, OAuth 2 app settings, remote MCP server URL, and llms.txt documentation index"
      />
    ),
  },
];

const more = [
  {
    icon: ShieldCheck,
    title: "JWT and JWKS identity",
    body: "Sign customer tokens with HS256, RSA, P-256, or Ed25519 keys, or point dimmi at your auth provider's JWKS URL.",
  },
  {
    icon: Broadcast,
    title: "Realtime streams or cursor polling",
    body: "Request a realtime stream token for live updates in custom chat clients, or poll /updates with a cursor.",
  },
  {
    icon: Key,
    title: "Scoped API token abilities",
    body: "Lock each API token to the exact read or write abilities it needs, capped by your workspace role.",
  },
  {
    icon: ArrowsClockwise,
    title: "Rotate secrets without downtime",
    body: "Keep old and new webhook or identity keys active side by side while your servers roll out.",
  },
  {
    icon: Code,
    title: "Additive public contracts",
    body: "OpenAPI and webhook schemas only add fields. Existing paths, types, and SDK calls stay working across releases.",
  },
  {
    icon: Plugs,
    title: "MCP server with confirmation",
    body: "Connect Claude, ChatGPT, or Cursor to /mcp over OAuth. Sensitive writes and deletes pause for your approval.",
  },
];

const questions = [
  {
    question: "When should we skip headless and use the hosted messenger instead?",
    answer:
      "Most teams should start with the hosted messenger. It ships in 5 minutes with help search, tickets, changelog, and unread badges ready to go. Build on /widget/v1 only when you need custom native mobile screens or strict UI control.",
  },
  {
    question: "Do dimmi support agents, workflows, and SLAs work on API threads?",
    answer:
      "Conversations opened through /widget/v1 or /api/v1 land in your shared inbox with the same routing, SLAs, tickets, and workflows. Support agents answer those threads, run procedures, and hand over to teammates like any other channel.",
  },
  {
    question: "Can we bridge our own chat channel or internal app into dimmi?",
    answer:
      "Open threads and post customer messages into any inbox's API channel with /api/v1, then subscribe to message.created webhooks to push teammate and agent replies back to your app. Stable event IDs and sequence numbers keep both sides in sync.",
  },
  {
    question: "Do API access, webhooks, or SDKs require a higher plan?",
    answer:
      "All 11 SDKs, /api/v1, /widget/v1, webhooks, OAuth apps, and /mcp come included on dimmi Cloud, starting with 100 free conversations a month, and in the self-hosted build for $0.",
  },
  {
    question: "Can native iOS and Android apps call the Messenger API?",
    answer:
      "Turn on native app access for your messenger and call /widget/v1 from iOS, Android, or React Native without cookies. Pass a JWT signed by your server to tie sessions to customers and merge anonymous threads.",
  },
  {
    question: "Will an API update break our integration or SDKs?",
    answer:
      "Public contracts for /api/v1, /widget/v1, and webhooks are additive only. We add fields and endpoints rather than renaming or removing existing ones, and we generate all 11 SDKs from the committed OpenAPI contract so types stay in sync.",
  },
];

export function ApiChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Headless & API"
        title="Own every pixel of customer chat without rebuilding a helpdesk."
        body="Ship your own web or mobile chat on /widget/v1, or open threads from your backend with /api/v1. Your team and support agents answer them from one shared inbox."
        visual={
          <ShotVideo
            name="channel-api-hero"
            poster="channel-api-hero"
            alt="A custom in-app chat client sending messages over /widget/v1 while a teammate and support agent reply in the dimmi inbox"
            bleed
          />
        }
      />
      <Rows title="Skip the iframe hacks and wire support straight into your app." rows={rows} />
      <FeatureGrid title="Built like infrastructure your team will not regret maintaining." items={more} />
      <Questions body="What engineering leads ask before building on the dimmi API." items={questions} />
      <FinalCta />
    </>
  );
}
