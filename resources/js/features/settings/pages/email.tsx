/*
 * Email settings on the server: each address the workspace receives
 * mail at and a way to add another, then the domains replies may come
 * from and the team's signature for each address. Each section lives in its own file
 * under features/email; this page only puts them in order. Changing
 * addresses follows the email:manage permission the server gives.
 */
import { useQuery } from "@tanstack/react-query";

import { DomainList } from "@/features/email/domains/domain-list";
import { AddAddress } from "@/features/email/setup/add-address";
import { AddressCard } from "@/features/email/setup/address-card";
import { Failure } from "@/features/email/setup/failure";
import { TeamSignatures } from "@/features/email/signatures/team-signatures";
import { TeammateSignatures } from "@/features/email/signatures/teammate-signatures";
import { emailAddressesQuery } from "@/lib/api/email";
import { callerQuery } from "@/lib/api/workspace";

import { Empty, Section, SettingsPage } from "../layout";
import { useAccount } from "../use-account";

export function EmailSettings() {
  const me = useAccount();
  const { data: addresses, error } = useQuery(emailAddressesQuery(me.workspace.id));
  const { data: caller } = useQuery(callerQuery(me.workspace.id));
  const permissions = caller?.member?.permissions ?? [];
  const manage = permissions.includes("email:manage");

  return (
    <SettingsPage title="Email" description="Forward your support address to dimmi and reply from it.">
      {addresses ? null : <Section title="Addresses" loading rows={2} />}
      {addresses && addresses.length === 0 ? (
        <Section title="Addresses">
          <Empty>No address is connected yet.</Empty>
        </Section>
      ) : null}
      {addresses?.map((address) => (
        <AddressCard
          key={address.id}
          address={address}
          manage={manage}
          readRaw={permissions.includes("email:read_raw")}
        />
      ))}
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
