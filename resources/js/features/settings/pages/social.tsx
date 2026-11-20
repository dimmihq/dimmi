/*
 * A workspace's Facebook Pages or Instagram accounts, one page for each
 * platform: a row for each connected account with the inbox it feeds,
 * opening the account's own page. Connecting opens Meta's sign-in, where
 * the person picks the business and the accounts to share; the accounts
 * Meta shared are then listed to pick the one to connect, with the inbox
 * its conversations land in.
 */
import { RiArrowLeftLine, RiArrowRightSLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useBlocker, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { HintText } from "@/components/base/input/hint-text";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { inboxesQuery } from "@/lib/api/inboxes";
import {
  connectSocial,
  disconnectSocial,
  moveSocial,
  type SocialChannel,
  type SocialGrant,
  type SocialPlatform,
  socialChannelsQuery,
  socialSignupConfigQuery,
  startSocialSignup,
} from "@/lib/api/social";

import { Danger } from "../controls";
import { usePermission } from "../inboxes";
import { Empty, Item, Logo, PageSkeleton, Row, Section, SettingsPage } from "../layout";
import { rowLink } from "../row-link";
import { SaveBar } from "../save-bar";
import { handleOf, socialLook } from "../social-look";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import { openSignup, SignupClosed, takeFinishedSignup } from "../whatsapp-connect";

const listRoute = {
  facebook: "/d/settings/integrations/facebook",
  instagram: "/d/settings/integrations/instagram",
} as const;

const detailRoute = {
  facebook: "/d/settings/integrations/facebook/$channel",
  instagram: "/d/settings/integrations/instagram/$channel",
} as const;

function useCanManage(workspace: string): boolean {
  const canChannels = usePermission(workspace, "channels:manage");
  const canWrite = usePermission(workspace, "settings:write");

  return canChannels && canWrite;
}

export function SocialSettings({ platform }: { platform: SocialPlatform }) {
  const workspace = useAccount().workspace.id;
  const look = socialLook[platform];
  const { data: channels } = useQuery(socialChannelsQuery(workspace, platform));
  const { data: inboxes = [] } = useQuery(inboxesQuery(workspace));
  const canManage = useCanManage(workspace);
  const inboxName = (id: string) => inboxes.find((i) => i.id === id)?.name ?? "Inbox";

  return (
    <SettingsPage
      title={look.name}
      mark={<Logo icon={look.icon} brand={look.brand} />}
      description={look.description}
      actions={canManage ? <ConnectSocial workspace={workspace} platform={platform} /> : null}
    >
      <BackTo to="integrations" />
      <Section title={look.accounts} loading={!channels} rows={2}>
        {channels?.length === 0 ? <Empty>{look.empty}</Empty> : null}
        {channels?.map((channel) => (
          <Item
            key={channel.id}
            mark={<AccountMark platform={platform} account={channel} />}
            title={
              <Link to={detailRoute[platform]} params={{ channel: channel.id }} className={rowLink}>
                {channel.name}
              </Link>
            }
            detail={[handleOf(channel), `${inboxName(channel.inbox_id)} inbox`].filter(Boolean).join(" · ")}
          >
            <StatusChip channel={channel} />
            <RiArrowRightSLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
          </Item>
        ))}
      </Section>
    </SettingsPage>
  );
}

function StatusChip({ channel }: { channel: SocialChannel }) {
  return channel.status === "active" ? (
    <Chip variant="caption" color="lime">
      Connected
    </Chip>
  ) : (
    <Chip variant="caption" color="rose">
      Needs reconnecting
    </Chip>
  );
}

function BackTo({ to, platform }: { to: "integrations" | "list"; platform?: SocialPlatform }) {
  const navigate = useNavigate();
  const list = to === "list" && platform;

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() =>
          list
            ? navigate({ to: listRoute[platform] })
            : navigate({ to: "/d/settings/$section", params: { section: "integrations" } })
        }
      >
        {list ? `All ${socialLook[platform].accounts.toLowerCase()}` : "All integrations"}
      </Button>
    </div>
  );
}

/*
 * ConnectSocial opens Meta's sign-in, then the picker of the accounts
 * Meta shared.
 */
