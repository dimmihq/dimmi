/*
 * The workspace's WhatsApp numbers: one row each with the name customers
 * see, the number and the inbox it feeds, opening the number's own page.
 * Connecting opens Meta's signup, where the person picks or creates the
 * business account and number, a number on the WhatsApp Business app
 * included; Meta bills the business directly for its messages. A server
 * without its own Meta app connects a number with a token of the
 * workspace's own Meta app instead, read with it before one is picked.
 */
import { RiArrowLeftLine, RiArrowRightSLine, RiWhatsappLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import { inboxesQuery } from "@/lib/api/inboxes";
import {
  connectWhatsApp,
  finishWhatsAppSignup,
  readWhatsAppAccount,
  type WhatsAppAccount,
  type WhatsAppChannel,
  whatsappChannelsQuery,
  whatsappSignupConfigQuery,
} from "@/lib/api/whatsapp";

import { usePermission } from "../inboxes";
import { Empty, Item, Logo, Section, SettingsPage } from "../layout";
import { rowLink } from "../row-link";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import { openSignup, SignupClosed } from "../whatsapp-connect";
import { qualityLook, whatsappGreen } from "../whatsapp-look";

export function WhatsAppSettings() {
  const workspace = useAccount().workspace.id;
  const { data: channels } = useQuery(whatsappChannelsQuery(workspace));
  const { data: inboxes = [] } = useQuery(inboxesQuery(workspace));
  const canChannels = usePermission(workspace, "channels:manage");
  const canWrite = usePermission(workspace, "settings:write");
  const canManage = canChannels && canWrite;
  const inboxName = (id: string) => inboxes.find((i) => i.id === id)?.name ?? "Inbox";

  return (
    <SettingsPage
      title="WhatsApp"
      mark={<Logo icon={RiWhatsappLine} brand={whatsappGreen} />}
      description="Customers write to your business number and the conversation lands in the inbox. Meta bills your business directly for messages."
      actions={canManage ? <ConnectWhatsApp workspace={workspace} /> : null}
    >
      <BackToIntegrations />
      <Section title="Numbers" loading={!channels} rows={2}>
        {channels?.length === 0 ? <Empty>No WhatsApp number yet. Connect one to answer customers there.</Empty> : null}
        {channels?.map((channel) => (
          <NumberRow key={channel.id} channel={channel} inbox={inboxName(channel.inbox_id)} />
        ))}
      </Section>
    </SettingsPage>
  );
}

function BackToIntegrations() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "integrations" } })}
      >
        All integrations
      </Button>
    </div>
  );
}

function NumberRow({ channel, inbox }: { channel: WhatsAppChannel; inbox: string }) {
  const quality = qualityLook(channel.quality_rating);

  return (
    <Item
      mark={<Logo icon={RiWhatsappLine} brand={whatsappGreen} />}
      title={
        <Link to="/d/settings/integrations/whatsapp/$channel" params={{ channel: channel.id }} className={rowLink}>
          {channel.verified_name || channel.display_phone_number}
        </Link>
      }
      detail={`${channel.display_phone_number} · ${inbox} inbox`}
    >
      <Chip variant="caption" color={quality.color}>
        {quality.label}
      </Chip>
      <RiArrowRightSLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
    </Item>
  );
}

/*
 * ConnectWhatsApp opens Meta's signup when this server has its own Meta
 * app, and the own-app dialog otherwise.
 */
export function ConnectWhatsApp({ workspace }: { workspace: string }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { data: config } = useQuery(whatsappSignupConfigQuery(workspace));
  const [ownApp, setOwnApp] = useState(false);

  const connected = async (channel: WhatsAppChannel) => {
    await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "whatsapp"] });
    toast.success(`${channel.verified_name || channel.display_phone_number} is connected`, {
      description: "Messages to this number now land in the inbox.",
    });
    await navigate({ to: "/d/settings/integrations/whatsapp/$channel", params: { channel: channel.id } });
  };

  const signup = useMutation({
    mutationFn: async () => {
      if (!config?.available) {
        throw new Error("unavailable");
      }
      const result = await openSignup(config);
      return finishWhatsAppSignup(workspace, result);
    },
    onSuccess: connected,
    onError: (error) => {
      if (error instanceof SignupClosed && !error.blocked) {
        return;
      }
      toast.error("WhatsApp was not connected", { description: failureText(error) });
    },
  });

  if (!config) {
    return null;
  }

  if (!config.available) {
    return (
      <>
        <Button size="small" leadingIcon={RiWhatsappLine} onClick={() => setOwnApp(true)}>
          Connect WhatsApp
        </Button>
        {ownApp ? (
          <OwnAppDialog workspace={workspace} onClose={() => setOwnApp(false)} onConnected={connected} />
        ) : null}
      </>
    );
  }

  return (
    <Button size="small" leadingIcon={RiWhatsappLine} loading={signup.isPending} onClick={() => signup.mutate()}>
      Connect WhatsApp
    </Button>
  );
}

