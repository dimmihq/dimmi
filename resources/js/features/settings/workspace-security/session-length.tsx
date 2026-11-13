/*
 * How long a session may work in the workspace before signing in again.
 * The API takes any number of minutes; the page offers round ones. A value
 * set some other way is listed too, so the choice never lies. Activity is
 * written at most every five minutes, so the idle limit can run that much
 * over, which is why the shortest choice is fifteen. A pick waits in the
 * page's draft until its save bar sends it.
 */
import { Select, SelectItem } from "@/components/base/select/select";

import { Row, Section } from "../layout";
import type { SecurityDraft } from "../pages/admin";

interface Choice {
  minutes: number | null;
  label: string;
}

const maxAgeChoices: Choice[] = [
  { minutes: null, label: "No limit" },
  { minutes: 60, label: "1 hour" },
  { minutes: 480, label: "8 hours" },
  { minutes: 720, label: "12 hours" },
  { minutes: 1440, label: "24 hours" },
  { minutes: 10080, label: "7 days" },
  { minutes: 43200, label: "30 days" },
];

const idleChoices: Choice[] = [
  { minutes: null, label: "No limit" },
  { minutes: 15, label: "15 minutes" },
  { minutes: 60, label: "1 hour" },
  { minutes: 480, label: "8 hours" },
  { minutes: 1440, label: "24 hours" },
  { minutes: 10080, label: "7 days" },
];

const noLimit = "none";

function withCurrent(choices: Choice[], current: number | null): Choice[] {
  if (choices.some((choice) => choice.minutes === current)) {
    return choices;
  }

  return [...choices, { minutes: current, label: `${current} minutes` }];
}

function LimitSelect({
  label,
  choices,
  value,
  disabled,
  onChange,
}: {
  label: string;
  choices: Choice[];
  value: number | null;
  disabled: boolean;
  onChange: (minutes: number | null) => void;
}) {
  return (
    <Select
      aria-label={label}
      isDisabled={disabled}
      selectedKey={value === null ? noLimit : String(value)}
      onSelectionChange={(key) => onChange(key === noLimit ? null : Number(key))}
    >
      {withCurrent(choices, value).map((choice) => (
        <SelectItem
          key={choice.label}
          id={choice.minutes === null ? noLimit : String(choice.minutes)}
          textValue={choice.label}
        >
          {choice.label}
        </SelectItem>
      ))}
    </Select>
  );
}

export function SessionLengthSection({ draft }: { draft: SecurityDraft }) {
  const { policy, canManage } = draft;

  if (!policy) {
    return null;
  }

  return (
    <Section title="Session length" description="Only this workspace asks; a person's other workspaces carry on.">
      <Row label="Sign in again after" description="Counted from the last full sign-in.">
        <LimitSelect
          label="Sign in again after"
          choices={maxAgeChoices}
          value={policy.session_max_age_minutes}
          disabled={!canManage}
          onChange={(minutes) => draft.set({ session_max_age_minutes: minutes })}
        />
      </Row>
      <Row
        label="Sign out when idle for"
        description="Activity is noted every five minutes, so this can run that over."
      >
        <LimitSelect
          label="Sign out when idle for"
          choices={idleChoices}
          value={policy.session_idle_timeout_minutes}
          disabled={!canManage}
          onChange={(minutes) => draft.set({ session_idle_timeout_minutes: minutes })}
        />
      </Row>
    </Section>
  );
}
