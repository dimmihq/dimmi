/*
 * One email address on a page of its own: its forwarding setup, the mail
 * that arrived and its settings, reached from the list of addresses on
 * the email settings page.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { PageSkeleton, SettingsPage } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { emailAddressesQuery } from "@/lib/api/email";
import { callerQuery } from "@/lib/api/workspace";

import { AddressCard } from "./address-card";

function BackToEmail() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "email" } })}
      >
        Email settings
      </Button>
    </div>
  );
}

export function AddressPage({ address }: { address: string }) {
  const me = useAccount();
  const { data: addresses } = useQuery(emailAddressesQuery(me.workspace.id));
  const { data: caller } = useQuery(callerQuery(me.workspace.id));
  const permissions = caller?.member?.permissions ?? [];

  if (!addresses) {
    return <PageSkeleton />;
  }

  const found = addresses.find((a) => a.id === address);
  if (!found) {
    return (
      <SettingsPage title="Address not found" description="It was removed, or it belongs to another workspace.">
        <BackToEmail />
      </SettingsPage>
    );
  }

  return (
    <SettingsPage title={found.address} description={found.display_name || undefined}>
      <BackToEmail />
      <AddressCard
        address={found}
        title="Setup"
        manage={permissions.includes("email:manage")}
        readRaw={permissions.includes("email:read_raw")}
      />
    </SettingsPage>
  );
}
