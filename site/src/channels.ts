/*
 * Every channel connection dimmi ships completely, with its marketing page
 * route, short blurb, and icon. Navigation, footer, and the inbox channel
 * strip all read from here.
 */
import {
  ChatCircleText,
  Code,
  EnvelopeSimple,
  InstagramLogo,
  MessengerLogo,
  type Icon as PhosphorIcon,
  SlackLogo,
  WhatsappLogo,
} from "@phosphor-icons/react";

export type ChannelItem = {
  name: string;
  href: string;
  blurb: string;
  icon: PhosphorIcon;
};

export const CHANNELS: ChannelItem[] = [
  {
    name: "Messenger",
    href: "/channels/messenger",
    blurb: "Live chat, help center, and changelog inside your app.",
    icon: ChatCircleText,
  },
  {
    name: "Email",
    href: "/channels/email",
    blurb: "Support addresses on your own verified sending domain.",
    icon: EnvelopeSimple,
  },
  {
    name: "Slack",
    href: "/channels/slack",
    blurb: "Customer Slack Connect channels synced to your inbox.",
    icon: SlackLogo,
  },
  {
    name: "WhatsApp",
    href: "/channels/whatsapp",
    blurb: "WhatsApp Business numbers, media, and templates with $0 markup.",
    icon: WhatsappLogo,
  },
  {
    name: "Instagram",
    href: "/channels/instagram",
    blurb: "Instagram DMs beside email, chat, and customer history.",
    icon: InstagramLogo,
  },
  {
    name: "Facebook Messenger",
    href: "/channels/facebook-messenger",
    blurb: "Facebook Page messages routed straight into one queue.",
    icon: MessengerLogo,
  },
  {
    name: "Headless & API",
    href: "/channels/api",
    blurb: "Custom chat UIs, server threads, webhooks, and 11 typed SDKs.",
    icon: Code,
  },
];

export const CHANNEL_PATHS = new Set(CHANNELS.map((c) => c.href));
