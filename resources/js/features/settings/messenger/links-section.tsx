/*
 * The links a messenger lists on its home screen under Get in touch, such
 * as booking a call or reading the docs: up to six, each with an icon
 * picked from a grid, a title and an https or mailto address. Every edit replaces the whole
 * list in the draft, so the preview shows it at once, and a link the
 * server would refuse says why on its own row.
 */
import { RiAddLine, RiDeleteBinLine } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import type { HomeLink } from "@/lib/api/widget-settings";

import { Field } from "../controls";
import { Section } from "../layout";
import { IconPick } from "./icon-pick";
import type { TabProps } from "./parts";

const maxLinks = 6;

/*
 * keysFor gives each link a key that stays with it while others are
 * added or removed, so each row keeps what was typed into it.
 */
function keysFor(count: number, start = 0): number[] {
  return Array.from({ length: count }, (_, i) => start + i);
}

function LinkRow({
  index,
  link,
  locked,
  problems,
  onChange,
  onRemove,
}: {
  index: number;
  link: HomeLink;
  locked: boolean;
  problems: Record<string, string>;
  onChange: (link: HomeLink) => void;
  onRemove: () => void;
}) {
  const problem = ["title", "url", "icon"].map((field) => problems[`home_links.${index}.${field}`]).find(Boolean);

  return (
    <div className="flex flex-col gap-1.5 border-b border-separator-border py-2.5 pe-2.5 last:border-b-0">
      <div className="flex flex-wrap items-center gap-2">
        <IconPick
          label={`Icon of link ${index + 1}`}
          value={link.icon || "link"}
          disabled={locked}
          onChange={(icon) => onChange({ ...link, icon })}
        />
        <Field
          label={`Title of link ${index + 1}`}
          value={link.title}
          placeholder="Book a call"
          width="w-40 flex-1"
          disabled={locked}
          onChange={(title) => onChange({ ...link, title })}
        />
        <Field
          label={`Address of link ${index + 1}`}
          value={link.url}
          placeholder="https://cal.com/acme"
          width="w-56 flex-1"
          disabled={locked}
          onChange={(url) => onChange({ ...link, url: url.trim() })}
        />
        {locked ? null : (
          <IconButton size="small" icon={RiDeleteBinLine} aria-label={`Remove link ${index + 1}`} onClick={onRemove} />
        )}
      </div>
      {problem ? (
        <p role="alert" className="text-body-2-regular text-text-error-primary">
          {problem}
        </p>
      ) : null}
    </div>
  );
}

export function LinksSection({ value, set, locked, problems }: TabProps & { problems: Record<string, string> }) {
  const links = value.home_links;
  const [keys, setKeys] = useState(() => keysFor(links.length));
  const save = (next: HomeLink[], nextKeys: number[]) => {
    setKeys(nextKeys);
    set({ home_links: next });
  };
  const add = () => save([...links, { title: "", url: "", icon: "link" }], [...keys, Math.max(-1, ...keys) + 1]);
  const action =
    locked || links.length >= maxLinks ? null : (
      <Button size="small" variant="secondary" leadingIcon={RiAddLine} onClick={add}>
        Add link
      </Button>
    );

  return (
    <Section title="Links" description="Listed under Get in touch on the home screen. Up to six." action={action}>
      {links.length === 0 ? (
        <p className="py-3.5 pe-3 text-body-2-regular text-text-tertiary">No links yet.</p>
      ) : (
        links.map((link, index) => (
          <LinkRow
            key={keys[index] ?? index}
            index={index}
            link={link}
            locked={locked}
            problems={problems}
            onChange={(next) => save(links.toSpliced(index, 1, next), keys)}
            onRemove={() => save(links.toSpliced(index, 1), keys.toSpliced(index, 1))}
          />
        ))
      )}
    </Section>
  );
}
