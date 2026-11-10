/*
 * A draft broadcast: the composer, saved as it is written, with a test
 * and sending or scheduling in the header. Sending always asks first,
 * naming how many it will reach, and saves whatever is still waiting
 * before it goes. A refused send opens the review step with the
 * server's reason on top. An email draft without an address to send
 * from starts on the workspace's default one.
 */
import { RiDeleteBinLine, RiSendPlaneLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { type Broadcast, broadcastEstimateQuery, testBroadcast } from "@/lib/api/broadcasts";
import { ApiError } from "@/lib/api/client";
import { formatNumber } from "@/lib/i18n/format";

import { BroadcastComposer, Stepper } from "./broadcast-composer";
import { Pick } from "./controls";
import { checksOf, type Draft, readyToSend, type StepId, steps, testable } from "./draft";
import { channelLook, reachNoun, roomName } from "./looks";
import { Frame } from "./page-frame";
import { useCustomerRooms, useEmailAddresses, useNames } from "./sources";
import { useBroadcastActions } from "./use-actions";
import { useDraft } from "./use-draft";

/*
 * useDefaultAddress starts an email draft on the workspace's default
 * address, once, when it has none.
 */
function useDefaultAddress(workspace: string, broadcast: Broadcast, draft: Draft, change: (c: Partial<Draft>) => void) {
  const email = broadcast.channel === "email";
  const addresses = useEmailAddresses(workspace, email);
  const done = useRef(false);

  useEffect(() => {
    if (done.current || !email || draft.email_address_id !== null || addresses.length === 0) {
      return;
    }
    done.current = true;
    const chosen = addresses.find((address) => address.is_default) ?? addresses[0];
    if (chosen) {
      change({ email_address_id: chosen.id });
    }
  }, [email, draft.email_address_id, addresses, change]);
}

function TestSend({
  workspace,
  broadcast,
  isOpen,
  onOpenChange,
  onSend,
}: {
  workspace: string;
  broadcast: Broadcast;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onSend: (to: { email: string } | { room_id: string }) => Promise<void>;
}) {
  const me = useAccount().email;
  const slack = broadcast.channel === "slack";
  const { rooms } = useCustomerRooms(workspace, slack && isOpen);
  const [email, setEmail] = useState(me);
  const [room, setRoom] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const to = slack ? (room ? { room_id: room } : null) : email.trim() ? { email: email.trim() } : null;

  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange} size="sm">
      {({ close }) => (
        <>
          <ModalHeader
            title="Send a test"
            description={
              slack
                ? "Posts it once into one channel."
                : "Details are filled in from your own profile where they can be."
            }
          />
          <ModalBody>
            {slack ? (
              <Pick
                label="Channel"
                options={rooms.map((one) => ({ id: one.id, name: roomName(one.name) }))}
                value={room}
                placeholder="Choose a channel"
                onChange={setRoom}
              />
            ) : (
              <Input label="Send to" inputDir="ltr" value={email} onChange={setEmail} autoFocus />
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button
              leadingIcon={RiSendPlaneLine}
              disabled={!to || busy}
              onClick={() => {
                if (!to) {
                  return;
                }
                setBusy(true);
                void onSend(to)
                  .then(close)
                  .finally(() => setBusy(false));
              }}
            >
              Send test
            </Button>
          </ModalFooter>
        </>
      )}
    </Modal>
  );
}

function sendTitle(later: boolean, reach: string): string {
  return later ? `Schedule for ${reach}?` : `Send to ${reach} now?`;
}

export function DraftPage({ workspace, broadcast }: { workspace: string; broadcast: Broadcast }) {
  const { draft, saving, change, flush } = useDraft(workspace, broadcast);
  const { data: estimate } = useQuery(broadcastEstimateQuery(workspace, broadcast.id));
  const { act, remove } = useBroadcastActions(workspace, broadcast.id);
  const names = useNames(workspace, broadcast.channel, draft);
  const brand = useAccount().workspace.name;
  const [tested, setTested] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);
  const [place, setPlace] = useState<{ step: StepId; forward: boolean }>({ step: "setup", forward: true });
  const [testing, setTesting] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [deleting, setDeleting] = useState(false);
  useDefaultAddress(workspace, broadcast, draft, change);

  const checks = checksOf(broadcast.channel, draft, estimate, tested);
  const goTo = (step: StepId) => setPlace((now) => ({ step, forward: steps.indexOf(step) >= steps.indexOf(now.step) }));
  const later = draft.schedule_kind !== "now";
  const reach = `${formatNumber(estimate?.reach ?? 0)} ${reachNoun(broadcast.channel, estimate?.reach ?? 0)}`;

  const sendTest = async (to: { email: string } | { room_id: string }) => {
    await flush();
    try {
      await testBroadcast(workspace, broadcast.id, to);
      setTested(true);
      toast.success("Test sent.");
    } catch (error) {
      toast.error((error as Error).message);
      throw error;
    }
  };

  const send = async () => {
    await flush();
    try {
      await act.mutateAsync("send");
      toast.success(later ? "Scheduled." : "Sending now.");
    } catch (error) {
      if (error instanceof ApiError && error.status === 422) {
        setRefusal(error.message);
        goTo("review");
        return;
      }
      toast.error((error as Error).message);
    }
  };

  return (
    <Frame
      name={draft.name}
      status={broadcast.status}
      note={saving ? "Saving…" : ""}
      steps={<Stepper checks={checks} step={place.step} onStep={goTo} />}
      message={{ ...draft, channel: broadcast.channel, sender: names.sender, brand }}
      writing={place.step === "write"}
      actions={
        <>
          <TestSend
            workspace={workspace}
            broadcast={broadcast}
            isOpen={testing}
            onOpenChange={setTesting}
            onSend={sendTest}
          />
          <AlertDialog
            isOpen={confirming}
            onOpenChange={setConfirming}
            variant="default"
            icon={RiSendPlaneLine}
            title={sendTitle(later, reach)}
            description={`${names.audience}, by ${channelLook[broadcast.channel].name.toLowerCase()}. It can't be unsent.`}
            confirmLabel={later ? "Schedule" : "Send now"}
            onConfirm={send}
          />
          <AlertDialog
            isOpen={deleting}
            onOpenChange={setDeleting}
            title="Delete this draft?"
            description="It is gone for good."
            confirmLabel="Delete"
            onConfirm={() => remove.mutateAsync().catch((error: Error) => void toast.error(error.message))}
          />
          <IconButton size="small" icon={RiDeleteBinLine} aria-label="Delete draft" onClick={() => setDeleting(true)} />
          {testable(broadcast.channel) ? (
            <Button variant="secondary" size="small" onClick={() => setTesting(true)}>
              Send test
            </Button>
          ) : null}
          {place.step === "review" ? (
            <Button size="small" disabled={!readyToSend(checks)} onClick={() => setConfirming(true)}>
              {later ? "Schedule" : "Send now"}
            </Button>
          ) : null}
        </>
      }
    >
      <BroadcastComposer
        workspace={workspace}
        broadcast={broadcast}
        draft={draft}
        names={names}
        estimate={estimate}
        checks={checks}
        refusal={refusal}
        step={place.step}
        forward={place.forward}
        onStep={goTo}
        onChange={(next) => {
          setRefusal(null);
          change(next);
        }}
      />
    </Frame>
  );
}
