/*
 * The domains the company proves it owns. A domain is proven by a TXT
 * record the owner copies into their DNS, or by the operator of the
 * install; once proven, the people at it can be sent to a connection.
 * Checking shows what the DNS answered, so a typo in the record is easy
 * to spot.
 */
import { RiGlobalLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import {
  addDomain,
  checkDomain,
  connectionsQuery,
  type DomainCheck,
  domainsQuery,
  type OrganizationDomain,
  removeDomain,
  routeDomain,
} from "@/lib/api/sso";

import { Action, Danger } from "../controls";
import { Empty, Glyph, Item, RowsSkeleton, Section } from "../layout";
import { CopyField } from "./copy-field";
import { useSSOChange } from "./use-sso";

const unrouted = "none";

const statusLabels: Record<OrganizationDomain["status"], string> = {
  pending: "Not verified",
  verified: "Verified",
  failed: "Lost",
};

function AddDomainDialog({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const { send, failure, prompt } = useSSOChange();
  const [domain, setDomain] = useState("");

  return (
    <>
      <AlertDialog
        isOpen={isOpen}
        onOpenChange={(open) => (open ? null : onClose())}
        variant="default"
        icon={false}
        title="Add domain"
        description={failure ?? "The domain your people's email addresses end in, such as acme.com."}
        confirmLabel="Add domain"
        isConfirmDisabled={domain.trim() === ""}
        onConfirm={async () => {
          if (await send((workspace) => addDomain(workspace, domain.trim()), true)) {
            setDomain("");
          }
        }}
      >
        <Input label="Domain" value={domain} onChange={setDomain} placeholder="acme.com" />
      </AlertDialog>
      {prompt}
    </>
  );
}

function RecordDialog({ domain, onClose }: { domain: OrganizationDomain | null; onClose: () => void }) {
  const { send, failure, prompt } = useSSOChange();
  const [check, setCheck] = useState<DomainCheck | null>(null);
  const close = () => {
    setCheck(null);
    onClose();
  };
  const found = check && check.status !== "verified" ? check.found_records : null;

  return (
    <>
      <Modal size="lg" isOpen={domain !== null} onOpenChange={(open) => (open ? null : close())}>
        <ModalHeader
          title={`Verify ${domain?.domain ?? "domain"}`}
          description="Add this TXT record at your DNS provider, then check it. DNS can take a while to update."
        />
        <ModalBody className="flex flex-col gap-4">
          {domain ? (
            <>
              <CopyField label="Name" value={domain.record_name} />
              <CopyField label="Value" value={domain.record_value} />
            </>
          ) : null}
          {found ? (
            <p role="status" className="text-body-2-regular text-text-secondary">
              {found.length === 0 ? "No TXT record found yet." : `Found instead: ${found.join(", ")}`}
            </p>
          ) : null}
          {failure ? (
            <p role="alert" className="text-body-2-regular text-text-error-primary">
              {failure}
            </p>
          ) : null}
        </ModalBody>
        <ModalFooter className="mt-6">
          <Button variant="secondary" onClick={close}>
            Done
          </Button>
          <Button
            onClick={async () => {
              const target = domain?.id;
              if (!target) {
                return;
              }

              await send(async (workspace) => {
                const result = await checkDomain(workspace, target);
                setCheck(result);
                if (result.status === "verified") {
                  close();
                }
              });
            }}
          >
            Check now
          </Button>
        </ModalFooter>
      </Modal>
      {prompt}
    </>
  );
}

function RouteSelect({ domain, canManage }: { domain: OrganizationDomain; canManage: boolean }) {
  const { send, workspace } = useSSOChange();
  const { data: connections } = useQuery(connectionsQuery(workspace));

  return (
    <Select
      aria-label={`Connection for ${domain.domain}`}
      size="sm"
      isDisabled={!canManage}
      selectedKey={domain.sso_connection_id ?? unrouted}
      onSelectionChange={(key) => void send((id) => routeDomain(id, domain.id, key === unrouted ? null : String(key)))}
      popoverClassName="w-56"
    >
      {[{ id: unrouted, name: "No connection" }, ...(connections ?? [])].map((connection) => (
        <SelectItem key={connection.id} id={connection.id} textValue={connection.name}>
          {connection.name}
        </SelectItem>
      ))}
    </Select>
  );
}

export function DomainsSection({ canManage }: { canManage: boolean }) {
  const { send, failure, prompt, workspace } = useSSOChange();
  const { data: domains } = useQuery(domainsQuery(workspace));
  const [adding, setAdding] = useState(false);
  const [verifying, setVerifying] = useState<OrganizationDomain | null>(null);

  return (
    <Section
      title="Domains"
      description="Prove the company owns a domain, then choose the connection its people sign in through."
      action={canManage ? <Action onPress={() => setAdding(true)}>Add domain</Action> : undefined}
    >
      {domains ? null : <RowsSkeleton rows={1} />}
      {domains?.length === 0 ? <Empty>No domains yet.</Empty> : null}
      {domains?.map((domain) => (
        <Item
          key={domain.id}
          mark={<Glyph icon={RiGlobalLine} />}
          title={domain.domain}
          detail={domain.verified_via === "cli" ? "Verified by the operator" : domain.record_name}
        >
          <Chip variant="caption" color={domain.status === "verified" ? "lime" : "neutral"}>
            {statusLabels[domain.status]}
          </Chip>
          {domain.status === "verified" ? (
            <RouteSelect domain={domain} canManage={canManage} />
          ) : (
            <Action size="xs" onPress={() => setVerifying(domain)}>
              Verify
            </Action>
          )}
          {canManage ? <Danger onPress={() => void send((id) => removeDomain(id, domain.id))}>Remove</Danger> : null}
        </Item>
      ))}
      {failure ? (
        <p role="alert" className="py-2.5 pe-3 text-body-2-regular text-text-error-primary">
          {failure}
        </p>
      ) : null}
      <AddDomainDialog isOpen={adding} onClose={() => setAdding(false)} />
      <RecordDialog domain={verifying} onClose={() => setVerifying(null)} />
      {prompt}
    </Section>
  );
}
