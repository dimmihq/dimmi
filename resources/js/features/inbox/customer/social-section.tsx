/*
 * Who the customer is on Messenger or Instagram, for a conversation on
 * one of them: their username with a link to their profile, and on
 * Instagram how many follow them, whether Instagram verified them and
 * whether they follow the account. Meta says this only about someone
 * who wrote to the account; when it says nothing, the section is left
 * out.
 */
import { RiVerifiedBadgeFill } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { type SocialPlatform, socialConversationQuery } from "@/lib/api/social";

import { Row, Section } from "./rows";

const titles: Record<SocialPlatform, string> = { facebook: "Facebook Messenger", instagram: "Instagram" };

const count = new Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 });

export function SocialSection({
  workspace,
  conversation,
  channelType,
}: {
  workspace: string;
  conversation: string;
  channelType: string;
}) {
  const platform = channelType === "instagram" || channelType === "facebook" ? channelType : null;
  const { data } = useQuery({
    ...socialConversationQuery(workspace, platform ?? "instagram", conversation),
    enabled: platform !== null,
  });
  const profile = data?.profile;

  if (!platform || !profile || (!profile.username && !profile.name)) {
    return null;
  }

  return (
    <Section title={titles[platform]}>
      <dl className="flex w-full flex-col">
        {profile.username ? (
          <Row label="Username">
            <a
              href={`https://www.instagram.com/${encodeURIComponent(profile.username)}/`}
              target="_blank"
              rel="noreferrer"
              className="flex min-w-0 items-center gap-1 truncate text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <span className="truncate">@{profile.username}</span>
              {profile.verified ? (
                <RiVerifiedBadgeFill aria-label="Verified" className="size-4 shrink-0 text-status-blue-text" />
              ) : null}
            </a>
          </Row>
        ) : (
          <Row label="Name">
            <span className="truncate">{profile.name}</span>
          </Row>
        )}
        {platform === "instagram" ? (
          <>
            <Row label="Followers">
              <span className="truncate">{count.format(profile.followers)}</span>
            </Row>
            <Row label="Follows you">
              <span className="truncate">{profile.follows ? "Yes" : "No"}</span>
            </Row>
          </>
        ) : null}
      </dl>
    </Section>
  );
}