function ConnectSocial({ workspace, platform }: { workspace: string; platform: SocialPlatform }) {
  const look = socialLook[platform];
  const { data: config } = useQuery(socialSignupConfigQuery(workspace, platform));
  const [grant, setGrant] = useState<SocialGrant | null>(null);

  const signIn = useMutation({
    mutationFn: async () => {
      if (!config?.available) {
        throw new Error("unavailable");
      }
      const result = await openSignup(config, true);

      return startSocialSignup(workspace, platform, { code: result.code, redirect_uri: result.redirect_uri });
    },
    onSuccess: setGrant,
    onError: (error) => {
      if (error instanceof SignupClosed && !error.blocked) {
        return;
      }
      toast.error(`${look.name} was not connected`, { description: failureText(error) });
    },
  });

  const resume = useMutation({
    mutationFn: (result: { code: string; redirect_uri: string }) => startSocialSignup(workspace, platform, result),
    onSuccess: setGrant,
    onError: (error) => toast.error(`${look.name} was not connected`, { description: failureText(error) }),
  });
  const resumeSignup = resume.mutate;

  useEffect(() => {
    const finished = takeFinishedSignup();
    if (finished) {
      resumeSignup({ code: finished.code, redirect_uri: finished.redirect_uri });
    }
  }, [resumeSignup]);

  if (!config?.available) {
    return null;
  }

  return (
    <>
      <Button
        size="small"
        leadingIcon={look.icon}
        loading={signIn.isPending || resume.isPending}
        onClick={() => signIn.mutate()}
      >
        Connect {platform === "facebook" ? "a Page" : "an account"}
      </Button>
      {grant ? (
        <PickAccount workspace={workspace} platform={platform} grant={grant} onClose={() => setGrant(null)} />
      ) : null}
    </>
  );
}

/*
 * PickAccount lists the accounts a sign-in shared and connects the one
 * picked to an inbox.
 */
