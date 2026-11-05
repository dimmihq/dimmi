/*
 * Email settings on the server: the addresses the workspace receives
 * mail at, each opening a page of its own, and a way to add another, then the domains replies may come
 * from and the team's signature for each address. Each section lives in its own file
 * under features/email; this page only puts them in order. Changing
 * addresses follows the email:manage permission the server gives.
 */

import { RiArrowRightSLine, RiMailLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { DomainList } from "@/features/email/domains/domain-list";
import { AddAddress } from "@/features/email/setup/add-address";
import { AddressStatus } from "@/features/email/setup/address-status";
import { Failure } from "@/features/email/setup/failure";
import { TeamSignatures } from "@/features/email/signatures/team-signatures";
import { TeammateSignatures } from "@/features/email/signatures/teammate-signatures";
import { emailAddressesQuery } from "@/lib/api/email";
import { callerQuery } from "@/lib/api/workspace";
import { Empty, Glyph, Item, Section, SettingsPage } from "../layout";
import { rowLink } from "../row-link";
import { useAccount } from "../use-account";

export function EmailSettings() {
  const me = useAccount();
  const { data: addresses, error } = useQuery(emailAddressesQuery(me.workspace.id));
  const { data: caller } = useQuery(callerQuery(me.workspace.id));
  const permissions = caller?.member?.permissions ?? [];
  const manage = permissions.includes("email:manage");

  return (
    <SettingsPage title="Email" description="Forward your support address to dimmi and reply from it.">
      <Section title="Addresses" description="Where customers write to you." loading={!addresses} rows={2}>
        {addresses?.length === 0 ? <Empty>No address is connected yet.</Empty> : null}
        {addresses?.map((address) => (
          <Item
            key={address.id}
            mark={<Glyph icon={RiMailLine} />}
            title={
              <Link to="/d/settings/email/addresses/$address" params={{ address: address.id }} className={rowLink}>
                {address.address}
              </Link>
            }
            detail={`${address.inbox.name} inbox`}
          >
            <AddressStatus address={address} />
            <RiArrowRightSLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
          </Item>
        ))}
      </Section>
      {manage && addresses ? <AddAddress /> : null}
      <DomainList manage={manage} />
      {addresses && permissions.includes("email_signatures:manage_any") ? (
        <>
          <TeamSignatures workspace={me.workspace.id} addresses={addresses} />
          <TeammateSignatures workspace={me.workspace.id} email={me.email} addresses={addresses} />
        </>
      ) : null}
      <Failure error={error} />
    </SettingsPage>
  );
}
