/*
 * The conversation's attributes, as the details panel shows them: each
 * active attribute the teammate's teams see, beside its value, edited
 * where it stands with the control its type takes. Text and numbers save
 * when the teammate leaves the field or presses Enter, Escape puts the
 * value back; lists, yes or no, dates, files and links save as they are
 * picked. A value the AI set carries a sparkle, and every value says
 * who set it and when on hover. An attribute only the AI fills reads as
 * text. One required before closing is marked while it is empty. A
 * value the server refuses shows why in place. The section refetches
 * whenever anyone changes a value, and is left out when the workspace
 * has no active attribute.
 */
import { RiAttachment2, RiCloseLine, RiSparkling2Line } from "@remixicon/react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type KeyboardEvent, useEffect, useRef, useState } from "react";
import { Focusable } from "react-aria-components";

import { Button } from "@/components/base/buttons/button";
import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";
import { ago } from "@/features/inbox/conversations";
import { aiValueLabel } from "@/lib/ai-values";
import { ApiError } from "@/lib/api/client";
import {
  type ConversationAttribute,
  type ConversationAttributeValue,
  type ConversationAttributeValues,
  conversationAttributeValuesQuery,
  setConversationAttributeValues,
} from "@/lib/api/conversation-attributes";
import { companiesQuery, peopleQuery } from "@/lib/api/customers";
import { uploadAttachment } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { quiet } from "./quiet";
import { QuietTextarea, Row, Rows, Section } from "./rows";

const empty = "Empty";
const clear = "__clear";

/*
 * problemText says why the server refused a value.
 */
const problemText: Record<string, string> = {
  type_mismatch: "That isn't a valid value.",
  invalid_option: "Pick one of the options.",
  not_whole: "Use a whole number.",
  too_long: "That's too long.",
  unknown_file: "That file is gone. Upload it again.",
  archived_attribute: "This attribute was archived.",
  unknown_attribute: "This attribute is no longer available.",
};

function refusal(error: unknown, key: string): string {
  if (!(error instanceof ApiError)) {
    return "The value could not be saved.";
  }
  const problems = error.details.problems as Record<string, string> | undefined;

  return problemText[problems?.[key] ?? ""] ?? error.message;
}

/*
 * sourceText says who set a value and when.
 */
function sourceText(item: ConversationAttributeValue): string | null {
  const s = item.set_by;
  if (!s) {
    return null;
  }
  const who = s.by === "ai" ? "the AI" : s.by === "import" ? "an import" : (s.name ?? "a teammate");

  return `Set by ${who} ${ago(s.at).toLowerCase()}`;
}

/*
 * localInput turns a stored moment into what a datetime-local field
 * shows, in the reader's own time.
 */
