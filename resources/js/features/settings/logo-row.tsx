/*
 * The workspace's logo, shown to customers in the messenger and in mail.
 * Upload opens the file picker directly and the logo saves at once; the
 * server keeps a small square redrawn from the file.
 */
import { RiImageLine } from "@remixicon/react";

import { Action } from "./controls";
import { Row } from "./layout";
import { PicturePicker } from "./picture-picker";
import { useLogo } from "./use-logo";

/*
 * LogoMark is the logo as a rounded square, or a placeholder without one.
 */
export function LogoMark({ url, className }: { url: string | null; className?: string }) {
  if (url) {
    return <img src={url} alt="Workspace logo" className={className ?? "size-10 rounded-xl object-cover"} />;
  }

  return (
    <span
      aria-hidden
      className={
        className ??
        "flex size-10 items-center justify-center rounded-xl bg-background-tertiary-default text-foreground-icon-tertiary"
      }
    >
      <RiImageLine className="size-5" />
    </span>
  );
}

export function LogoRow({ workspace, url }: { workspace: string; url: string | null }) {
  const logo = useLogo(workspace);

  return (
    <Row
      label="Logo"
      description={logo.failure ?? "Shown in the messenger and in email. PNG, JPEG, WebP or GIF, up to 2 MB."}
    >
      <LogoMark url={url} />
      <PicturePicker isPending={logo.upload.isPending} onPick={logo.pick} onTooLarge={logo.refuse}>
        {logo.upload.isPending ? "Uploading…" : "Upload"}
      </PicturePicker>
      {url ? <Action onPress={() => logo.remove.mutate()}>Remove</Action> : null}
    </Row>
  );
}
