/*
 * The link checks of the thread being read, taken from the workspace's
 * settings. When the workspace hides the links of email, an email
 * conversation shows them as plain text, and a teammate allowed to read
 * raw mail can show them for as long as they stay on the conversation.
 */
import { RiLinkM } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { callerQuery, workspaceQuery } from "@/lib/api/workspace";

import { useInbox } from "../conversations";
import { LinkPolicyContext } from "./link-checks";

const defaults = { enabled: true, allow_domains: [], block_domains: [], strip_inbound_email_links: false };

export function LinkPolicyProvider({ channelType, children }: { channelType?: string; children: ReactNode }) {
  const { workspace } = useInbox();
  const { data: current } = useQuery({ ...workspaceQuery(workspace), enabled: workspace !== "" });
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const [shown, setShown] = useState(false);
  const settings = current?.link_safety ?? defaults;
  const hidden = channelType === "email" && settings.strip_inbound_email_links;
  const canReveal = caller?.member?.permissions.includes("email:read_raw") ?? false;

  return (
    <LinkPolicyContext.Provider value={{ settings, plain: hidden && !shown }}>
      {hidden && canReveal ? (
        <div className="mx-auto flex w-full max-w-3xl justify-end px-3 pb-1">
          <Button variant="ghost" size="small" leadingIcon={RiLinkM} onClick={() => setShown((now) => !now)}>
            {shown ? "Hide links" : "Show links"}
          </Button>
        </div>
      ) : null}
      {children}
    </LinkPolicyContext.Provider>
  );
}
