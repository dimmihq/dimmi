/*
 * The workspace's help center: its name and address, how it looks, how
 * readers sign in to see articles kept to an audience, and whether
 * search engines may index it and readers may ask the AI. The address
 * and look are saved together with Save, as is the product's login
 * address; switches save at once.
 */
import { RiBookOpenLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Notification } from "@/components/base/notification/notification";
import { type HelpCenter, helpCentersQuery, updateHelpCenter } from "@/lib/api/help-centers";

import { Action, Colour, Field, Toggle } from "./controls";
import { Glyph, Item, Row, Section, SettingsPage } from "./layout";
import { useAccount } from "./use-account";

interface Form {
  name: string;
  subdomain: string;
  color: string;
  title: string;
  login: string;
}

function formOf(center: HelpCenter): Form {
  return {
    name: center.name,
    subdomain: center.subdomain,
    color: center.settings.primary_color ?? "#10b981",
    title: center.settings.home_title ?? "",
    login: center.settings.login_url ?? "",
  };
}

export function HelpCentersSettings() {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const { data: centers = [], isPending } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers[0];
  const [form, setForm] = useState<Form | null>(null);

  useEffect(() => {
    if (center && form === null) {
      setForm(formOf(center));
    }
  }, [center, form]);

  const update = useMutation({
    mutationFn: (change: Parameters<typeof updateHelpCenter>[2]) =>
      updateHelpCenter(workspace, center?.id ?? "", change),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey }),
  });

  return (
    <SettingsPage title="Help centers" description="Your public docs, at their own address.">
      <Section title="Help centers" loading={isPending} rows={1}>
        {centers.map((c) => (
          <Item key={c.id} mark={<Glyph icon={RiBookOpenLine} />} title={c.name} detail={c.url}>
            {c.is_default ? (
              <Chip variant="caption" color="neutral">
                Default
              </Chip>
            ) : null}
          </Item>
        ))}
      </Section>
      {center && form ? (
        <>
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
                      home_title: form.title,
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
            <Row label="Brand color" description="Links, buttons and the header.">
              <Colour label="Help center color" value={form.color} onChange={(color) => setForm({ ...form, color })} />
            </Row>
          </Section>
          <Section
            title="Readers sign in with"
            description="To read articles kept to an audience."
            action={
              <Action
                loading={update.isPending}
                onPress={() =>
                  update.mutate({ settings: { ...center.settings, login_url: form.login.trim() || undefined } })
                }
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
        </>
      ) : null}
    </SettingsPage>
  );
}
