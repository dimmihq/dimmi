/*
 * The workspace's help centers, one picked to edit: its name and address,
 * its own domain, how it looks, how
 * readers sign in to see articles kept to an audience, and whether
 * search engines may index it and readers may ask the AI, and which of
 * the workspace's messengers its pages show, if any. Every change waits
 * in a draft until Save in the bar at the bottom; its own domain is set
 * up on its own. A help center that is not the default can be deleted at
 * the bottom.
 */
import { RiBookOpenLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { Fragment, useEffect, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { CreateHelpCenterDialog } from "@/features/help-center/create-help-center";
import { HelpCenterSwitcher } from "@/features/help-center/help-center-switcher";
import { useHelpCenterPick } from "@/features/help-center/use-help-center-pick";
import { type HelpCenter, type HelpCenterSettings, helpCentersQuery, updateHelpCenter } from "@/lib/api/help-centers";
import { type WidgetSummary, widgetSettingsQuery } from "@/lib/api/widget-settings";

import { Action, Colour, Field, Toggle } from "./controls";
import { DeleteHelpCenter } from "./help-center-delete";
import { HelpCenterDomain } from "./help-center-domain";
import { Glyph, Item, Row, Section, SettingsPage } from "./layout";
import { rowLink } from "./row-link";
import { SaveBar } from "./save-bar";

interface Form {
  name: string;
  subdomain: string;
  color: string;
  title: string;
  subtitle: string;
  footer: string;
  contact: string;
  login: string;
  emailSignIn: boolean;
  messenger: boolean;
  messengerId: string;
  askAi: boolean;
  indexing: boolean;
}

function formOf(center: HelpCenter): Form {
  return {
    name: center.name,
    subdomain: center.subdomain,
    color: center.settings.primary_color ?? "#10b981",
    title: center.settings.home_title ?? "",
    subtitle: center.settings.home_subtitle ?? "",
    footer: center.settings.footer_text ?? "",
    contact: center.settings.contact_url ?? "",
    login: center.settings.login_url ?? "",
    emailSignIn: center.settings.email_sign_in ?? true,
    messenger: center.settings.messenger ?? true,
    messengerId: center.settings.messenger_id ?? "",
    askAi: center.ask_ai_enabled,
    indexing: center.indexing_enabled,
  };
}

/*
 * changeOf is the update a save sends: only what the form changed from
 * the help center as it is, text trimmed. The settings go whole, with
 * the changed ones laid over them, since the server keeps what it is
 * sent.
 */
function changeOf(center: HelpCenter, form: Form): Parameters<typeof updateHelpCenter>[2] {
  const was = formOf(center);
  const differs = (key: keyof Form) => form[key] !== was[key];
  const text = (key: "title" | "subtitle" | "footer" | "contact" | "login") => form[key].trim();
  const settings: HelpCenterSettings = {
    ...(differs("color") ? { primary_color: form.color } : {}),
    ...(differs("title") ? { home_title: text("title") } : {}),
    ...(differs("subtitle") ? { home_subtitle: text("subtitle") } : {}),
    ...(differs("footer") ? { footer_text: text("footer") } : {}),
    ...(differs("contact") ? { contact_url: text("contact") } : {}),
    ...(differs("login") ? { login_url: text("login") } : {}),
    ...(differs("emailSignIn") ? { email_sign_in: form.emailSignIn } : {}),
    ...(differs("messenger") ? { messenger: form.messenger } : {}),
    ...(differs("messengerId") ? { messenger_id: form.messengerId } : {}),
  };

  return {
    ...(differs("name") ? { name: form.name } : {}),
    ...(differs("subdomain") ? { subdomain: form.subdomain } : {}),
    ...(differs("askAi") ? { ask_ai_enabled: form.askAi } : {}),
    ...(differs("indexing") ? { indexing_enabled: form.indexing } : {}),
    ...(Object.keys(settings).length > 0 ? { settings: { ...center.settings, ...settings } } : {}),
  };
}

/*
 * HelpCentersSettings keeps every change to the picked help center in
 * one draft. The bar at the bottom saves it or discards it; leaving the
 * page, or picking another help center, with changes unsaved asks first.
 * version counts saves and discards: the draft's sections remount on
 * it, so the controls that keep their own state show the help center
 * again.
 */
export function HelpCentersSettings() {
  const queryClient = useQueryClient();
  const { workspace, centers, center, isPending, pick } = useHelpCenterPick();
  const [form, setForm] = useState<Form | null>(null);
  const [formFor, setFormOf] = useState("");
  const [version, setVersion] = useState(0);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (center && formFor !== center.id) {
      setForm(formOf(center));
      setFormOf(center.id);
    }
  }, [center, formFor]);

  const dirty =
    center !== undefined &&
    form !== null &&
    formFor === center.id &&
    JSON.stringify(form) !== JSON.stringify(formOf(center));
  const save = useMutation({
    mutationFn: () => updateHelpCenter(workspace, center?.id ?? "", changeOf(center as HelpCenter, form as Form)),
    onSuccess: async (saved) => {
      await queryClient.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey });
      setForm(formOf(saved));
      setVersion((v) => v + 1);
      toast.success("Help center saved");
    },
  });
  const discard = () => {
    if (center) {
      setForm(formOf(center));
    }
    setVersion((v) => v + 1);
    save.reset();
  };
  const switchTo = (id: string) => {
    if (id === center?.id || (dirty && !window.confirm("Leave without saving your changes?"))) {
      return;
    }
    save.reset();
    pick(id);
  };
  useBlocker({
    shouldBlockFn: () => dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => dirty,
  });

  return (
    <SettingsPage
      title="Help centers"
      description="Your public docs, at their own address."
      actions={
        centers.length > 1 ? (
          <HelpCenterSwitcher
            workspace={workspace}
            centers={centers}
            center={center}
            onPick={switchTo}
            className="w-56"
          />
        ) : null
      }
      footer={
        <SaveBar
          dirty={dirty}
          saving={save.isPending}
          error={save.error?.message}
          onSave={() => save.mutate()}
          onDiscard={discard}
        />
      }
    >
      <Section
        title="Help centers"
        loading={isPending}
        rows={1}
        action={<Action onPress={() => setCreating(true)}>New help center</Action>}
      >
        {centers.map((c) => (
          <Item
            key={c.id}
            mark={<Glyph icon={RiBookOpenLine} />}
            title={
              <button type="button" className={rowLink} onClick={() => switchTo(c.id)}>
                {c.name}
              </button>
            }
            detail={c.url}
          >
            {c.id === center?.id && centers.length > 1 ? (
              <Chip variant="caption" color="lime">
                Editing
              </Chip>
            ) : null}
            {c.is_default ? (
              <Chip variant="caption" color="neutral">
                Default
              </Chip>
            ) : null}
          </Item>
        ))}
      </Section>
      {creating ? <CreateHelpCenterDialog workspace={workspace} onClose={() => setCreating(false)} /> : null}
      {center && form && formFor === center.id ? (
        <Fragment key={center.id}>
          <Section key={`look-${version}`} title="Address and look">
            <Row label="Name">
              <Field label="Help center name" value={form.name} onChange={(name) => setForm({ ...form, name })} />
            </Row>
            <Row label="Address" description={center.url}>
              <Field
                label="Subdomain"
                value={form.subdomain}
                width="w-40"
                onChange={(subdomain) => setForm({ ...form, subdomain })}
              />
            </Row>
            <Row label="Home title" description="The question readers see first.">
              <Field
                label="Home title"
                value={form.title}
                placeholder="How can we help?"
                onChange={(title) => setForm({ ...form, title })}
              />
            </Row>
            <Row label="Home subtitle" description="A line under the logo on the home page.">
              <Field
                label="Home subtitle"
                value={form.subtitle}
                placeholder="Answers from the team behind Acme"
                onChange={(subtitle) => setForm({ ...form, subtitle })}
              />
            </Row>
            <Row label="Footer" description="Shown at the bottom of every page.">
              <Field
                label="Footer text"
                value={form.footer}
                placeholder="© Acme Inc."
                onChange={(footer) => setForm({ ...form, footer })}
              />
            </Row>
            <Row label="Contact link" description="A Contact us link in the footer. Leave empty for none.">
              <Field
                label="Contact link"
                value={form.contact}
                placeholder="https://acme.com/contact"
                onChange={(contact) => setForm({ ...form, contact })}
              />
            </Row>
            <Row label="Brand color" description="Links, buttons and the header.">
              <Colour label="Help center color" value={form.color} onChange={(color) => setForm({ ...form, color })} />
            </Row>
          </Section>
          <HelpCenterDomain workspace={workspace} center={center} />
          <Section
            key={`sign-in-${version}`}
            title="Readers sign in with"
            description="To read articles kept to an audience."
          >
            <Row
              label="Email code"
              description="Readers type their email and the code we send. Messenger visitors arrive signed in."
            >
              <Toggle
                label="Email code"
                on={form.emailSignIn}
                onChange={(emailSignIn) => setForm({ ...form, emailSignIn })}
              />
            </Row>
            <Row
              label="Your product's login"
              description="Where readers sign in to your product. It sends them back with an identity token."
            >
              <Field
                label="Login address"
                value={form.login}
                placeholder="https://app.example.com/help-login"
                onChange={(login) => setForm({ ...form, login })}
              />
            </Row>
          </Section>
          <MessengerSection
            key={`messenger-${version}`}
            workspace={workspace}
            on={form.messenger}
            pickedId={form.messengerId}
            onChange={(change) => setForm({ ...form, ...change })}
          />
          <Section key={`features-${version}`} title="Features">
            <Row label="Ask AI" description="Let readers ask a question and get an answer from your articles.">
              <Toggle label="Ask AI" on={form.askAi} onChange={(askAi) => setForm({ ...form, askAi })} />
            </Row>
            <Row label="Search engines" description="Let Google and others index your articles.">
              <Toggle
                label="Search engines"
                on={form.indexing}
                onChange={(indexing) => setForm({ ...form, indexing })}
              />
            </Row>
          </Section>
          {center.is_default ? null : <DeleteHelpCenter workspace={workspace} center={center} />}
        </Fragment>
      ) : null}
    </SettingsPage>
  );
}

