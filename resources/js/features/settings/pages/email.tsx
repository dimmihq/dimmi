/*
 * Email settings on the server: the addresses the workspace receives
 * mail at, each opening a page of its own, and a way to add another, then the domains replies may come
 * from and the team's signature for each address. Each section lives in its own file
 * under features/email; this page only puts them in order. Changing
 * addresses follows the email:manage permission the server gives.
 * Turning teammates' signatures on or off waits for the save bar at the
 * foot; leaving with changes unsaved asks first.
 */

import { RiArrowRightSLine, RiMailLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useBlocker } from "@tanstack/react-router";
import { toast } from "@/components/base/toast/toast";
import { DomainList } from "@/features/email/domains/domain-list";
import { AddAddress } from "@/features/email/setup/add-address";
import { AddressStatus } from "@/features/email/setup/address-status";
import { Failure } from "@/features/email/setup/failure";
import { useSignatureToggles } from "@/features/email/signatures/signature-card";
import { TeammateSignatures } from "@/features/email/signatures/teammate-signatures";
import { emailAddressesQuery } from "@/lib/api/email";
import { signaturesKey } from "@/lib/api/email-signatures";
import { callerQuery } from "@/lib/api/workspace";
import { Empty, Glyph, Item, Section, SettingsPage } from "../layout";
import { rowLink } from "../row-link";
import { SaveBar } from "../save-bar";
import { useAccount } from "../use-account";

export function EmailSettings() {
  const me = useAccount();
  const { data: addresses, error } = useQuery(emailAddressesQuery(me.workspace.id));
  const { data: caller } = useQuery(callerQuery(me.workspace.id));
  const permissions = caller?.member?.permissions ?? [];
  const manage = permissions.includes("email:manage");
  const queryClient = useQueryClient();
  const toggles = useSignatureToggles(me.workspace.id);
  const save = useMutation({
    mutationFn: () => toggles.send(),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: signaturesKey(me.workspace.id) });
      toggles.clear();
      toast.success("Signatures saved");
    },
  });
  useBlocker({
    shouldBlockFn: () => toggles.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => toggles.dirty,
  });

  return (
    <SettingsPage
      title="Email"
      description="Forward your support address to dimmi and reply from it."
      footer={
        <SaveBar
          dirty={toggles.dirty}
          saving={save.isPending}
          error={save.error?.message}
          onSave={() => save.mutate()}
          onDiscard={() => {
            save.reset();
            toggles.clear();
          }}
        />
      }
    >
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
        <TeammateSignatures workspace={me.workspace.id} email={me.email} addresses={addresses} toggles={toggles} />
      ) : null}
      <Failure error={error} />
    </SettingsPage>
  );
}