function PickAccount({
  workspace,
  platform,
  grant,
  onClose,
}: {
  workspace: string;
  platform: SocialPlatform;
  grant: SocialGrant;
  onClose: () => void;
}) {
  const look = socialLook[platform];
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { data: inboxes = [] } = useQuery(inboxesQuery(workspace));
  const [picked, setPicked] = useState(grant.accounts.find((a) => !a.connected)?.id ?? "");
  const [inbox, setInbox] = useState<string | null>(null);

  const connect = useMutation({
    mutationFn: () =>
      connectSocial(workspace, platform, { grant: grant.grant, account_id: picked, inbox_id: inbox ?? undefined }),
    onSuccess: async (channel) => {
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "social", platform] });
      toast.success(`${channel.name} is connected`, { description: "Its messages now land in the inbox." });
      onClose();
      await navigate({ to: detailRoute[platform], params: { channel: channel.id } });
    },
  });

  return (
    <Modal isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={`Pick the ${look.account} to connect`}
        description={
          grant.accounts.length === 0
            ? `Meta shared no ${look.accounts.toLowerCase()}. Connect again and select at least one.`
            : "These are the ones you shared with dimmi at Meta."
        }
      />
      <ModalBody className="mt-5 flex flex-col gap-4">
        <RadioGroup aria-label={look.accounts} value={picked} onChange={setPicked} className="flex flex-col gap-2">
          {grant.accounts.map((account) => (
            <RadioCard
              key={account.id}
              value={account.id}
              isDisabled={account.connected}
              mark={<AccountMark platform={platform} account={account} />}
              title={account.name || handleOf(account)}
              description={
                account.connected ? "Already connected" : handleOf(account) || `${look.name} ${look.account}`
              }
            />
          ))}
        </RadioGroup>
        {grant.accounts.length > 0 ? (
          <div className="flex flex-col gap-1.5">
            <Select
              label="Inbox"
              selectedKey={inbox ?? inboxes.find((i) => i.default)?.id ?? inboxes[0]?.id ?? null}
              onSelectionChange={(key) => setInbox(String(key))}
              className="w-full"
            >
              {inboxes.map((i) => (
                <SelectItem key={i.id} id={i.id} textValue={i.name}>
                  {i.name}
                </SelectItem>
              ))}
            </Select>
            <HintText>Where its conversations land. You can change this later.</HintText>
          </div>
        ) : null}
        {connect.error ? (
          <p role="alert" className="text-body-2-regular text-status-rose-text">
            {failureText(connect.error)}
          </p>
        ) : null}
      </ModalBody>
      <ModalFooter className="mt-6">
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button disabled={picked === ""} loading={connect.isPending} onClick={() => connect.mutate()}>
          Connect
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * AccountMark is an account's own picture with its platform's logo on
 * the corner, or the logo alone when Meta gave no picture.
 */
function AccountMark({
  platform,
  account,
}: {
  platform: SocialPlatform;
  account: { name: string; username: string; picture_url: string };
}) {
  const look = socialLook[platform];
  if (!account.picture_url) {
    return <Logo icon={look.icon} brand={look.brand} />;
  }

  return (
    <span className="relative flex">
      <Avatar size="lg" src={account.picture_url} alt="" initials={(account.name || account.username).charAt(0)} />
      <span
        aria-hidden
        className="absolute -end-1 -bottom-1 flex size-4 items-center justify-center rounded-full ring-2 ring-background-primary-default"
        style={{ backgroundColor: look.brand }}
      >
        <look.icon className="size-2.5 text-text-white" />
      </span>
    </span>
  );
}

/*
 * SocialDetail is one connected account: the inbox it feeds, and
 * disconnecting it.
 */
export function SocialDetail({ platform, channel: id }: { platform: SocialPlatform; channel: string }) {
  const workspace = useAccount().workspace.id;
  const look = socialLook[platform];
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const canManage = useCanManage(workspace);
  const { data: channels, isPending } = useQuery(socialChannelsQuery(workspace, platform));
  const { data: inboxes = [] } = useQuery(inboxesQuery(workspace));
  const [inbox, setInbox] = useState<string | null>(null);
  const [disconnecting, setDisconnecting] = useState(false);
  const channel = channels?.find((c) => c.id === id);
  const dirty = inbox !== null && inbox !== channel?.inbox_id;

  const save = useMutation({
    mutationFn: () => moveSocial(workspace, platform, id, inbox ?? ""),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "social", platform] });
      setInbox(null);
      toast.success("Inbox saved", { description: "New conversations land there." });
    },
  });

  useBlocker({
    shouldBlockFn: () => dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => dirty,
  });

  if (isPending) {
    return <PageSkeleton />;
  }

  if (!channel) {
    return (
      <SettingsPage title="Not found">
        <BackTo to="list" platform={platform} />
        <Section title={look.accounts}>
          <Empty>This {look.account} is not connected to the workspace.</Empty>
        </Section>
      </SettingsPage>
    );
  }

  return (
    <SettingsPage
      title={channel.name}
      description={handleOf(channel) || look.name}
      mark={<Logo icon={look.icon} brand={look.brand} />}
      footer={
        <SaveBar
          dirty={dirty}
          saving={save.isPending}
          error={save.error ? failureText(save.error) : null}
          onSave={() => save.mutate()}
          onDiscard={() => setInbox(null)}
        />
      }
    >
      <BackTo to="list" platform={platform} />
      <Section title="Connection">
        <Row label="Status" description={channel.last_error ?? "Meta delivers this account's messages to dimmi."}>
          <StatusChip channel={channel} />
        </Row>
        <Row
          label="Replying"
          description="Anyone, the AI agent included, can reply for 24 hours after a customer's last message. Teammates can reply for 7 days."
        >
          <span className="text-body-regular text-text-primary">Meta's rule</span>
        </Row>
      </Section>
      <Section title="Inbox" description="Where its conversations land. Conversations already in an inbox stay there.">
        <Row label="New conversations land in">
          <Select
            aria-label="Inbox"
            size="sm"
            selectedKey={inbox ?? channel.inbox_id}
            onSelectionChange={(key) => setInbox(String(key))}
            isDisabled={!canManage}
            popoverClassName="w-56"
          >
            {inboxes.map((i) => (
              <SelectItem key={i.id} id={i.id} textValue={i.name}>
                {i.name}
              </SelectItem>
            ))}
          </Select>
        </Row>
      </Section>
      {canManage ? (
        <Section title="Disconnect">
          <Row
            label={`Disconnect this ${look.account}`}
            description="Its messages stop reaching the inbox. Its conversations stay, and you can connect it again."
          >
            <Danger onPress={() => setDisconnecting(true)}>Disconnect</Danger>
          </Row>
        </Section>
      ) : null}
      <AlertDialog
        isOpen={disconnecting}
        onOpenChange={setDisconnecting}
        title={`Disconnect ${channel.name}?`}
        description="New messages to it stop reaching dimmi. Conversations stay where they are."
        confirmLabel="Disconnect"
        pendingLabel="Disconnecting…"
        onConfirm={async () => {
          await disconnectSocial(workspace, platform, channel.id);
          await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "social", platform] });
          toast.success(`${channel.name} is disconnected`);
          await navigate({ to: listRoute[platform] });
        }}
      />
    </SettingsPage>
  );
}
