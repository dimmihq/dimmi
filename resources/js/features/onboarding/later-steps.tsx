/*
 * The logo step, whose upload opens the file picker straight away and
 * saves at once, and the knowledge step, for the pages the AI agent will
 * answer from, which is still on its way and saves nothing yet.
 */
import { RiAddLine, RiLinkM } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { LogoMark } from "@/features/settings/logo-row";
import { PicturePicker } from "@/features/settings/picture-picker";
import { useLogo } from "@/features/settings/use-logo";
import { workspaceQuery } from "@/lib/api/workspace";

import { StepActions, StepHeader } from "./frame";
import type { StepProps } from "./step-props";

export function LogoStep({ workspace, label, onNext, onBack }: StepProps) {
  const { data } = useQuery(workspaceQuery(workspace));
  const logo = useLogo(workspace);
  const url = data?.logo_url ?? null;

  return (
    <div>
      <StepHeader
        label={label}
        title="Add your logo"
        description="Your logo appears in the messenger and in emails to your customers."
      />
      <div className="mt-8 flex items-center gap-4">
        <LogoMark
          url={url}
          className={
            url
              ? "size-16 rounded-2xl object-cover"
              : "flex size-16 items-center justify-center rounded-2xl bg-background-secondary-default text-foreground-icon-tertiary"
          }
        />
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <PicturePicker isPending={logo.upload.isPending} onPick={logo.pick} onTooLarge={logo.refuse}>
              {logo.upload.isPending ? "Uploading…" : url ? "Replace logo" : "Upload logo"}
            </PicturePicker>
            {url ? (
              <Button variant="secondary" size="small" onClick={() => logo.remove.mutate()}>
                Remove
              </Button>
            ) : null}
          </div>
          <p
            className={
              logo.failure ? "text-body-2-regular text-text-error-primary" : "text-body-2-regular text-text-tertiary"
            }
          >
            {logo.failure ?? "A square works best. PNG, JPEG, WebP or GIF, up to 2 MB."}
          </p>
        </div>
      </div>
      <StepActions
        primary={url ? onNext : undefined}
        primaryLabel="Continue"
        skipLabel={url ? undefined : "Continue without a logo"}
        onSkip={url ? undefined : onNext}
        onBack={onBack}
      />
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
