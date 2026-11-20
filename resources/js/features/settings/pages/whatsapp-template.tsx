/*
 * Writing a WhatsApp template in dimmi. A new one gets a name and a
 * language once; every template has a kind, a message and, if wanted,
 * buttons. The message is written like a saved reply, in the same
 * editor, with the customer's details picked from the Variable menu; a
 * detail that is not the customer's, such as an order number, is added
 * there as a custom value with an example. The message as the customer
 * reads it shows under the editor. Changes wait in the bar at the foot;
 * submitting sends the template to Meta for review, and what Meta would
 * refuse comes back on its field. A template Meta has can be sent back
 * to review with new content, or deleted.
 */
import { RiAddLine, RiArrowLeftLine, RiCloseLine, RiWhatsappLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { lazy, Suspense, useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { HintText } from "@/components/base/input/hint-text";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { AlertDialog } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { MessageBody } from "@/features/inbox/message-body";
import { ApiError } from "@/lib/api/client";
import {
  createWhatsAppTemplate,
  deleteWhatsAppTemplate,
  updateWhatsAppTemplate,
  type WhatsAppButton,
  type WhatsAppButtonType,
  type WhatsAppTemplate,
  type WhatsAppTemplateDraft,
  whatsappTemplatesQuery,
} from "@/lib/api/whatsapp";

import { Danger } from "../controls";
import { Empty, Logo, PageSkeleton, Row, Section, SettingsPage } from "../layout";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import { whatsappGreen } from "../whatsapp-look";
import {
  buttonKinds,
  draftOf,
  emptyForm,
  formFrom,
  previewOf,
  sameForm,
  type TemplateForm,
  templateLanguages,
  templateName,
} from "../whatsapp-template-draft";

type Fields = Record<string, string>;
type Change = (next: Partial<TemplateForm>) => void;
type Parts = { form: TemplateForm; change: Change; fields: Fields };

const TemplateWriter = lazy(() => import("../whatsapp-template-writer"));

export function WhatsAppTemplatePage({ channel, template }: { channel: string; template: string }) {
  const workspace = useAccount().workspace.id;
  const { data: templates } = useQuery(whatsappTemplatesQuery(workspace, channel));

  if (template === "new") {
    return <TemplateEditor workspace={workspace} channel={channel} />;
  }

  if (!templates) {
    return <PageSkeleton />;
  }

  const existing = templates.find((t) => t.id === template);
  if (!existing) {
    return (
      <SettingsPage title="Template not found" description="It may have been deleted in WhatsApp Manager.">
        <BackToNumber channel={channel} />
      </SettingsPage>
    );
  }

  return <TemplateEditor key={existing.id} workspace={workspace} channel={channel} existing={existing} />;
}

function BackToNumber({ channel }: { channel: string }) {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/integrations/whatsapp/$channel", params: { channel } })}
      >
        Back to number
      </Button>
    </div>
  );
}

/*
 * useTemplateSave sends the template to Meta and goes back to the number.
 */
function useTemplateSave(workspace: string, channel: string, existing?: WhatsAppTemplate) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  return useMutation({
    mutationFn: (draft: WhatsAppTemplateDraft) =>
      existing
        ? updateWhatsAppTemplate(workspace, channel, existing.id, draft)
        : createWhatsAppTemplate(workspace, channel, draft),
    onSuccess: async (saved) => {
      await queryClient.invalidateQueries({ queryKey: whatsappTemplatesQuery(workspace, channel).queryKey });
      toast.success(`${saved.name} sent for review`, {
        description: "Meta usually answers within minutes. Its answer shows on the number's page.",
      });
      navigate({ to: "/d/settings/integrations/whatsapp/$channel", params: { channel } });
    },
  });
}

function TemplateEditor({
  workspace,
  channel,
  existing,
}: {
  workspace: string;
  channel: string;
  existing?: WhatsAppTemplate;
}) {
  const [start] = useState(() => (existing ? formFrom(existing) : emptyForm()));
  const [form, setForm] = useState(start);
  const save = useTemplateSave(workspace, channel, existing);
  const fields: Fields = save.error instanceof ApiError ? save.error.fields : {};
  const change: Change = (next) => setForm((now) => ({ ...now, ...next }));
  const ready = form.body.trim() !== "" && (existing !== undefined || form.name !== "");

  return (
    <SettingsPage
      title={existing ? existing.name : "New template"}
      description={
        existing
          ? "Changes go back to Meta's review. Until Meta approves them, the template cannot be sent."
          : "Meta reviews each template before it can be sent, usually within minutes."
      }
      mark={<Logo icon={RiWhatsappLine} brand={whatsappGreen} />}
      footer={
        <SaveBar
          dirty={!sameForm(form, start)}
          canSave={ready}
          saving={save.isPending}
          error={save.error ? failureText(save.error) : null}
          message={existing ? "Unsaved changes" : "Not sent to Meta yet"}
          saveLabel="Submit for review"
          onSave={() => save.mutate(draftOf(form))}
          onDiscard={() => setForm(start)}
        />
      }
    >
      <BackToNumber channel={channel} />
      {existing ? null : <NameSection form={form} change={change} fields={fields} />}
      <KindSection form={form} change={change} />
      <MessageSection form={form} change={change} fields={fields} />
      <ButtonsSection form={form} change={change} fields={fields} />
      {existing ? <DeleteSection workspace={workspace} channel={channel} template={existing} /> : null}
    </SettingsPage>
  );
}

