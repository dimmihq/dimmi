/*
 * Where a conversation stands on Messenger or Instagram, for the
 * composer: read only for conversations on those two channels, nothing
 * while a note is written, and read again as soon as a new message lands
 * in the thread, since a customer's message starts Meta's time over.
 */
import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";

import { messagesQuery } from "@/lib/api/inbox";
import { type SocialPlatform, socialConversationQuery } from "@/lib/api/social";

const platforms: Record<string, SocialPlatform> = { facebook: "facebook", instagram: "instagram" };

export const socialNames: Record<SocialPlatform, string> = { facebook: "Messenger", instagram: "Instagram" };

/*
 * useSocialWindow reads where the conversation stands, for a
 * conversation on Messenger or Instagram only, and not while a note is
 * written.
 */
export function useSocialWindow(workspace: string, conversation: string, channelType: string, note: boolean) {
  const platform = platforms[channelType];
  const query = useQuery({
    ...socialConversationQuery(workspace, platform ?? "facebook", conversation),
    enabled: platform !== undefined,
  });
  const newest = useQuery({
    ...messagesQuery(workspace, conversation),
    enabled: false,
    select: (messages) => messages.at(-1)?.id ?? null,
  }).data;
  const { refetch } = query;

  useEffect(() => {
    if (platform && newest) {
      void refetch();
    }
  }, [platform, newest, refetch]);

  return note || !platform ? undefined : query.data;
}

/*
 * left says how long is left until the moment, in days once it is more
 * than two, else hours and minutes; null once it passed.
 */
export function left(until: string | null, now = Date.now()): string | null {
  if (!until) {
    return null;
  }

  const minutes = Math.floor((Date.parse(until) - now) / 60_000);
  if (minutes <= 0) {
    return null;
  }

  if (minutes >= 48 * 60) {
    return `${Math.floor(minutes / 1440)}d left`;
  }

  if (minutes >= 60) {
    return `${Math.floor(minutes / 60)}h ${minutes % 60}m left`;
  }

  return `${minutes}m left`;
}
