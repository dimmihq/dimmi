/*
 * The chat apps a workspace can connect, each with its brand colour, so
 * settings and setup list them the same way.
 */
import { RiDiscordFill, RiSlackFill, RiTelegramFill, RiWhatsappFill } from "@remixicon/react";

export const chatApps = [
  { name: "Slack", icon: RiSlackFill, brand: "#4A154B", detail: "Shared channels and Slack Connect" },
  { name: "Discord", icon: RiDiscordFill, brand: "#5865F2", detail: "A support channel in your server" },
  { name: "Telegram", icon: RiTelegramFill, brand: "#26A5E4", detail: "Your bot's direct messages" },
  { name: "WhatsApp", icon: RiWhatsappFill, brand: "#25D366", detail: "Your business number" },
];