function NameSection({ form, change, fields }: Parts) {
  return (
    <Section title="Template">
      <Row label="Name" description="Teammates find it by this name when they type / in a reply.">
        <Input
          aria-label="Name"
          size="small"
          placeholder="order_shipped"
          autoFocus
          value={form.name}
          onChange={(typed) => change({ name: templateName(typed) })}
          isInvalid={Boolean(fields.name)}
          errorMessage={fields.name}
          className="w-56"
        />
      </Row>
      <Row label="Language" description="The language it is written in. Neither can change once it is sent.">
        <Select
          aria-label="Language"
          size="sm"
          selectedKey={form.language}
          onSelectionChange={(key) => change({ language: String(key) })}
          popoverClassName="w-56"
        >
          {templateLanguages.map((l) => (
            <SelectItem key={l.id} id={l.id} textValue={l.name}>
              {l.name}
            </SelectItem>
          ))}
        </Select>
      </Row>
    </Section>
  );
}

function KindSection({ form, change }: { form: TemplateForm; change: Change }) {
  return (
    <Section
      bare
      title="Kind"
      description="Meta prices each kind differently, and moves a template it finds in the wrong one."
    >
      <RadioGroup
        aria-label="Kind"
        value={form.category}
        onChange={(value) => change({ category: value === "marketing" ? "marketing" : "utility" })}
      >
        <RadioCard
          value="utility"
          title="Utility"
          description="About something the customer did or asked for: an order, a booking, a ticket."
        />
        <RadioCard
          value="marketing"
          title="Marketing"
          description="Offers, news and anything else that promotes. Customers can opt out."
        />
      </RadioGroup>
    </Section>
  );
}

function MessageSection({ form, change, fields }: Parts) {
  const bodyId = useId();

  return (
    <Section title="Message" description="Variables fill in with the customer's details when the template is sent.">
      <div className="flex flex-col gap-4 py-3 pe-3">
        <Input
          label="Header"
          hint="Optional. Shown in bold above the message."
          size="small"
          maxLength={60}
          value={form.header}
          onChange={(header) => change({ header })}
          isInvalid={Boolean(fields.header)}
          errorMessage={fields.header}
        />
        <div className="flex flex-col gap-1.5">
          <Label id={bodyId}>Body</Label>
          <Suspense fallback={<div className="min-h-24" />}>
            <TemplateWriter
              value={form.body}
              label="Template body"
              placeholder="Write the message customers will get. Type {{ for a variable."
              customs={form.customs}
              onChange={(body) => change({ body })}
              onCustoms={(customs) => change({ customs })}
            />
          </Suspense>
          {fields.body ? <HintText isInvalid>{fields.body}</HintText> : null}
        </div>
        <Input
          label="Footer"
          hint="Optional. Small and quiet under the message."
          size="small"
          maxLength={60}
          value={form.footer}
          onChange={(footer) => change({ footer })}
          isInvalid={Boolean(fields.footer)}
          errorMessage={fields.footer}
        />
        <Preview form={form} />
      </div>
    </Section>
  );
}

/*
 * Preview is the message as the customer reads it, each variable as its
 * example.
 */
