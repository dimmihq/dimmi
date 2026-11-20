/*
 * One WhatsApp number: how Meta sees it (its name, quality and how many
 * customers a day it may start writing to), the inbox its conversations
 * land in, the templates approved for it, the webhook a workspace's own
 * Meta app is set up with, and disconnecting it. Moving it to another
 * inbox waits in a draft, saved from the bar at the foot; leaving with it
 * unsaved asks first. Templates are written here or in Meta's WhatsApp
 * Manager and approved by Meta; their copy here can be taken again at
 * any time.
 */
import {
  RiAddLine,
  RiArrowLeftLine,
  RiArrowRightSLine,
  RiExternalLinkLine,
  RiFileTextLine,
  RiRefreshLine,
  RiWhatsappLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Focusable } from "react-aria-components";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";
import { inboxesQuery } from "@/lib/api/inboxes";
import {
  disconnectWhatsApp,
  moveWhatsApp,
  refreshWhatsAppTemplates,
  type WhatsAppChannel,
  type WhatsAppTemplate,
  whatsappChannelsQuery,
  whatsappTemplatesQuery,
} from "@/lib/api/whatsapp";

import { Danger } from "../controls";
import { usePermission } from "../inboxes";
import { Empty, Glyph, Item, Logo, PageSkeleton, Row, Section, SettingsPage } from "../layout";
import { rowLink } from "../row-link";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import { qualityLook, whatsappGreen } from "../whatsapp-look";

const limits: Record<string, string> = {
  TIER_250: "250 new customers a day",
  TIER_1K: "1,000 new customers a day",
  TIER_2K: "2,000 new customers a day",
  TIER_10K: "10,000 new customers a day",
  TIER_100K: "100,000 new customers a day",
  TIER_UNLIMITED: "No daily limit",
};

const nameStatuses: Record<string, { label: string; color: "lime" | "yellow" | "rose" | "neutral" }> = {
  APPROVED: { label: "Approved", color: "lime" },
  AVAILABLE_WITHOUT_REVIEW: { label: "In use", color: "lime" },
  PENDING_REVIEW: { label: "In review", color: "yellow" },
  DECLINED: { label: "Declined", color: "rose" },
};

/*
 * templateDetail is a template's language, category and variables, or
 * why Meta rejected it.
 */
function templateDetail(t: WhatsAppTemplate): string {
  if (t.status === "rejected" && t.rejected_reason) {
    return `Rejected: ${t.rejected_reason.toLowerCase().replaceAll("_", " ")}`;
  }
  const variables = t.variables.length > 0 ? ` · ${t.variables.length} variables` : "";
  return `${t.language} · ${t.category}${variables}`;
}

/*
 * templateLook names a template's status as the list shows it.
 */
function templateLook(t: WhatsAppTemplate): { label: string; color: "lime" | "yellow" | "rose" | "neutral" } {
  switch (t.status) {
    case "approved":
      return t.sendable
        ? { label: "Approved", color: "lime" }
        : { label: "Approved, not sendable here", color: "neutral" };
    case "pending":
    case "in_appeal":
      return { label: "In review", color: "yellow" };
    case "rejected":
    case "disabled":
    case "paused":
      return {
        label: t.status === "paused" ? "Paused" : t.status === "disabled" ? "Disabled" : "Rejected",
        color: "rose",
      };
    default:
      return { label: t.status, color: "neutral" };
  }
}

/*
 * connectedThrough says how the number was connected.
 */
function connectedThrough(channel: WhatsAppChannel): string {
  if (channel.mode === "own") {
    return "Your own Meta app";
  }

  return channel.coexistence ? "Meta signup, shared with the WhatsApp Business app" : "Meta signup";
}

function BackToNumbers() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/integrations/whatsapp" })}
      >
        All numbers
      </Button>
    </div>
  );
}

