/*
 * The logo step, whose upload opens the file picker straight away and
 * saves at once, and the knowledge step, which adds a website the AI
 * agent answers from and starts reading it straight away.
 */
import { RiAddLine, RiLinkM } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { LogoMark } from "@/features/settings/logo-row";
import { PicturePicker } from "@/features/settings/picture-picker";
import { useLogo } from "@/features/settings/use-logo";
import { ApiError } from "@/lib/api/client";
import { createSource, sourcesQuery } from "@/lib/api/knowledge-sources";
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
              {url ? "Replace logo" : "Upload logo"}
            </PicturePicker>
            {url ? (
              <Button
                variant="secondary"
                size="small"
                loading={logo.remove.isPending}
                onClick={() => logo.remove.mutate()}
              >
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

/*
 * withScheme puts https:// in front of an address typed without one.
 */
function withScheme(address: string): string {
  const trimmed = address.trim();
  return /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
}

export function KnowledgeStep({ workspace, label, onNext, onBack }: StepProps) {
  const client = useQueryClient();
  const [address, setAddress] = useState("");
  const { data: added = [] } = useQuery(sourcesQuery(workspace));
  const add = useMutation({
    mutationFn: (start: string) =>
      createSource(workspace, {
        type: "website",
        name: new URL(start).host,
        mode: "sync",
        targets: ["agent"],
        settings: { start_url: start },
        schedule: "weekly",
        sync_now: true,
      }),
    onSuccess: async () => {
      setAddress("");
      await client.invalidateQueries({ queryKey: ["workspace", workspace, "knowledge"] });
    },
  });
  const failure =
    add.error instanceof ApiError
      ? (add.error.fields["settings.start_url"] ?? add.error.fields.start_url ?? add.error.message)
      : add.error
        ? "The site could not be added. Check the address and try again."
        : undefined;
  const submit = (event: FormEvent) => {
    event.preventDefault();
    try {
      add.mutate(withScheme(address));
    } catch {
      // new URL refused the address; the server explains below.
    }
  };

  return (
    <div>
      <StepHeader
        label={label}
        title="Add your knowledge sources"
        description="Point dimmi at your docs, help center or website, and the AI agent answers from them."
      />
      <form className="mt-8 flex items-end gap-2" onSubmit={submit} aria-busy={add.isPending}>
        <Input
          className="flex-1"
          label="Website or docs address"
          placeholder="docs.acme.com"
          leadingIcon={RiLinkM}
          inputDir="ltr"
          value={address}
          onChange={(value) => {
            setAddress(value);
            add.reset();
          }}
          isInvalid={Boolean(failure)}
          errorMessage={failure}
        />
        <Button
          type="submit"
          variant="secondary"
          leadingIcon={RiAddLine}
          loading={add.isPending}
          disabled={address.trim() === ""}
        >
          Add
        </Button>
      </form>
      {added.length > 0 ? (
        <ul className="mt-4 flex flex-col gap-1">
          {added.map((source) => (
            <li key={source.id} className="text-body-2-regular text-text-secondary">
              {source.name} · {source.status === "syncing" || source.status === "queued" ? "reading pages" : "added"}
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-2 text-body-2-regular text-text-tertiary">
          dimmi reads the site's sitemap, or follows its links, and keeps it in sync every week.
        </p>
      )}
      <StepActions
        primary={added.length > 0 ? onNext : undefined}
        primaryLabel="Continue"
        skipLabel={added.length > 0 ? undefined : "Continue without knowledge"}
        onSkip={added.length > 0 ? undefined : onNext}
        onBack={onBack}
      />
    </div>
  );
}