function localInput(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) {
    return "";
  }
  const pad = (n: number) => String(n).padStart(2, "0");

  return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}T${pad(at.getHours())}:${pad(at.getMinutes())}`;
}

/*
 * readTyped turns what a text field holds into the value the API stores.
 */
function readTyped(attribute: ConversationAttribute, typed: string): unknown {
  const text = typed.trim();
  if (text === "") {
    return null;
  }
  switch (attribute.data_type) {
    case "number":
    case "decimal": {
      const n = Number(text.replace(",", "."));
      return Number.isNaN(n) ? text : n;
    }
    case "datetime":
      return new Date(text).toISOString();
    default:
      return typed;
  }
}

function shownOf(attribute: ConversationAttribute, value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (attribute.data_type === "datetime") {
    return localInput(String(value));
  }

  return String(value);
}

/*
 * TypedField is text, a number, a date or a moment typed in place: it
 * keeps what is typed until the teammate leaves it or presses Enter.
 */
function TypedField({
  attribute,
  value,
  onSave,
}: {
  attribute: ConversationAttribute;
  value: unknown;
  onSave: (value: unknown) => void;
}) {
  const stored = shownOf(attribute, value);
  const [typed, setTyped] = useState(stored);
  const editing = useRef(false);
  useEffect(() => {
    if (!editing.current) {
      setTyped(stored);
    }
  }, [stored]);

  const commit = () => {
    editing.current = false;
    if (typed !== stored) {
      onSave(readTyped(attribute, typed));
    }
  };
  const keys = (event: KeyboardEvent) => {
    if (event.key === "Enter" && attribute.data_type !== "long_text") {
      (event.target as HTMLElement).blur();
    }
    if (event.key === "Escape") {
      editing.current = false;
      setTyped(stored);
    }
  };

  if (attribute.data_type === "long_text") {
    return (
      <QuietTextarea
        aria-label={attribute.label}
        placeholder={empty}
        value={typed}
        onChange={(next) => {
          editing.current = true;
          setTyped(next);
        }}
        onBlur={commit}
        onKeyDown={keys}
      />
    );
  }

  const type = { date: "date", datetime: "datetime-local" }[attribute.data_type as string] ?? "text";

  return (
    <Input
      aria-label={attribute.label}
      size="small"
      placeholder={empty}
      className="-ms-1.5 min-w-0 flex-1"
      fieldClassName="bg-transparent hover:bg-background-secondary-hover focus-within:bg-background-tertiary-default"
      inputClassName="text-body-2-regular"
      type={type}
      inputMode={attribute.data_type === "number" || attribute.data_type === "decimal" ? "decimal" : undefined}
      value={typed}
      onChange={(next) => {
        editing.current = true;
        setTyped(next);
      }}
      onBlur={commit}
      onKeyDown={keys}
    />
  );
}

function OneOf({
  attribute,
  value,
  choices,
  onSave,
}: {
  attribute: ConversationAttribute;
  value: string | null;
  choices: { id: string; name: string }[];
  onSave: (value: string | null) => void;
}) {
  return (
    <Select
      aria-label={attribute.label}
      size="sm"
      variant="quiet"
      placeholder={empty}
      selectedKey={value}
      onSelectionChange={(key) => onSave(key === null || key === clear ? null : String(key))}
    >
      {[
        ...choices.map((c) => (
          <SelectItem key={c.id} id={c.id} textValue={c.name}>
            {c.name}
          </SelectItem>
        )),
        ...(value === null
          ? []
          : [
              <SelectItem key={clear} id={clear} textValue="Clear">
                <span className="text-text-tertiary">Clear</span>
              </SelectItem>,
            ]),
      ]}
    </Select>
  );
}

function ManyOf({
  attribute,
  value,
  onSave,
}: {
  attribute: ConversationAttribute;
  value: unknown;
  onSave: (value: unknown) => void;
}) {
  const picked = Array.isArray(value) ? value.map(String) : [];

  return (
    <Dropdown>
      <DropdownTrigger aria-label={attribute.label} className={quiet}>
        <span className={cx("truncate", picked.length === 0 && "text-text-tertiary")}>
          {picked.length === 0 ? empty : picked.join(", ")}
        </span>
      </DropdownTrigger>
      <DropdownPopover aria-label={attribute.label} className="w-56 p-1.5">
        {attribute.options.map((o) => (
          <DropdownItem
            key={o}
            selected={picked.includes(o)}
            onSelect={() => {
              const next = picked.includes(o) ? picked.filter((v) => v !== o) : [...picked, o];
              onSave(next.length === 0 ? null : next);
            }}
            className="px-2 py-1.5 text-body-2-medium"
          >
            <span className="min-w-0 flex-1 truncate">{o}</span>
          </DropdownItem>
        ))}
      </DropdownPopover>
    </Dropdown>
  );
}

/*
 * RecordPick links a contact or a company, searched by name as the
 * teammate types.
 */
function RecordPick({
  workspace,
  attribute,
  value,
  onSave,
}: {
  workspace: string;
  attribute: ConversationAttribute;
  value: unknown;
  onSave: (value: unknown) => void;
}) {
  const [query, setQuery] = useState("");
  const contacts = useInfiniteQuery({
    ...peopleQuery(workspace, { q: query }),
    enabled: attribute.data_type === "contact",
  }).data;
  const companies = useInfiniteQuery({
    ...companiesQuery(workspace, query),
    enabled: attribute.data_type === "company",
  }).data;
  const items =
    attribute.data_type === "contact"
      ? (contacts?.pages.flatMap((p) => p.items) ?? []).map((c) => ({
          id: c.id,
          name: c.name || c.email || "Unnamed contact",
        }))
      : (companies?.pages.flatMap((p) => p.items) ?? []).map((c) => ({ id: c.id, name: c.name }));

  return (
    <ComboBox
      aria-label={attribute.label}
      size="small"
      placeholder={empty}
      className="-ms-1.5 min-w-0 flex-1"
      items={items}
      inputValue={query}
      onInputChange={setQuery}
      selectedKey={typeof value === "string" ? value : null}
      onSelectionChange={(key) => {
        if (key !== null) {
          onSave(String(key));
        }
      }}
      emptyMessage={query ? "No matches" : "Type to search"}
    >
      {(item) => (
        <ComboBoxItem id={item.id} textValue={item.name}>
          {item.name}
        </ComboBoxItem>
      )}
    </ComboBox>
  );
}

/*
 * LinkedRecord shows the contact or company a value links to, with a
 * button that takes the link off.
 */
function LinkedRecord({
  workspace,
  attribute,
  value,
  onSave,
}: {
  workspace: string;
  attribute: ConversationAttribute;
  value: unknown;
  onSave: (value: unknown) => void;
}) {
  const [changing, setChanging] = useState(false);
  const id = typeof value === "string" ? value : null;
  const path = attribute.data_type === "contact" ? `/d/customers/people/${id}` : `/d/customers/companies/${id}`;

  if (!id || changing) {
    return (
      <RecordPick
        workspace={workspace}
        attribute={attribute}
        value={id}
        onSave={(next) => {
          setChanging(false);
          onSave(next);
        }}
      />
    );
  }

  return (
    <span className="flex min-w-0 items-center gap-1">
      <a href={path} className="min-w-0 truncate text-text-primary underline-offset-2 hover:underline">
        {attribute.data_type === "contact" ? "View contact" : "View company"}
      </a>
      <Button variant="ghost" size="xs" onClick={() => setChanging(true)}>
        Change
      </Button>
      <Button
        variant="ghost"
        size="xs"
        iconOnly
        leadingIcon={RiCloseLine}
        aria-label="Remove link"
        onClick={() => onSave(null)}
      />
    </span>
  );
}

function FilesField({
  workspace,
  item,
  onSave,
}: {
  workspace: string;
  item: ConversationAttributeValue;
  onSave: (value: unknown) => void;
}) {
  const picker = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const ids = Array.isArray(item.value) ? item.value.map(String) : [];

  const add = async (files: FileList | null) => {
    if (!files || files.length === 0) {
      return;
    }
    setUploading(true);
    try {
      const uploaded = await Promise.all([...files].map((f) => uploadAttachment(workspace, f)));
      onSave([...ids, ...uploaded.map((u) => u.id)]);
    } finally {
      setUploading(false);
    }
  };

  return (
    <div className="flex min-w-0 flex-1 flex-col items-start gap-1 py-1">
      {item.files.map((f) => (
        <span key={f.id} className="flex max-w-full items-center gap-1">
          <RiAttachment2 aria-hidden className="size-3.5 shrink-0 text-foreground-icon-secondary" />
          {f.url ? (
            <a href={f.url} target="_blank" rel="noopener noreferrer" className="min-w-0 truncate hover:underline">
              {f.filename}
            </a>
          ) : (
            <span className="min-w-0 truncate text-text-secondary">{f.filename}</span>
          )}
          <Button
            variant="ghost"
            size="xs"
            iconOnly
            leadingIcon={RiCloseLine}
            aria-label={`Remove ${f.filename}`}
            onClick={() => {
              const next = ids.filter((id) => id !== f.id);
              onSave(next.length === 0 ? null : next);
            }}
          />
        </span>
      ))}
      <input ref={picker} type="file" multiple hidden onChange={(e) => void add(e.target.files)} />
      <Button variant="secondary" size="xs" disabled={uploading} onClick={() => picker.current?.click()}>
        {uploading ? "Uploading…" : ids.length === 0 ? "Add files" : "Add more"}
      </Button>
    </div>
  );
}

function Control({
  workspace,
  item,
  onSave,
}: {
  workspace: string;
  item: ConversationAttributeValue;
  onSave: (value: unknown) => void;
}) {
  const { attribute, value } = item;

  if (attribute.filled_by === "ai") {
    const shown = aiValueLabel(attribute, value);
    return shown === null ? (
      <span className="text-text-tertiary">Not found yet</span>
    ) : (
      <span dir="auto" className="min-w-0 truncate" title={shown}>
        {shown}
      </span>
    );
  }

  switch (attribute.data_type) {
    case "choice":
      return (
        <OneOf
          attribute={attribute}
          value={typeof value === "string" ? value : null}
          choices={attribute.options.map((o) => ({ id: o, name: o }))}
          onSave={onSave}
        />
      );
    case "boolean":
      return (
        <OneOf
          attribute={attribute}
          value={typeof value === "boolean" ? String(value) : null}
          choices={[
            { id: "true", name: "Yes" },
            { id: "false", name: "No" },
          ]}
          onSave={(next) => onSave(next === null ? null : next === "true")}
        />
      );
    case "multiple_choice":
      return <ManyOf attribute={attribute} value={value} onSave={onSave} />;
    case "file":
      return <FilesField workspace={workspace} item={item} onSave={onSave} />;
    case "contact":
    case "company":
      return <LinkedRecord workspace={workspace} attribute={attribute} value={value} onSave={onSave} />;
    default:
      return <TypedField attribute={attribute} value={value} onSave={onSave} />;
  }
}

function Mark({ item }: { item: ConversationAttributeValue }) {
  const text = sourceText(item);
  if (!text || item.value === null) {
    return null;
  }

  return (
    <TooltipTrigger>
      <Focusable>
        <span role="img" aria-label={text} className="flex shrink-0 items-center">
          {item.set_by?.by === "ai" ? (
            <RiSparkling2Line aria-hidden className="size-3.5 text-foreground-icon-secondary" />
          ) : (
            <span aria-hidden className="size-1.5 rounded-full bg-foreground-icon-tertiary" />
          )}
        </span>
      </Focusable>
      <Tooltip>{text}</Tooltip>
    </TooltipTrigger>
  );
}

function AttributeRow({
  workspace,
  conversation,
  item,
  missing,
}: {
  workspace: string;
  conversation: string;
  item: ConversationAttributeValue;
  missing: boolean;
}) {
  const client = useQueryClient();
  const key = conversationAttributeValuesQuery(workspace, conversation).queryKey;
  const save = useMutation({
    mutationFn: (value: unknown) =>
      setConversationAttributeValues(workspace, conversation, { [item.attribute.key]: value }),
    onSuccess: (next) => client.setQueryData<ConversationAttributeValues>(key, next),
  });
  const { attribute } = item;
  const required = attribute.required_to_close && (item.value === null || item.value === "");
  const long = attribute.data_type === "long_text" || attribute.data_type === "file";

  const name = (
    <span
      className={cx("flex min-w-0 items-center gap-1", missing && "text-text-error-primary")}
      title={attribute.description || attribute.label}
    >
      <span className="truncate">{attribute.label}</span>
      {required ? <span className="text-text-tertiary">*</span> : null}
    </span>
  );
  const control = <Control workspace={workspace} item={item} onSave={(value) => save.mutate(value)} />;
  const notes = (
    <>
      {required && missing ? (
        <span className="text-caption-1-medium text-text-error-primary">Required to close</span>
      ) : null}
      {save.error ? (
        <span role="alert" className="text-caption-1-medium text-text-error-primary">
          {refusal(save.error, attribute.key)}
        </span>
      ) : null}
    </>
  );

  if (long) {
    return (
      <div className="flex flex-col pb-1" data-attribute={attribute.key}>
        <dt className="flex min-h-8 items-center gap-1.5 text-body-2-regular text-text-tertiary">
          {name}
          <Mark item={item} />
        </dt>
        <dd className="flex min-w-0 flex-col gap-0.5 text-body-2-regular text-text-primary">
          {control}
          {notes}
        </dd>
      </div>
    );
  }

  return (
    <div className="flex flex-col" data-attribute={attribute.key}>
      <Row label={name}>
        {control}
        <Mark item={item} />
      </Row>
      <div className="flex flex-col gap-0.5 ps-35 empty:hidden">{notes}</div>
    </div>
  );
}

export function ConversationAttributesSection({
  workspace,
  conversation,
  missing = [],
}: {
  workspace: string;
  conversation: string;
  /* Keys a refused close named, marked until they are filled. */
  missing?: string[];
}) {
  const { data } = useQuery(conversationAttributeValuesQuery(workspace, conversation));
  const active = data?.values.filter((v) => v.attribute.status === "active") ?? [];
  if (active.length === 0) {
    return null;
  }

  return (
    <Section
      title="Conversation attributes"
      aside={data?.checked_at ? `AI read ${ago(data.checked_at).toLowerCase()}` : undefined}
    >
      <Rows
        most={8}
        rows={active.map((item) => (
          <AttributeRow
            key={item.attribute.id}
            workspace={workspace}
            conversation={conversation}
            item={item}
            missing={missing.includes(item.attribute.key)}
          />
        ))}
      />
    </Section>
  );
}
