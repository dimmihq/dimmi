/*
 * The company's connections to identity providers. Each shows where it
 * stands and what the identity provider must be told about this side.
 * A tested connection is turned on once a verified domain routes to it;
 * enforcing it makes everyone at those domains sign in through it, so it
 * takes the owner's word that owners can still get in.
 */
import { RiLoginBoxLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import {
  type ConnectionStatus,
  connectionsQuery,
  deleteConnection,
  type SSOConnection,
  updateConnection,
} from "@/lib/api/sso";

import { Action, Danger } from "../controls";
import { Empty, Glyph, Item, RowsSkeleton, Section } from "../layout";
import { ConnectionDialog } from "./connection-dialog";
import { CopyField } from "./copy-field";
import { useSSOChange } from "./use-sso";

const statusLabels: Record<ConnectionStatus, string> = {
  draft: "Draft",
  tested: "Tested",
  active: "On",
  disabled: "Off",
};

const protocolLabels: Record<SSOConnection["protocol"], string> = { saml: "SAML", oidc: "OpenID Connect" };

/*
 * spFields lists what the identity provider must be told about this side
 * of the connection.
 */
function spFields(connection: SSOConnection): [string, string][] {
  const { sp } = connection;
  const fields: [string, string | undefined][] =
    connection.protocol === "oidc"
      ? [["Redirect URI", sp.redirect_uri]]
      : [
          ["Entity ID", sp.entity_id],
          ["Reply URL (ACS)", sp.acs_url],
          ["Metadata URL", sp.metadata_url],
        ];

  return fields.filter((field): field is [string, string] => Boolean(field[1]));
}

function ConnectionDetails({ connection, onClose }: { connection: SSOConnection | null; onClose: () => void }) {
  return (
    <Modal size="lg" isOpen={connection !== null} onOpenChange={(open) => (open ? null : onClose())}>
      <ModalHeader
        title={connection?.name ?? "Connection"}
        description="Give your identity provider these values when you add the application there."
      />
      <ModalBody className="flex flex-col gap-4">
        {connection
          ? spFields(connection).map(([label, value]) => <CopyField key={label} label={label} value={value} />)
          : null}
      </ModalBody>
      <ModalFooter className="mt-6">
        <Button variant="secondary" onClick={onClose}>
          Done
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * pending is a change to confirm first: enforcing, or deleting.
 */
type Pending = { kind: "enforce" | "delete"; connection: SSOConnection };

function ConnectionItem({
  connection,
  canManage,
  onShow,
  onConfirm,
  change,
}: {
  connection: SSOConnection;
  canManage: boolean;
  onShow: () => void;
  onConfirm: (pending: Pending) => void;
  change: (status: { status?: ConnectionStatus; enforced?: boolean }) => void;
}) {
  const { status, enforced } = connection;

  return (
    <Item
      mark={<Glyph icon={RiLoginBoxLine} />}
      title={connection.name}
      detail={[protocolLabels[connection.protocol], enforced ? "Required" : null].filter(Boolean).join(" · ")}
    >
      <Chip variant="caption" color={status === "active" ? "lime" : "neutral"}>
        {statusLabels[status]}
      </Chip>
      <Action size="xs" onPress={onShow}>
        Details
      </Action>
      {canManage && (status === "tested" || status === "disabled") ? (
        <Action size="xs" onPress={() => change({ status: "active" })}>
          Turn on
        </Action>
      ) : null}
      {canManage && status === "active" && !enforced ? (
        <Action size="xs" onPress={() => onConfirm({ kind: "enforce", connection })}>
          Require
        </Action>
      ) : null}
      {canManage && enforced ? (
        <Action size="xs" onPress={() => change({ enforced: false })}>
          Stop requiring
        </Action>
      ) : null}
      {canManage && status === "active" ? (
        <Action size="xs" onPress={() => change({ status: "disabled" })}>
          Turn off
        </Action>
      ) : null}
      {canManage ? <Danger onPress={() => onConfirm({ kind: "delete", connection })}>Delete</Danger> : null}
    </Item>
  );
}

function ConfirmDialog({ pending, onClose }: { pending: Pending | null; onClose: () => void }) {
  const { send, failure, prompt } = useSSOChange();
  const name = pending?.connection.name ?? "";
  const enforce = pending?.kind === "enforce";

  return (
    <>
      <AlertDialog
        isOpen={pending !== null}
        onOpenChange={(open) => (open ? null : onClose())}
        variant={enforce ? "default" : "danger"}
        title={enforce ? `Require ${name}` : `Delete ${name}`}
        description={
          failure ??
          (enforce
            ? "Everyone at the company's domains will sign in through it. Confirm an owner signed in through it today."
            : "People who sign in through it will need another way in. This cannot be undone.")
        }
        confirmLabel={enforce ? "Require it" : "Delete connection"}
        onConfirm={async () => {
          const target = pending?.connection.id;
          if (!target) {
            return;
          }

          await send(
            (workspace) =>
              enforce
                ? updateConnection(workspace, target, { enforced: true, confirm_owners_can_sign_in: true })
                : deleteConnection(workspace, target),
            true,
          );
        }}
      />
      {prompt}
    </>
  );
}

export function ConnectionsSection({ canManage }: { canManage: boolean }) {
  const { send, failure, prompt, workspace } = useSSOChange();
  const { data: connections } = useQuery(connectionsQuery(workspace));
  const [creating, setCreating] = useState(false);
  const [showing, setShowing] = useState<SSOConnection | null>(null);
  const [confirming, setConfirming] = useState<Pending | null>(null);

  return (
    <Section
      title="Connections"
      description="Your identity providers. A connection signs people in once it is tested and turned on."
      action={canManage ? <Action onPress={() => setCreating(true)}>New connection</Action> : undefined}
    >
      {connections ? null : <RowsSkeleton rows={1} />}
      {connections?.length === 0 ? <Empty>No connections yet.</Empty> : null}
      {connections?.map((connection) => (
        <ConnectionItem
          key={connection.id}
          connection={connection}
          canManage={canManage}
          onShow={() => setShowing(connection)}
          onConfirm={setConfirming}
          change={(change) => void send((id) => updateConnection(id, connection.id, change))}
        />
      ))}
      {failure ? (
        <p role="alert" className="py-2.5 pe-3 text-body-2-regular text-text-error-primary">
          {failure}
        </p>
      ) : null}
      <ConnectionDialog isOpen={creating} onClose={() => setCreating(false)} />
      <ConnectionDetails connection={showing} onClose={() => setShowing(null)} />
      <ConfirmDialog pending={confirming} onClose={() => setConfirming(null)} />
      {prompt}
    </Section>
  );
}
