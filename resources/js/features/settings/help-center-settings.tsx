/*
 * The workspace's help centers, one picked to edit: its name and address,
 * its own domain, how it looks, how
 * readers sign in to see articles kept to an audience, and whether
 * search engines may index it and readers may ask the AI, and which of
 * the workspace's messengers its pages show, if any. The address and look
 * are saved together with Save, as is the product's login address;
 * switches and the messenger pick save at once. A help center that is not
 * the default can be deleted at the bottom.
 */
import { RiBookOpenLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Fragment, useEffect, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Notification } from "@/components/base/notification/notification";
import { Select, SelectItem } from "@/components/base/select/select";
import { CreateHelpCenterDialog } from "@/features/help-center/create-help-center";
import { HelpCenterSwitcher } from "@/features/help-center/help-center-switcher";
import { useHelpCenterPick } from "@/features/help-center/use-help-center-pick";
import { type HelpCenter, helpCentersQuery, updateHelpCenter } from "@/lib/api/help-centers";
import { type WidgetSummary, widgetSettingsQuery } from "@/lib/api/widget-settings";

import { Action, Colour, Field, Toggle } from "./controls";
import { DeleteHelpCenter } from "./help-center-delete";
import { HelpCenterDomain } from "./help-center-domain";
import { Glyph, Item, Row, Section, SettingsPage } from "./layout";
import { rowLink } from "./row-link";

interface Form {
  name: string;
  subdomain: string;
  color: string;
  title: string;
  subtitle: string;
  footer: string;
  contact: string;
  login: string;
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
  };
}

export function HelpCentersSettings() {
  const queryClient = useQueryClient();
  const { workspace, centers, center, isPending, pick } = useHelpCenterPick();
  const [form, setForm] = useState<Form | null>(null);
  const [formFor, setFormOf] = useState("");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (center && formFor !== center.id) {
      setForm(formOf(center));
      setFormOf(center.id);
    }
  }, [center, formFor]);

  const update = useMutation({
    mutationFn: (change: Parameters<typeof updateHelpCenter>[2]) =>
      updateHelpCenter(workspace, center?.id ?? "", change),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey }),
  });

  return (
    <SettingsPage
      title="Help centers"
      description="Your public docs, at their own address."
      actions={
        centers.length > 1 ? (
          <HelpCenterSwitcher workspace={workspace} centers={centers} center={center} onPick={pick} className="w-56" />
        ) : null
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
              <button type="button" className={rowLink} onClick={() => pick(c.id)}>
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
          <Section
            title="Address and look"
            action={
              <Action
                loading={update.isPending}
                onPress={() =>
                  update.mutate({
                    name: form.name,
                    subdomain: form.subdomain,
                    settings: {
                      ...center.settings,
                      primary_color: form.color,
                      home_title: form.title.trim(),
                      home_subtitle: form.subtitle.trim(),
                      footer_text: form.footer.trim(),
                      contact_url: form.contact.trim(),
                    },
                  })
                }
              >
                Save
              </Action>
            }
          >
            {update.error ? (
              <div className="py-2.5 pe-2.5">
                <Notification status="error" title={update.error.message} dismissible={false} role="alert" />
              </div>
            ) : null}
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
            title="Readers sign in with"
            description="To read articles kept to an audience."
            action={
              <Action
                loading={update.isPending}
                onPress={() => update.mutate({ settings: { ...center.settings, login_url: form.login.trim() } })}
              >
                Save
              </Action>
            }
          >
            <Row
              label="Email code"
              description="Readers type their email and the code we send. Messenger visitors arrive signed in."
            >
              <Toggle
                label="Email code"
                on={center.settings.email_sign_in ?? true}
                onChange={(on) => update.mutate({ settings: { ...center.settings, email_sign_in: on } })}
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
            workspace={workspace}
            center={center}
            onChange={(change) => update.mutate({ settings: { ...center.settings, ...change } })}
          />
          <Section title="Features">
            <Row label="Ask AI" description="Let readers ask a question and get an answer from your articles.">
              <Toggle
                label="Ask AI"
                on={center.ask_ai_enabled}
                onChange={(on) => update.mutate({ ask_ai_enabled: on })}
              />
            </Row>
            <Row label="Search engines" description="Let Google and others index your articles.">
              <Toggle
                label="Search engines"
                on={center.indexing_enabled}
                onChange={(on) => update.mutate({ indexing_enabled: on })}
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
 * than one; the first is shown until one is picked.
 */
function MessengerSection({
  workspace,
  center,
  onChange,
}: {
  workspace: string;
  center: HelpCenter;
  onChange: (change: { messenger?: boolean; messenger_id?: string }) => void;
}) {
  const { data: messengers = [] } = useQuery({ ...widgetSettingsQuery(workspace), enabled: workspace !== "" });
  const on = center.settings.messenger ?? true;
  const picked = messengers.find((m) => m.id === center.settings.messenger_id) ?? messengers[0];

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
          <MessengerPicker messengers={messengers} picked={picked} onPick={(id) => onChange({ messenger_id: id })} />
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
