/*
 * Steps for features still on their way: a logo for the messenger and
 * mail, and the pages the AI agent will answer from. They show what is
 * coming so the setup reads whole, and let the team move on; nothing on
 * them is saved yet.
 */
import { RiAddLine, RiImageLine, RiLinkM } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";

import { StepActions, StepHeader } from "./frame";
import type { StepProps } from "./step-props";

export function LogoStep({ label, onNext, onBack }: StepProps) {
  return (
    <div>
      <StepHeader
        label={label}
        title="Add your logo"
        description="Your logo appears in the messenger and in emails to your customers."
      />
      <p className="mt-8 text-body-2-regular text-text-tertiary">
        Recommended size: 240×240 px. PNG or JPEG, up to 10 MB.
      </p>
      <div className="mt-3 flex items-center gap-3">
        <Button variant="secondary" size="small" leadingIcon={RiImageLine} disabled>
          Upload logo
        </Button>
        <span className="text-body-2-regular text-text-tertiary">Coming soon. You can add one later in settings.</span>
      </div>
      <StepActions skipLabel="Continue without a logo" onSkip={onNext} onBack={onBack} />
    </div>
  );
}

export function KnowledgeStep({ label, onNext, onBack }: StepProps) {
  return (
    <div>
      <StepHeader
        label={label}
        title="Add your knowledge sources"
        description="Point dimmi at your docs, help center or website, and the AI agent answers from them."
      />
      <div className="mt-8 flex items-end gap-2">
        <Input
          className="flex-1"
          label="Sitemap or page address"
          placeholder="docs.acme.com/sitemap.xml"
          leadingIcon={RiLinkM}
          inputDir="ltr"
          isDisabled
        />
        <Button variant="secondary" leadingIcon={RiAddLine} disabled>
          Add
        </Button>
      </div>
      <p className="mt-2 text-body-2-regular text-text-tertiary">
        Coming soon with the AI agent. A sitemap will index your whole site in one go.
      </p>
      <StepActions skipLabel="Continue without knowledge" onSkip={onNext} onBack={onBack} />
    </div>
  );
}
