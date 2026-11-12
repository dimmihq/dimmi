/*
 * The sections of a messenger's settings, in the order the page has
 * always shown them: install, general, appearance, texts, visitors, tabs
 * and the danger zone. Each reads the settings as the draft leaves them
 * and hands every change to the draft; nothing saves until Save.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { AlertDialog } from "@/components/base/modal/modal";
import { deleteWidget } from "@/lib/api/widget-settings";

import { Choice, Colour, Danger, Field, Toggle } from "../controls";
import { Row, Section } from "../layout";
import { Snippet } from "../snippet";
import { frameworks, snippetFor } from "./install";
import { labelled, type TabProps } from "./parts";

const themes = labelled({ auto: "Auto", light: "Light", dark: "Dark" });
const positions = labelled({ right: "Bottom right", left: "Bottom left" });
const launchers = labelled({ bubble: "Bubble", tab: "Tab", hidden: "Hidden" });
const emails = labelled({ offline: "When offline", always: "Always", never: "Never" });
const automatic = "Auto";

export function InstallSection({ value }: Pick<TabProps, "value">) {
  const [framework, setFramework] = useState(frameworks[0] ?? "");
  const seen = value.last_boot_origin ? `Seen on ${value.last_boot_origin}.` : "Not seen on a site yet.";
  const snippet = snippetFor(framework, window.location.origin, value.id);

  return (
    <Section title="Install" description={seen}>
      <Row label="Framework" description="We show the snippet for what your site is built with.">
        <Choice label="Framework" options={frameworks} value={framework} onChange={setFramework} />
      </Row>
      <div className="py-3 pe-3">
        <Snippet code={snippet.code} hint={snippet.hint} className="bg-background-tertiary-default" />
      </div>
    </Section>
  );
}

export function GeneralSection({ value, set, locked, locales }: TabProps & { locales: string[] }) {
  const names = new Intl.DisplayNames(undefined, { type: "language" });
  const nameOf = (code: string) => names.of(code) ?? code;

  return (
    <Section title="General">
      <Row label="Name">
        <Field label="Messenger name" value={value.name} disabled={locked} onChange={(name) => set({ name })} />
      </Row>
      <Row label="On" description="Turn off to hide the messenger everywhere at once.">
        <Toggle label="Messenger on" on={value.enabled} disabled={locked} onChange={(enabled) => set({ enabled })} />
      </Row>
      <Row label="Language" description="Auto follows each visitor's browser.">
        <Choice
          label="Messenger language"
          options={[automatic, ...locales.map(nameOf)]}
          value={value.locale === "auto" ? automatic : nameOf(value.locale)}
          disabled={locked}
          onChange={(label) =>
            set({ locale: label === automatic ? "auto" : (locales.find((code) => nameOf(code) === label) ?? "auto") })
          }
        />
      </Row>
    </Section>
  );
}

export function AppearanceSection({ value, set, locked }: TabProps) {
  const a = value.appearance;

  return (
    <Section title="Appearance">
      <Row label="Colour" description="The launcher, buttons and your messages.">
        <Colour
          label="Messenger colour"
          value={a.primary_color}
          disabled={locked}
          onChange={(hex) => set({ appearance: { primary_color: hex } })}
        />
      </Row>
      <Row label="Theme">
        <Choice
          label="Messenger theme"
          options={themes.options}
          value={themes.label(a.theme)}
          disabled={locked}
          onChange={(label) => set({ appearance: { theme: themes.value(label) } })}
        />
      </Row>
      <Row label="Position">
        <Choice
          label="Position"
          options={positions.options}
          value={positions.label(a.position)}
          disabled={locked}
          onChange={(label) => set({ appearance: { position: positions.value(label) } })}
        />
      </Row>
      <Row label="Launcher">
        <Choice
          label="Launcher"
          options={launchers.options}
          value={launchers.label(a.launcher.style)}
          disabled={locked}
          onChange={(label) => set({ appearance: { launcher: { style: launchers.value(label) } } })}
        />
      </Row>
    </Section>
  );
}

export function TextsSection({ value, set, locked }: TabProps) {
  return (
    <Section title="Texts">
      <Row label="Greeting" description="Up to 60 characters.">
        <Field
          label="Greeting"
          value={value.texts.greeting_intro ?? ""}
          placeholder="Hi there 👋"
          disabled={locked}
          onChange={(greeting_intro) => set({ texts: { greeting_intro } })}
        />
      </Row>
      <Row label="Away message" description="Shown when nobody on the team is available.">
        <Field
          label="Away message"
          value={value.texts.delay_notice ?? ""}
          placeholder="We'll reply by email."
          disabled={locked}
          onChange={(delay_notice) => set({ texts: { delay_notice } })}
        />
      </Row>
    </Section>
  );
}

export function VisitorsSection({ value, set, locked }: TabProps) {
  return (
    <Section title="Visitors">
      <Row label="Ask for an email" description="So you can reply after the visitor leaves.">
        <Choice
          label="Ask for an email"
          options={emails.options}
          value={emails.label(value.collect_email)}
          disabled={locked}
          onChange={(label) => set({ collect_email: emails.value(label) })}
        />
      </Row>
      <Row label="Anonymous visitors" description="Let visitors write before they sign in to your product.">
        <Toggle
          label="Anonymous visitors"
          on={value.allow_anonymous}
          disabled={locked}
          onChange={(allow_anonymous) => set({ allow_anonymous })}
        />
      </Row>
      <Row label="Allowed sites" description="Empty allows any site.">
        <Field
          label="Allowed sites"
          value={value.allowed_origins.join(", ")}
          placeholder="https://*.acme.com"
          disabled={locked}
          onChange={(text) =>
            set({
              allowed_origins: text
                .split(",")
                .map((site) => site.trim())
                .filter(Boolean),
            })
          }
        />
      </Row>
    </Section>
  );
}

export function TabsSection({ value, set, locked }: TabProps) {
  return (
    <Section title="Tabs">
      <Row label="Help" description="Search your help center inside the messenger.">
        <Toggle
          label="Help tab"
          on={value.help_tab_enabled}
          disabled={locked}
          onChange={(help_tab_enabled) => set({ help_tab_enabled })}
        />
      </Row>
      <Row label="What's new">
        <Toggle
          label="What's new tab"
          on={value.changelog_tab_enabled}
          disabled={locked}
          onChange={(changelog_tab_enabled) => set({ changelog_tab_enabled })}
        />
      </Row>
    </Section>
  );
}

export function DangerSection({ workspace, value }: { workspace: string; value: TabProps["value"] }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [confirming, setConfirming] = useState(false);
  const remove = useMutation({
    mutationFn: () => deleteWidget(workspace, value.id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "widget-settings"] });
      await navigate({ to: "/d/settings/$section", params: { section: "messenger" } });
    },
  });

  return (
    <Section title="Danger zone">
      <Row label="Delete this messenger" description="Sites using its snippet stop showing it.">
        <Danger onPress={() => setConfirming(true)}>Delete messenger</Danger>
      </Row>
      <AlertDialog
        isOpen={confirming}
        onOpenChange={setConfirming}
        title={`Delete ${value.name}?`}
        description="Its snippet stops working on every site. Conversations it started stay in the inbox."
        confirmLabel="Delete"
        pendingLabel="Deleting…"
        onConfirm={() => remove.mutateAsync()}
      />
    </Section>
  );
}