function Preview({ form }: { form: TemplateForm }) {
  if (form.body.trim() === "" && form.header.trim() === "") {
    return null;
  }

  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-body-2-medium text-text-secondary">As the customer reads it</span>
      <div className="flex w-fit max-w-md flex-col gap-2 rounded-2xl bg-background-primary-default p-3 text-body-regular text-text-primary">
        {form.header.trim() ? <p className="text-body-medium">{form.header}</p> : null}
        <MessageBody text={previewOf(form)} />
        {form.footer.trim() ? <p className="text-body-2-regular text-text-tertiary">{form.footer}</p> : null}
        {form.buttons.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {form.buttons.map((b, i) => (
              <Button
                // biome-ignore lint/suspicious/noArrayIndexKey: buttons have no id until Meta takes them
                key={i}
                variant="secondary"
                size="xs"
                disabled
              >
                {b.text || buttonKinds[b.type].label}
              </Button>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function ButtonsSection({ form, change, fields }: Parts) {
  const set = (buttons: WhatsAppButton[]) => change({ buttons });
  const update = (i: number, next: Partial<WhatsAppButton>) =>
    set(form.buttons.map((b, at) => (at === i ? { ...b, ...next } : b)));

  return (
    <Section
      title="Buttons"
      description="Optional. Shown under the message."
      action={<AddButtonMenu buttons={form.buttons} onAdd={(type) => set([...form.buttons, { type, text: "" }])} />}
    >
      {form.buttons.length === 0 ? <Empty>No buttons yet.</Empty> : null}
      {form.buttons.map((b, i) => (
        <ButtonRow
          // biome-ignore lint/suspicious/noArrayIndexKey: buttons have no id until Meta takes them
          key={i}
          button={b}
          index={i}
          fields={fields}
          onChange={(next) => update(i, next)}
          onRemove={() => set(form.buttons.filter((_, at) => at !== i))}
        />
      ))}
    </Section>
  );
}

function AddButtonMenu({ buttons, onAdd }: { buttons: WhatsAppButton[]; onAdd: (type: WhatsAppButtonType) => void }) {
  const [open, setOpen] = useState(false);
  const kinds = (Object.keys(buttonKinds) as WhatsAppButtonType[]).filter(
    (type) => buttons.filter((b) => b.type === type).length < buttonKinds[type].limit,
  );

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        isDisabled={kinds.length === 0 || buttons.length >= 10}
        className="flex h-8 items-center gap-1.5 rounded-lg border border-border-button-default bg-background-primary-default px-2.5 text-body-2-medium text-text-primary transition-colors duration-150 hover:bg-background-primary-hover disabled:opacity-50"
      >
        <RiAddLine aria-hidden className="size-4 text-foreground-icon-secondary" />
        Add
      </DropdownTrigger>
      <DropdownPopover aria-label="Add a button" placement="bottom end">
        {kinds.map((type) => (
          <DropdownItem
            key={type}
            onSelect={() => {
              onAdd(type);
              setOpen(false);
            }}
          >
            {buttonKinds[type].label}
          </DropdownItem>
        ))}
      </DropdownPopover>
    </Dropdown>
  );
}

function ButtonRow({
  button,
  index,
  fields,
  onChange,
  onRemove,
}: {
  button: WhatsAppButton;
  index: number;
  fields: Fields;
  onChange: (next: Partial<WhatsAppButton>) => void;
  onRemove: () => void;
}) {
  const kind = buttonKinds[button.type];
  const field = `buttons.${index}`;
  const target = button.type === "url" ? "url" : "phone_number";
  const targetError = fields[`${field}.${target}`] ?? fields[field];

  return (
    <Row label={kind.label} description={kind.detail}>
      <div className="flex items-start gap-2">
        <div className="flex w-56 flex-col gap-2">
          <Input
            aria-label={`${kind.label} label`}
            size="small"
            placeholder="Label"
            maxLength={25}
            value={button.text}
            onChange={(text) => onChange({ text })}
            isInvalid={Boolean(fields[`${field}.text`])}
            errorMessage={fields[`${field}.text`]}
          />
          {button.type === "quick_reply" ? null : (
            <Input
              aria-label={button.type === "url" ? "Link address" : "Phone number"}
              size="small"
              inputDir="ltr"
              placeholder={button.type === "url" ? "https://example.com/orders" : "+2348012345678"}
              value={button[target] ?? ""}
              onChange={(value) => onChange({ [target]: value })}
              isInvalid={Boolean(targetError)}
              errorMessage={targetError}
            />
          )}
        </div>
        <IconButton icon={RiCloseLine} size="small" aria-label={`Remove ${kind.label}`} onClick={onRemove} />
      </div>
    </Row>
  );
}

function DeleteSection({
  workspace,
  channel,
  template,
}: {
  workspace: string;
  channel: string;
  template: WhatsAppTemplate;
}) {
  const [confirming, setConfirming] = useState(false);
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  return (
    <Section title="Danger zone">
      <Row label="Delete template" description="Meta removes it at once. Its name cannot be used again for 30 days.">
        <Danger onPress={() => setConfirming(true)}>Delete</Danger>
      </Row>
      <AlertDialog
        isOpen={confirming}
        onOpenChange={setConfirming}
        title={`Delete ${template.name}?`}
        description="It is removed from WhatsApp for good. Messages already sent with it stay in their conversations."
        confirmLabel="Delete"
        pendingLabel="Deleting…"
        onConfirm={async () => {
          await deleteWhatsAppTemplate(workspace, channel, template.id);
          await queryClient.invalidateQueries({ queryKey: whatsappTemplatesQuery(workspace, channel).queryKey });
          toast.success(`${template.name} deleted`);
          navigate({ to: "/d/settings/integrations/whatsapp/$channel", params: { channel } });
        }}
      />
    </Section>
  );
}
