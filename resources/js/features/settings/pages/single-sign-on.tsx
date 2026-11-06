/*
 * Single sign-on for the company behind the workspace: its connections to
 * identity providers and the domains it proves it owns. Only the people
 * who manage single sign-on reach it; anyone else learns who to ask.
 */
import { useQuery } from "@tanstack/react-query";

import { callerQuery } from "@/lib/api/workspace";

import { Empty, Section, SettingsPage } from "../layout";
import { ConnectionsSection } from "../single-sign-on/connections";
import { DomainsSection } from "../single-sign-on/domains";
import { useAccount } from "../use-account";

export function SingleSignOnSettings() {
  const workspace = useAccount().workspace.id;
  const { data: caller } = useQuery(callerQuery(workspace));
  const canManage = caller?.member?.permissions.includes("sso:manage") ?? false;

  return (
    <SettingsPage title="Single sign-on" description="Let people sign in through your company's identity provider.">
      {caller && !canManage ? (
        <Section title="Single sign-on">
          <Empty>Only the workspace owners manage single sign-on.</Empty>
        </Section>
      ) : null}
      {canManage ? (
        <>
          <ConnectionsSection canManage={canManage} />
          <DomainsSection canManage={canManage} />
        </>
      ) : null}
    </SettingsPage>
  );
}
