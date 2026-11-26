/*
 * The languages a help center speaks: the default one its articles are
 * written in, the others readers may pick, each removable, and what
 * readers of another language see of an article not translated into
 * it. Every change goes into the settings page's draft; removing a
 * language hides its translations but never deletes them.
 */
import { RiCloseLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { IconButton } from "@/components/base/buttons/icon-button";
import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { type Language, languagesQuery } from "@/lib/api/translations";

import { Row, Section } from "./layout";

export interface LanguagesForm {
  defaultLocale: string;
  locales: string[];
  untranslated: "original" | "hide";
}

/*
 * languageName names a language in English and in itself, such as
 * "French · Français".
 */
export function languageName(languages: Language[], code: string): string {
  const found = languages.find((l) => l.code === code);
  if (!found) {
    return code;
  }
  return found.name === found.native_name ? found.name : `${found.name} · ${found.native_name}`;
}

export function LanguagesSection({
  workspace,
  form,
  onChange,
}: {
  workspace: string;
  form: LanguagesForm;
  onChange: (change: Partial<LanguagesForm>) => void;
}) {
  const { data: languages = [] } = useQuery({ ...languagesQuery(workspace), enabled: workspace !== "" });
  const others = form.locales.filter((l) => l !== form.defaultLocale);
  const addable = languages.filter((l) => l.code !== form.defaultLocale && !others.includes(l.code));
  const remove = (code: string) => onChange({ locales: [form.defaultLocale, ...others.filter((l) => l !== code)] });
  const add = (code: string) => onChange({ locales: [form.defaultLocale, ...others, code] });

  return (
    <Section title="Languages" description="Readers pick a language from the help center's header.">
      <Row label="Default language" description="The language your articles are written in. Its pages keep their address.">
        <Select
          aria-label="Default language"
          size="sm"
          selectedKey={form.defaultLocale}
          onSelectionChange={(key) => {
            if (key !== null) {
              const next = String(key);
              onChange({ defaultLocale: next, locales: [next, ...others.filter((l) => l !== next)] });
            }
          }}
          popoverClassName="w-64"
          className="w-64"
        >
          {languages.map((l) => (
            <SelectItem key={l.code} id={l.code} textValue={languageName(languages, l.code)}>
              {languageName(languages, l.code)}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row
        label="Other languages"
        description={
          others.length === 0
            ? "Add a language to translate articles into it."
            : "Each one lives under its own address, such as /fr. Removing one hides its translations."
        }
      >
        <div className="flex max-w-[26rem] flex-wrap items-center justify-end gap-2">
          {others.map((code) => (
            <span
              key={code}
              className="inline-flex h-8 items-center gap-1 rounded-lg border border-border-button-default bg-background-primary-default ps-2.5 pe-1 text-body-2-medium text-text-primary"
            >
              {languageName(languages, code)}
              <IconButton
                icon={RiCloseLine}
                size="small"
                variant="quiet"
                aria-label={`Remove ${languageName(languages, code)}`}
                onClick={() => remove(code)}
              />
            </span>
          ))}
          <ComboBox
            aria-label="Add a language"
            placeholder="Add a language"
            size="small"
            className="w-52"
            defaultItems={addable}
            selectedKey={null}
            onSelectionChange={(key) => (key === null ? undefined : add(String(key)))}
            menuTrigger="focus"
          >
            {(l: Language) => (
              <ComboBoxItem id={l.code} textValue={`${l.name} ${l.native_name}`}>
                {languageName(languages, l.code)}
              </ComboBoxItem>
            )}
          </ComboBox>
        </div>
      </Row>
      {others.length > 0 ? (
        <div className="px-5 py-4">
          <p className="text-body-regular text-text-primary">Articles without a translation</p>
          <p className="text-body-2-regular text-text-secondary">
            What readers of another language see of an article nobody translated yet.
          </p>
          <RadioGroup
            aria-label="Articles without a translation"
            className="mt-3 grid gap-2 sm:grid-cols-2"
            value={form.untranslated}
            onChange={(next) => onChange({ untranslated: next as LanguagesForm["untranslated"] })}
          >
            <RadioCard
              value="original"
              title={`Show in ${languageName(languages, form.defaultLocale).split(" · ")[0]}`}
              description="Readers get the original, with a note that it isn't translated yet."
            />
            <RadioCard value="hide" title="Hide" description="Readers only see what is translated into their language." />
          </RadioGroup>
        </div>
      ) : null}
    </Section>
  );
}