const settle = [0.22, 1, 0.36, 1] as const;

/*
 * OwnAppDialog connects a number with a token of the workspace's own Meta
 * app: the token, account and secret first, then a number picked from
 * those Meta lists on the account.
 */
function OwnAppDialog({
  workspace,
  onClose,
  onConnected,
}: {
  workspace: string;
  onClose: () => void;
  onConnected: (channel: WhatsAppChannel) => Promise<void>;
}) {
  const still = useReducedMotion();
  const [token, setToken] = useState("");
  const [waba, setWaba] = useState("");
  const [secret, setSecret] = useState("");
  const [account, setAccount] = useState<WhatsAppAccount | null>(null);
  const [picked, setPicked] = useState<string | null>(null);
  const credentials = { access_token: token.trim(), waba_id: waba.trim(), app_secret: secret.trim() || undefined };
  const ready = credentials.access_token !== "" && credentials.waba_id !== "";
  const heading = dialogHeading(account);

  const read = useMutation({
    mutationFn: () => readWhatsAppAccount(workspace, credentials),
    onSuccess: (found) => {
      setAccount(found);
      setPicked(found.numbers.find((n) => !n.connected)?.id ?? null);
    },
  });
  const connect = useMutation({
    mutationFn: () => connectWhatsApp(workspace, { ...credentials, phone_number_id: picked ?? "" }),
    onSuccess: async (channel) => {
      onClose();
      await onConnected(channel);
    },
  });
  const error = read.error ?? connect.error;

  return (
    <Modal isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title={heading.title} description={heading.description} />
      <ModalBody className="mt-5">
        <AnimatePresence initial={false} mode="wait">
          <motion.div
            key={account ? "numbers" : "token"}
            initial={still ? false : { opacity: 0, x: account ? 16 : -16, filter: "blur(4px)" }}
            animate={{ opacity: 1, x: 0, filter: "blur(0px)" }}
            exit={still ? { opacity: 0 } : { opacity: 0, x: account ? -16 : 16, filter: "blur(4px)" }}
            transition={{ duration: 0.24, ease: settle }}
            className="flex flex-col gap-4"
          >
            {account ? (
              <NumberStep account={account} picked={picked} onPick={setPicked} />
            ) : (
              <TokenStep
                token={token}
                waba={waba}
                secret={secret}
                onToken={setToken}
                onWaba={setWaba}
                onSecret={setSecret}
              />
            )}
          </motion.div>
        </AnimatePresence>
        {error ? (
          <p role="alert" className="mt-3 text-body-2-regular text-text-error-primary">
            {failureText(error)}
          </p>
        ) : null}
      </ModalBody>
      <ModalFooter className="mt-6">
        {account ? (
          <>
            <Button variant="secondary" onClick={() => setAccount(null)}>
              Back
            </Button>
            <Button loading={connect.isPending} disabled={!picked} onClick={() => connect.mutate()}>
              Connect number
            </Button>
          </>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button loading={read.isPending} disabled={!ready} onClick={() => read.mutate()}>
              Find numbers
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}

/*
 * dialogHeading is the own-app dialog's title and description for its
 * step.
 */
function dialogHeading(account: WhatsAppAccount | null): { title: string; description: string } {
  if (!account) {
    return {
      title: "Connect your Meta app",
      description: "A system user's token of your Meta app, with the WhatsApp permissions, and the app's secret.",
    };
  }

  const owner = account.business ? `, owned by ${account.business}` : "";

  return { title: "Pick a number", description: `Numbers on ${account.name}${owner}.` };
}

function NumberStep({
  account,
  picked,
  onPick,
}: {
  account: WhatsAppAccount;
  picked: string | null;
  onPick: (id: string) => void;
}) {
  return (
    <RadioGroup aria-label="Number" value={picked ?? ""} onChange={onPick} className="flex flex-col gap-2">
      {account.numbers.map((n) => (
        <RadioCard
          key={n.id}
          value={n.id}
          isDisabled={n.connected}
          icon={RiWhatsappLine}
          title={n.verified_name || n.display_phone_number}
          description={n.connected ? `${n.display_phone_number} · already connected` : n.display_phone_number}
        />
      ))}
    </RadioGroup>
  );
}

function TokenStep({
  token,
  waba,
  secret,
  onToken,
  onWaba,
  onSecret,
}: {
  token: string;
  waba: string;
  secret: string;
  onToken: (value: string) => void;
  onWaba: (value: string) => void;
  onSecret: (value: string) => void;
}) {
  return (
    <>
      <Input label="Access token" type="password" value={token} onChange={onToken} autoFocus />
      <Input label="WhatsApp account ID" value={waba} onChange={onWaba} />
      <Input label="App secret" type="password" value={secret} onChange={onSecret} />
    </>
  );
}
