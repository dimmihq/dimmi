/*
 * The messenger's own words. All languages edits the words every
 * visitor reads; picking a language edits words only its visitors read,
 * ahead of those. Each empty field shows, as its placeholder, what a
 * visitor reads without it, and a dot marks the languages with words of
 * their own. Emptying a field reverts it.
 */
import { StatusDot } from "@/components/base/badges/status-dot";
import { Select, SelectItem } from "@/components/base/select/select";

import { Field } from "../controls";
import { Row, Section } from "../layout";
import { useBuiltInWords } from "./built-in-words";
import { hasOwnWords } from "./draft";
import { allLanguages, type TabProps } from "./parts";

const fields = [
  { key: "greeting_intro", label: "Greeting", description: "Up to 60 characters." },
  { key: "delay_notice", label: "Away message", description: "Shown when nobody on the team is available." },
];

function LanguagePicker({
  locales,
  language,
  marked,
  onChange,
}: {
  locales: string[];
  language: string;
  marked: (locale: string) => boolean;
  onChange: (language: string) => void;
}) {
  const names = new Intl.DisplayNames(undefined, { type: "language" });
  const options = [allLanguages, ...locales];

  return (
    <Select
      aria-label="Texts language"
      size="sm"
      selectedKey={language}
      onSelectionChange={(key) => onChange(String(key))}
      popoverClassName="w-56"
    >
      {options.map((option) => {
        const name = option === allLanguages ? "All languages" : (names.of(option) ?? option);

        return (
          <SelectItem key={option} id={option} textValue={name}>
            <span className="flex min-w-0 flex-1 items-center justify-between gap-2">
              <span className="truncate">{name}</span>
              {option !== allLanguages && marked(option) ? (
                <>
                  <StatusDot color="indigo" />
                  <span className="sr-only">Has its own texts</span>
                </>
              ) : null}
            </span>
          </SelectItem>
        );
      })}
    </Select>
  );
}

export function TextsSection({
  value,
  set,
  locked,
  locales,
  language,
  onLanguage,
}: TabProps & { locales: string[]; language: string; onLanguage: (language: string) => void }) {
  const shared = language === allLanguages;
  const builtIn = useBuiltInWords(shared ? "en" : language) ?? {};
  const own = shared ? value.texts : (value.texts_by_locale[language] ?? {});

  return (
    <Section
      title="Texts"
      description={shared ? undefined : "Empty fields use the texts for all languages."}
      action={
        <LanguagePicker
          locales={locales}
          language={language}
          marked={(locale) => hasOwnWords(value.texts_by_locale, locale)}
          onChange={onLanguage}
        />
      }
    >
      {fields.map(({ key, label, description }) => (
        <Row key={`${language}:${key}`} label={label} description={description}>
          <Field
            label={label}
            value={own[key] ?? ""}
            placeholder={(shared ? undefined : value.texts[key]) || builtIn[key]}
            disabled={locked}
            onChange={(word) =>
              set(shared ? { texts: { [key]: word } } : { texts_by_locale: { [language]: { [key]: word } } })
            }
          />
        </Row>
      ))}
    </Section>
  );
}
