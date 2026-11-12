/*
 * The frame every broadcast page sits in: a header that names it, says
 * where it stands and holds what can be done now, the page itself, and
 * beside it the message as people get it. The preview steps aside while
 * the message is being written, since the writer already shows it as it
 * arrives. Where there is no room beside the page, the preview opens in
 * a drawer from the header.
 */
import { RiArrowLeftSLine, RiCloseLine, RiEyeLine } from "@remixicon/react";
import { Link } from "@tanstack/react-router";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { type ReactNode, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";
import type { BroadcastStatus } from "@/lib/api/broadcasts";

import { BroadcastStatusChip, MessagePreview, type PreviewMessage } from "./parts";

const settle = [0.22, 1, 0.36, 1] as const;

/*
 * Header holds the broadcast's name and state on the start side, its
 * steps in the middle and what can be done now on the end side. On
 * a narrower page the steps drop to a row of their own under it.
 */
function Header({
  name,
  status,
  note,
  steps,
  children,
}: {
  name: string;
  status: BroadcastStatus;
  note: string;
  steps: ReactNode;
  children: ReactNode;
}) {
  return (
    <header className="grid min-h-12 shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 ps-2.5 pe-2.5 pt-[7px] @3xl:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]">
      <div className="flex min-w-0 items-center gap-2">
        <Link
          to="/d/broadcasts"
          aria-label="Back to broadcasts"
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
        </Link>
        <h1 className="min-w-0 truncate text-body-medium text-text-primary">{name}</h1>
        <BroadcastStatusChip status={status} />
        {note ? <span className="hidden truncate text-body-2-regular text-text-tertiary sm:inline">{note}</span> : null}
      </div>
      {steps ? (
        <div className="order-last col-span-2 flex justify-center @3xl:order-none @3xl:col-span-1">{steps}</div>
      ) : null}
      <div className="flex shrink-0 items-center justify-end gap-2">{children}</div>
    </header>
  );
}

function PreviewPanel({
  message,
  details,
  className,
  onClose,
}: {
  message: PreviewMessage;
  details: ReactNode;
  className?: string;
  onClose?: () => void;
}) {
  return (
    <Panel label="Preview" ground className={className}>
      <PageHeader title="Preview" level={2}>
        <span className="text-body-2-regular text-text-tertiary">With sample details</span>
        {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close preview" onClick={onClose} /> : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <div className="flex flex-col gap-6 pt-2">
          <MessagePreview broadcast={message} />
          {details}
        </div>
      </ScrollArea>
    </Panel>
  );
}

/*
 * Beside eases the preview pane in and out from the end of the page, so
 * the writing step gets the whole width and the others get it back
 * without a jump.
 */
function Beside({ shown, children }: { shown: boolean; children: ReactNode }) {
  const still = useReducedMotion();
  const timing = still ? { duration: 0 } : { duration: 0.32, ease: settle };

  return (
    <AnimatePresence initial={false}>
      {shown ? (
        <motion.div
          key="preview"
          initial={{ width: 0, opacity: 0, filter: "blur(4px)" }}
          animate={{ width: 412, opacity: 1, filter: "blur(0px)" }}
          exit={{ width: 0, opacity: 0, filter: "blur(4px)" }}
          transition={timing}
          className="flex h-full shrink-0 justify-end overflow-hidden"
        >
          {children}
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}

/*
 * FrameProps are the frame's parts: the header's name, state, note,
 * steps and actions, the message to preview, details under it, whether
 * the preview steps aside, and the page.
 */
export interface FrameProps {
  name: string;
  status: BroadcastStatus;
  note: string;
  steps?: ReactNode;
  actions: ReactNode;
  message: PreviewMessage;
  details?: ReactNode;
  writing?: boolean;
  children: ReactNode;
}

export function Frame({ name, status, note, steps, actions, message, details, writing = false, children }: FrameProps) {
  const beside = useWide(1280);
  const [peeking, setPeeking] = useState(false);

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1">
      <Panel label="Broadcast" ground className="@container h-full min-w-0 flex-1">
        <Header name={name} status={status} note={note} steps={steps}>
          {beside ? null : (
            <Button variant="secondary" size="small" leadingIcon={RiEyeLine} onClick={() => setPeeking(true)}>
              Preview
            </Button>
          )}
          {actions}
        </Header>
        <ScrollArea ground>{children}</ScrollArea>
      </Panel>
      {beside ? (
        <Beside shown={!writing}>
          <PreviewPanel message={message} details={details} className="h-full w-[400px]" />
        </Beside>
      ) : (
        <Drawer
          isOpen={peeking}
          onOpenChange={setPeeking}
          placement="end"
          size="md"
          aria-label="Preview"
          className="p-0"
        >
          <PreviewPanel message={message} details={details} className="h-full" onClose={() => setPeeking(false)} />
        </Drawer>
      )}
    </div>
  );
}