/*
 * MessengerSection shows a messenger on every help center page, on until
 * the team turns it off, and picks which one when the workspace has more
 * than one; the first is shown until one is picked. Both go into the
 * page's draft.
 */
function MessengerSection({
  workspace,
  on,
  pickedId,
  onChange,
}: {
  workspace: string;
  on: boolean;
  pickedId: string;
  onChange: (change: { messenger?: boolean; messengerId?: string }) => void;
}) {
  const { data: messengers = [] } = useQuery({ ...widgetSettingsQuery(workspace), enabled: workspace !== "" });
  const picked = messengers.find((m) => m.id === pickedId) ?? messengers[0];

  return (
    <Section title="Messenger" description="So readers can ask your team without leaving the page.">
      <Row
        label="Show the messenger"
        description={
          messengers.length === 0
            ? "Add a messenger first; the help center shows it once you do."
            : "Readers chat with your team from every page. Ask a human opens it."
        }
      >
        <Toggle label="Show the messenger" on={on} onChange={(next) => onChange({ messenger: next })} />
      </Row>
      {on && messengers.length > 1 ? (
        <Row label="Which messenger" description="Its look, greeting and team show on the help center.">
          <MessengerPicker messengers={messengers} picked={picked} onPick={(id) => onChange({ messengerId: id })} />
        </Row>
      ) : null}
    </Section>
  );
}

function MessengerPicker({
  messengers,
  picked,
  onPick,
}: {
  messengers: WidgetSummary[];
  picked?: WidgetSummary;
  onPick: (id: string) => void;
}) {
  return (
    <Select
      aria-label="Which messenger"
      size="sm"
      selectedKey={picked?.id ?? null}
      onSelectionChange={(key) => (key === null ? undefined : onPick(String(key)))}
      popoverClassName="w-56"
    >
      {messengers.map((m) => (
        <SelectItem key={m.id} id={m.id} textValue={m.name}>
          {m.name}
        </SelectItem>
      ))}
    </Select>
  );
}