export function WhatsAppNumberPage({ channel: id }: { channel: string }) {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { data: channels, isPending } = useQuery(whatsappChannelsQuery(workspace));
  const { data: inboxes = [] } = useQuery(inboxesQuery(workspace));
  const canChannels = usePermission(workspace, "channels:manage");
  const canWrite = usePermission(workspace, "settings:write");
  const canManage = canChannels && canWrite;
  const channel = channels?.find((c) => c.id === id);
  const [inbox, setInbox] = useState<string | null>(null);
  const [disconnecting, setDisconnecting] = useState(false);
  const dirty = channel !== undefined && inbox !== null && inbox !== channel.inbox_id;

  const save = useMutation({
    mutationFn: () => moveWhatsApp(workspace, id, inbox ?? ""),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "whatsapp"] });
      setInbox(null);
      toast.success("Inbox saved", { description: "New conversations on this number land there." });
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
      <SettingsPage title="Number not found">
        <BackToNumbers />
        <Section title="Number">
          <Empty>This WhatsApp number is not connected to the workspace.</Empty>
        </Section>
      </SettingsPage>
    );
  }

  const quality = qualityLook(channel.quality_rating);
  const name = nameStatuses[channel.name_status] ?? { label: channel.name_status || "Not set", color: "neutral" };

  return (
    <SettingsPage
      title={channel.verified_name || channel.display_phone_number}
      description={channel.display_phone_number}
      mark={<Logo icon={RiWhatsappLine} brand={whatsappGreen} />}
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
      <BackToNumbers />
      <Section title="Number" description="How Meta sees this number. It updates as Meta reviews it.">
        <Row label="Phone number" description="The number customers write to.">
          <span className="text-body-regular text-text-primary">{channel.display_phone_number}</span>
        </Row>
        <Row label="Display name" description={channel.verified_name || "Shown to customers once Meta approves it."}>
          <Chip variant="caption" color={name.color}>
            {name.label}
          </Chip>
        </Row>
        <Row label="Quality" description="Meta lowers a number's limits when customers block or report it.">
          <Chip variant="caption" color={quality.color}>
            {quality.label}
          </Chip>
        </Row>
        <Row
          label="Daily limit"
          description="How many new customers a day the business may start writing to with templates."
        >
          <span className="text-body-regular text-text-primary">
            {limits[channel.messaging_limit] ?? "Set by Meta after review"}
          </span>
        </Row>
        <Row label="Connected through" description="Meta bills your business directly for messages.">
          <span className="text-body-regular text-text-primary">{connectedThrough(channel)}</span>
        </Row>
      </Section>
      <Section
        title="Inbox"
        description="Where conversations on this number land. Conversations already in an inbox stay there."
      >
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
      <TemplatesSection workspace={workspace} channel={channel.id} waba={channel.waba_id} />
      {channel.webhook ? (
        <Section title="Webhook" description="Paste these into your Meta app's WhatsApp webhook settings.">
          <Row label="Callback URL">
            <code className="text-body-2-regular text-text-primary">{channel.webhook.callback_url}</code>
          </Row>
          <Row label="Verify token">
            <code className="text-body-2-regular text-text-primary">{channel.webhook.verify_token}</code>
          </Row>
        </Section>
      ) : null}
      {canManage ? (
        <Section title="Disconnect">
          <Row
            label="Disconnect this number"
            description="Messages to it stop reaching the inbox. Its conversations stay, and you can connect it again."
          >
            <Danger onPress={() => setDisconnecting(true)}>Disconnect</Danger>
          </Row>
        </Section>
      ) : null}
      <AlertDialog
        isOpen={disconnecting}
        onOpenChange={setDisconnecting}
        title={`Disconnect ${channel.display_phone_number}?`}
        description="Customers' new messages to this number stop reaching dimmi. Conversations stay where they are."
        confirmLabel="Disconnect"
        pendingLabel="Disconnecting…"
        onConfirm={async () => {
          await disconnectWhatsApp(workspace, channel.id);
          await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "whatsapp"] });
          toast.success(`${channel.display_phone_number} is disconnected`);
          await navigate({ to: "/d/settings/integrations/whatsapp" });
        }}
      />
    </SettingsPage>
  );
}

function TemplatesSection({ workspace, channel, waba }: { workspace: string; channel: string; waba: string }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const canManage = usePermission(workspace, "channels:manage");
  const open = (template: string) =>
    navigate({
      to: "/d/settings/integrations/whatsapp/$channel/templates/$template",
      params: { channel, template },
    });
  const query = whatsappTemplatesQuery(workspace, channel);
  const { data: templates } = useQuery(query);
  const refresh = useMutation({
    mutationFn: () => refreshWhatsAppTemplates(workspace, channel),
    onSuccess: (fresh) => {
      queryClient.setQueryData(query.queryKey, fresh);
      toast.success("Templates updated", { description: `${fresh.length} templates from WhatsApp Manager.` });
    },
    onError: (error) => toast.error("Templates were not updated", { description: failureText(error) }),
  });

  return (
    <Section
      title="Templates"
      description="Only approved templates can be sent more than 24 hours after a customer's last message."
      action={
        <div className="flex items-center gap-3">
          <LinkButton
            variant="secondary"
            size="small"
            trailingIcon={RiExternalLinkLine}
            href={`https://business.facebook.com/wa/manage/message-templates/?waba_id=${encodeURIComponent(waba)}`}
            target="_blank"
            rel="noreferrer"
          >
            WhatsApp Manager
          </LinkButton>
          <TooltipTrigger delay={200}>
            <Focusable>
              <IconButton
                variant="quiet"
                size="small"
                icon={RiRefreshLine}
                aria-label="Refresh from WhatsApp Manager"
                disabled={refresh.isPending}
                className={refresh.isPending ? "[&_svg]:animate-spin" : undefined}
                onClick={() => refresh.mutate()}
              />
            </Focusable>
            <Tooltip>Refresh from WhatsApp Manager</Tooltip>
          </TooltipTrigger>
          {canManage ? (
            <Button variant="secondary" size="xs" leadingIcon={RiAddLine} onClick={() => open("new")}>
              New template
            </Button>
          ) : null}
        </div>
      }
      loading={!templates}
      rows={3}
    >
      {templates?.length === 0 ? <Empty>No templates yet. Write one to message customers after 24 hours.</Empty> : null}
      {templates?.map((t) => {
        const look = templateLook(t);
        const opens = canManage && t.content.editable;

        return (
          <Item
            key={t.id}
            mark={<Glyph icon={RiFileTextLine} />}
            title={
              opens ? (
                <Link
                  to="/d/settings/integrations/whatsapp/$channel/templates/$template"
                  params={{ channel, template: t.id }}
                  className={rowLink}
                >
                  {t.name}
                </Link>
              ) : (
                t.name
              )
            }
            detail={templateDetail(t)}
          >
            <Chip variant="caption" color={look.color}>
              {look.label}
            </Chip>
            {opens ? <RiArrowRightSLine aria-hidden className="size-4 text-foreground-icon-tertiary" /> : null}
          </Item>
        );
      })}
    </Section>
  );
}
