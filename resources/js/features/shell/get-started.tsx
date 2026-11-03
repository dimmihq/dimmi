/*
 * The first steps of a new workspace, at the foot of the sidebar. They
 * stay unchecked until each feature exists and can tell whether it was
 * set up. The list starts folded, so the sidebar's own pages stay in
 * view on short screens; the person opens it with one click, and the
 * browser remembers.
 */
import { RiArrowDownSLine, RiCheckboxBlankCircleLine } from "@remixicon/react";
import { useState } from "react";

import { cx } from "@/utils/cx";

const steps = [
  "Install the messenger",
  "Connect your email",
  "Invite your team",
  "Connect GitHub",
  "Use the MCP server",
];

const key = "dimmi:get-started-folded";

function folded(): boolean {
  try {
    return window.localStorage.getItem(key) !== "0";
  } catch {
    return true;
  }
}

export function GetStarted() {
  const [isFolded, setFolded] = useState(folded);

  const toggle = () => {
    setFolded(!isFolded);
    try {
      window.localStorage.setItem(key, isFolded ? "0" : "1");
    } catch {
      // Blocked storage keeps the choice for this page only.
    }
  };

  return (
    <section aria-label="Get started" className="flex w-full flex-col rounded-xl bg-background-tertiary-default p-2">
      <button
        type="button"
        aria-expanded={!isFolded}
        onClick={toggle}
        className="flex w-full cursor-pointer items-center justify-between rounded-lg px-1 py-0.5 outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <span className="text-body-medium text-text-primary">Get started</span>
        <span className="flex items-center gap-1 text-caption-1-medium text-text-tertiary">
          0/{steps.length}
          <RiArrowDownSLine
            aria-hidden
            className={cx("size-4 transition-transform duration-200", isFolded && "-rotate-90")}
          />
        </span>
      </button>
      {isFolded ? null : (
        <ul className="mt-1.5 flex flex-col gap-0.5">
          {steps.map((step) => (
            <li key={step} className="flex items-center gap-2 px-1 py-1 text-body-regular text-text-secondary">
              <RiCheckboxBlankCircleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
              {step}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
