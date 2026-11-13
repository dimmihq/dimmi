/*
 * The steps for the customer's DNS host, read from the domain's name
 * servers. A wrong guess costs nothing: the link opens a dashboard, and
 * whoever may manage email can pick the right host, which also changes
 * how record names are shown. The host picked is shown as selected
 * while it waits for the page's save bar; the steps follow once it is
 * saved.
 */
import { RiExternalLinkLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import { Row, Section } from "@/features/settings/layout";
import type { HostGuide as Guide } from "@/lib/api/email-domains";

const hosts = [
  ["cloudflare", "Cloudflare"],
  ["route53", "Amazon Route 53"],
  ["godaddy", "GoDaddy"],
  ["namecheap", "Namecheap"],
  ["squarespace", "Squarespace"],
  ["vercel", "Vercel"],
  ["netlify", "Netlify"],
  ["porkbun", "Porkbun"],
  ["hover", "Hover"],
  ["other", "Another host"],
] as const;

export function HostGuide({
  guide,
  manage,
  selected = guide.key,
  onPick,
}: {
  guide: Guide;
  manage: boolean;
  selected?: string;
  onPick: (host: string) => void;
}) {
  return (
    <Section
      title={`Set up at ${guide.name}`}
      description={guide.detected ? "We found your DNS host from your domain's name servers." : undefined}
      action={
        manage ? (
          <Select
            aria-label="DNS host"
            size="sm"
            selectedKey={selected}
            onSelectionChange={(key) => (key === null ? undefined : onPick(String(key)))}
            popoverClassName="w-56"
          >
            {hosts.map(([key, name]) => (
              <SelectItem key={key} id={key} textValue={name}>
                {name}
              </SelectItem>
            ))}
          </Select>
        ) : null
      }
    >
      <ol className="flex list-decimal flex-col gap-1.5 py-3 ps-5 pe-3 text-body-2-regular text-text-secondary">
        {guide.steps.map((step) => (
          <li key={step}>{step}</li>
        ))}
      </ol>
      {guide.quirks.map((quirk) => (
        <Row key={quirk} label="Watch out" description={quirk} />
      ))}
      {guide.deep_link ? (
        <Row label={`Open ${guide.name}`}>
          <Button
            variant="secondary"
            size="small"
            trailingIcon={RiExternalLinkLine}
            onClick={() => window.open(guide.deep_link, "_blank", "noopener")}
          >
            Open DNS settings
          </Button>
        </Row>
      ) : null}
    </Section>
  );
}
