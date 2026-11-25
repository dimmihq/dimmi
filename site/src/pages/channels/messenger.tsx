import { ChatCircleText, ChatsCircle, Code, LockKey, Translate, UsersThree } from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Install",
    title: "One line of HTML, or 6 typed framework packages.",
    body: "Paste a single script tag, or install packages for Browser, React, Next.js, Vue, Angular, and Svelte. Run multiple messengers per inbox across your sites and brands.",
    visual: (
      <ShotStill
        name="channel-messenger-install"
        alt="Messenger install settings showing the framework picker for HTML, Browser, React, Next.js, Vue, Angular, and Svelte with a one-line snippet"
      />
    ),
  },
  {
    label: "Home tab",
    title: "Chat, docs, release notes, and feature requests without leaving your app.",
    body: "Customers start a conversation, search your help center, and browse feature requests from one home screen. New changelog posts trigger an unread dot so people spot releases right away.",
    visual: (
      <ShotVideo
        name="channel-messenger-home"
        poster="channel-messenger-home"
        alt="Opening the dimmi messenger home tab to start a conversation, search help articles, check the changelog unread dot, and browse feature requests"
        maxWidth={520}
      />
    ),
  },
  {
    label: "Identity verification",
    title: "Signed customer tokens your support agents can trust.",
    body: "Never trust a browser console with account identity. Your server signs a short HMAC token, or dimmi checks your auth provider's JWKS, before loading customer attributes and product events.",
    visual: (
      <ShotStill
        name="channel-messenger-identity"
        alt="Identity verification settings with HS256 signing secrets, token debugger, and a conversation showing verified customer attributes and product events"
      />
    ),
  },
  {
    label: "Brand & security",
    title: "Matches your theme, respects strict CSP, and never fakes a human.",
    body: "Style the messenger in light or dark mode, and lock it down with allowed sites and CSP nonces. Support agents show an AI badge and hand over with a summary.",
    visual: (
      <ShotStill
        name="channel-messenger-brand"
        alt="Messenger appearance and security settings showing brand colors, light and dark mode, allowed sites, CSP nonce, and an AI-badged agent handover"
      />
    ),
  },
];

const more = [
  {
    icon: Code,
    title: "Open from your own buttons",
    body: "Hide the default launcher and trigger chat from any button using a data-dimmi attribute or the JavaScript API.",
  },
  {
    icon: ChatsCircle,
    title: "Live unread count hooks",
    body: "Show unread badges inside your own navigation bar with React, Vue, Angular, and Svelte hooks or browser events.",
  },
  {
    icon: UsersThree,
    title: "Audience visibility controls",
    body: "Restrict the launcher or new conversations to saved audiences, such as paid customers, without hiding past replies.",
  },
  {
    icon: Translate,
    title: "Built for global customers",
    body: "Translate your greeting, away message, and labels for any language, with automatic fallback to dimmi's default text.",
  },
  {
    icon: ChatCircleText,
    title: "Pre-fill context on click",
    body: "Launch a new conversation with a starting message from billing errors, upgrade prompts, or docs pages.",
  },
  {
    icon: LockKey,
    title: "Safe key rotation and checks",
    body: "Rotate identity secrets with 7 days of overlap, revoke leaked keys right away, and inspect any JWT in settings.",
  },
];

const questions = [
  {
    question: "Do we have to build a token signing endpoint from scratch?",
    answer:
      "Not if you use Clerk, Auth.js, Supabase, or Better Auth with our Next.js package, or point dimmi at your auth provider's JWKS URL. For custom setups, our Express, Go, PHP, Laravel, and Python SDKs sign tokens in a few lines.",
  },
  {
    question: "Will the messenger break our strict Content Security Policy?",
    answer:
      "Add your dimmi host to script-src, connect-src, and img-src; style-src needs no changes. Sites using strict-dynamic pass a per-request nonce on the script tag or inside any framework package.",
  },
  {
    question: "Can we hide the AI badge so the support agent looks like a human?",
    answer:
      "We do not allow that. Replies from the dimmi agent always display a clear AI badge in the customer's language so people know whether they are talking to software or a teammate.",
  },
  {
    question: "Can we run different messengers on our marketing site and inside our app?",
    answer:
      "Create separate messengers inside one workspace, each with its own theme, greeting, allowed domains, and help center. You can require sign-in inside your app while leaving anonymous chat open on your marketing pages.",
  },
  {
    question: "When should we skip the web messenger?",
    answer:
      "Look elsewhere if you need a native iOS or Android chat SDK, or a marketing pop-up for lead scoring and ad funnels. We built the messenger for web apps and sites, though you can build a custom client on the Messenger API.",
  },
  {
    question: "Does adding the messenger cost extra per site or per teammate?",
    answer:
      "Unlimited seats and multiple messengers come standard on every workspace. dimmi Cloud includes 100 free conversations every month, and self-hosting runs the exact same messenger on your own server for $0.",
  },
];

export function MessengerChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Web messenger"
        title="Put support, docs, and updates inside your app with one line."
        body="Drop in a single script tag or pick one of 6 framework packages. Customers chat, search docs, and read release notes while signed tokens confirm who signed in."
        visual={
          <ShotVideo
            name="channel-messenger-hero"
            poster="channel-messenger-hero"
            alt="The dimmi web messenger open inside a customer app, showing conversations, help center search, and signed-in customer context"
            bleed
          />
        }
      />
      <Rows title="Ship support in minutes, without shipping a mystery script to production." rows={rows} />
      <FeatureGrid title="Small details engineers look for before shipping chat to production." items={more} />
      <Questions
        body="What engineers and support leads ask before dropping the messenger into production."
        items={questions}
      />
      <FinalCta />
    </>
  );
}
