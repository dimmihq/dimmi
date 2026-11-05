/*
 * The forwarding steps for the mail service an address lives in. The
 * server puts the service that fits the address's domain first; anyone
 * can pick another, since a wrong guess costs nothing. Each guide names
 * what tends to go wrong there and links its settings screen.
 */
import { RiExternalLinkLine } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import { Row } from "@/features/settings/layout";
import type { SetupGuide } from "@/lib/api/email";

export function ForwardingGuide({ guides }: { guides: SetupGuide[] }) {
  const [key, setKey] = useState(guides[0]?.key ?? "");
  const guide = guides.find((g) => g.key === key) ?? guides[0];
  if (!guide) {
    return null;
  }

  return (
    <>
      <Row
        label="Set up forwarding"
        description={
          guide.recommended
            ? "We picked the mail service your address's domain uses."
            : "Steps for the mail service your address lives in."
        }
      >
        <Select
          aria-label="Mail service"
          size="sm"
          selectedKey={guide.key}
          onSelectionChange={(picked) => (picked === null ? undefined : setKey(String(picked)))}
          popoverClassName="w-64"
        >
          {guides.map((g) => (
            <SelectItem key={g.key} id={g.key} textValue={g.name}>
              {g.recommended ? `${g.name} (recommended)` : g.name}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <ol className="flex list-decimal flex-col gap-1 border-b border-separator-border py-3 ps-5 pe-3 text-body-2-regular text-text-secondary">
        {guide.steps.map((step) => (
          <li key={step}>{step}</li>
        ))}
      </ol>
      {guide.warnings.map((warning) => (
        <Row key={warning} label="Watch out" description={warning} />
      ))}
      {guide.deep_link ? (
        <Row label={`Open ${guide.name}`}>
          <Button
            variant="secondary"
            size="small"
            trailingIcon={RiExternalLinkLine}
            onClick={() => window.open(guide.deep_link ?? "", "_blank", "noopener")}
          >
            Open settings
          </Button>
        </Row>
      ) : null}
    </>
  );
}
