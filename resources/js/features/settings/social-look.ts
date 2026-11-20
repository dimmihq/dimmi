/*
 * How Facebook Pages and Instagram accounts look and read in settings:
 * each platform's brand colour and mark, what one of its accounts is
 * called, and the words its pages use.
 */
import { RiInstagramLine, RiMessengerLine } from "@remixicon/react";

import type { SocialPlatform } from "@/lib/api/social";

export const socialLook = {
  facebook: {
    name: "Facebook Messenger",
    icon: RiMessengerLine,
    brand: "#0866FF",
    account: "Page",
    accounts: "Pages",
    pitch: "Answer people who message your Facebook Page.",
    description:
      "People message your Facebook Page and the conversation lands in the inbox. You can reply for 7 days after their last message.",
    empty: "No Facebook Page yet. Connect one to answer its messages here.",
  },
  instagram: {
    name: "Instagram",
    icon: RiInstagramLine,
    brand: "#E1306C",
    account: "account",
    accounts: "Accounts",
    pitch: "Answer direct messages to your Instagram account.",
    description:
      "People message your Instagram professional account and the conversation lands in the inbox. You can reply for 7 days after their last message.",
    empty: "No Instagram account yet. Connect one to answer its direct messages here.",
  },
} as const satisfies Record<SocialPlatform, unknown>;

/*
 * handleOf is how an account is told apart from its name: an Instagram
 * username with its at sign, nothing for a Page.
 */
export function handleOf(account: { username: string }): string {
  return account.username ? `@${account.username}` : "";
}
