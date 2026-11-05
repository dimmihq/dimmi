/*
 * The two address allowlists: one binds people in a browser or app, the
 * other tokens, since an office network and a CI runner share nothing.
 * Empty means anywhere. The server refuses a dashboard list that leaves
 * out the caller's own address; the page shows that address and says why,
 * and warns when a saved API list would stop tokens already in use.
 */
import { RiGlobalLine } from "@remixicon/react";
import { useState } from "react";

import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import type { AccessPolicy, AllowedAddress } from "@/lib/api/access-policy";

import { Action, Danger } from "../controls";
import { Empty, Glyph, Item, Section } from "../layout";
import { useAccessPolicy } from "./use-access-policy";

type ListKey = "dashboard_ip_allowlist" | "api_ip_allowlist";

function AddAddressDialog({
  title,
  isOpen,
  onClose,
  onAdd,
}: {
  title: string;
  isOpen: boolean;
  onClose: () => void;
  onAdd: (entry: AllowedAddress) => Promise<boolean>;
}) {
  const [cidr, setCidr] = useState("");
  const [label, setLabel] = useState("");

  return (
    <AlertDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      variant="default"
      icon={false}
      title={title}
      description="A single address or a range, such as 203.0.113.0/24."
      confirmLabel="Add address"
      isConfirmDisabled={cidr.trim() === "" || label.trim() === ""}
      onConfirm={async () => {
        if (await onAdd({ cidr: cidr.trim(), label: label.trim() })) {
          setCidr("");
          setLabel("");
        }
      }}
    >
      <div className="flex flex-col gap-3">
        <Input label="Address or range" value={cidr} onChange={setCidr} placeholder="203.0.113.0/24" />
        <Input label="Label" value={label} onChange={setLabel} placeholder="Office" maxLength={60} />
      </div>
    </AlertDialog>
  );
}

function AllowlistSection({
  policy,
  list,
  title,
  description,
  canManage,
  save,
  failure,
}: {
  policy: AccessPolicy;
  list: ListKey;
  title: string;
  description: string;
  canManage: boolean;
  save: (entries: AllowedAddress[]) => Promise<boolean>;
  failure: string | null;
}) {
  const [adding, setAdding] = useState(false);
  const entries = policy[list];

  return (
    <Section
      title={title}
      description={description}
      action={canManage ? <Action onPress={() => setAdding(true)}>Add address</Action> : undefined}
    >
      {entries.length === 0 ? <Empty>Any address may connect.</Empty> : null}
      {entries.map((entry) => (
        <Item key={entry.cidr} mark={<Glyph icon={RiGlobalLine} />} title={entry.label} detail={entry.cidr}>
          {canManage ? (
            <Danger onPress={() => void save(entries.filter((other) => other.cidr !== entry.cidr))}>Remove</Danger>
          ) : null}
        </Item>
      ))}
      {failure ? (
        <p role="alert" className="py-2.5 pe-3 text-body-2-regular text-text-error-primary">
          {failure}
        </p>
      ) : null}
      <AddAddressDialog
        title={title}
        isOpen={adding}
        onClose={() => setAdding(false)}
        onAdd={async (entry) => {
          const saved = await save([...entries, entry]);
          setAdding(false);

          return saved;
        }}
      />
    </Section>
  );
}

export function IPAllowlistSections() {
  const dashboard = useAccessPolicy();
  const api = useAccessPolicy();
  const { policy, canManage } = dashboard;

  if (!policy) {
    return null;
  }

  return (
    <>
      <AllowlistSection
        policy={policy}
        list="dashboard_ip_allowlist"
        title="Dashboard IP allowlist"
        description={`People in a browser or app. Your address: ${policy.your_ip}.`}
        canManage={canManage}
        failure={dashboard.failure}
        save={(entries) => dashboard.save({ dashboard_ip_allowlist: entries })}
      />
      {dashboard.prompt}
      <AllowlistSection
        policy={policy}
        list="api_ip_allowlist"
        title="API IP allowlist"
        description="Access tokens and the command line. Directory sync is not bound by it: identity providers connect from ranges that change."
        canManage={canManage}
        failure={api.failure}
        save={(entries) => api.save({ api_ip_allowlist: entries })}
      />
      {api.prompt}
      {policy.tokens_outside > 0 ? (
        <p role="status" className="text-body-2-regular text-text-error-primary">
          {policy.tokens_outside === 1
            ? "1 token in use was last seen outside this list and will stop working."
            : `${policy.tokens_outside} tokens in use were last seen outside this list and will stop working.`}
        </p>
      ) : null}
    </>
  );
}
