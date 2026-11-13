/*
 * One email address on a page of its own: its forwarding setup, the mail
 * that arrived, its settings and the team's signature for it, reached
 * from the list of addresses on the email settings page. The address's
 * settings and whether its team signature signs replies are one draft,
 * saved or discarded from the bar at the foot; leaving with it unsaved
 * asks first.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { PageSkeleton, SettingsPage } from "@/features/settings/layout";
import { SaveBar } from "@/features/settings/save-bar";
import { useAccount } from "@/features/settings/use-account";
import { emailAddressesQuery } from "@/lib/api/email";
import { signaturesKey } from "@/lib/api/email-signatures";
import { callerQuery } from "@/lib/api/workspace";

import { useSignatureToggles } from "../signatures/signature-card";
import { TeamSignature } from "../signatures/team-signatures";
import { AddressCard, useAddressDraft } from "./address-card";

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
  const found = addresses?.find((a) => a.id === address);
  const queryClient = useQueryClient();
  const draft = useAddressDraft(me.workspace.id, found);
  const toggles = useSignatureToggles(me.workspace.id);
  const dirty = draft.dirty || toggles.dirty;
  const save = useMutation({
    mutationFn: () => Promise.all([draft.send(), toggles.send()]),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: emailAddressesQuery(me.workspace.id).queryKey }),
        queryClient.invalidateQueries({ queryKey: signaturesKey(me.workspace.id) }),
      ]);
      draft.clear();
      toggles.clear();
      toast.success("Address settings saved");
    },
  });
  useBlocker({
    shouldBlockFn: () => dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => dirty,
  });

  if (!addresses) {
    return <PageSkeleton />;
  }

  if (!found) {
    return (
      <SettingsPage title="Address not found" description="It was removed, or it belongs to another workspace.">
        <BackToEmail />
      </SettingsPage>
    );
  }

  return (
    <SettingsPage
      title={found.address}
      description={found.display_name || undefined}
      footer={
        <SaveBar
          dirty={dirty}
          saving={save.isPending}
          error={save.error?.message}
          onSave={() => save.mutate()}
          onDiscard={() => {
            save.reset();
            draft.clear();
            toggles.clear();
          }}
        />
      }
    >
      <BackToEmail />
      <AddressCard
        address={found}
        title="Setup"
        manage={permissions.includes("email:manage")}
        readRaw={permissions.includes("email:read_raw")}
        draft={draft}
      />
      {permissions.includes("email_signatures:manage_any") ? (
        <TeamSignature workspace={me.workspace.id} address={found} toggles={toggles} />
      ) : null}
    </SettingsPage>
  );
}
