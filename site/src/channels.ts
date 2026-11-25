/*
 * Every channel connection dimmi ships completely, with its marketing page
 * route, short blurb, and official brand mark. Navigation, footer, the
 * channels hub, and cross-link strips all read from here.
 */
import type { ComponentType } from "react";

import {
  ApiBrandLogo,
  type ChannelLogoProps,
  EmailBrandLogo,
  FacebookMessengerBrandLogo,
  InstagramBrandLogo,
  MessengerBrandLogo,
  SlackBrandLogo,
  WhatsAppBrandLogo,
} from "./components/channel-logos";

export type ChannelItem = {
  name: string;
  href: string;
  blurb: string;
  logo: ComponentType<ChannelLogoProps>;
};

export const CHANNELS: ChannelItem[] = [
  {
    name: "Messenger",
    href: "/channels/messenger",
    blurb: "Live chat, help center, and updates inside your app.",
    logo: MessengerBrandLogo,
  },
  {
    name: "Email",
    href: "/channels/email",
    blurb: "Support email addresses on your own verified domain.",
    logo: EmailBrandLogo,
  },
  {
    name: "Slack",
    href: "/channels/slack",
    blurb: "Customer Slack Connect channels synced to your inbox.",
    logo: SlackBrandLogo,
  },
  {
    name: "WhatsApp",
    href: "/channels/whatsapp",
    blurb: "WhatsApp Business numbers, media, and templates with $0 markup.",
    logo: WhatsAppBrandLogo,
  },
  {
    name: "Instagram",
    href: "/channels/instagram",
    blurb: "Instagram DMs beside email, chat, and customer history.",
    logo: InstagramBrandLogo,
  },
  {
    name: "Facebook Messenger",
    href: "/channels/facebook-messenger",
    blurb: "Facebook Page messages routed straight into one inbox.",
    logo: FacebookMessengerBrandLogo,
  },
  {
    name: "Headless & API",
    href: "/channels/api",
    blurb: "Custom chat screens, backend APIs, webhooks, and 11 SDKs.",
    logo: ApiBrandLogo,
  },
];

export const CHANNEL_PATHS = new Set(["/channels", ...CHANNELS.map((c) => c.href)]);
