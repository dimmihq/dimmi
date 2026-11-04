/*
 * The shortcut practice page: sample conversations and one prompt at a
 * time. Pressing the key asked for moves on; nothing is sent anywhere.
 */
import { RiCheckboxCircleLine, RiKeyboardLine } from "@remixicon/react";
import { useEffect, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { EmptyState, PageHeader, Panel } from "@/features/shell/page";
import { cx } from "@/utils/cx";

import { advance, practiceSteps, savedStep, saveStep } from "./practice-steps";

const samples = ["Can't log in", "Invoice question", "Feature request: dark mode", "Refund for order 1042"];

export function ShortcutPractice() {
  const [step, setStep] = useState(savedStep);
  const [row, setRow] = useState(0);

  useEffect(() => {
    const press = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) {
        return;
      }
      const next = advance(step, event.key);
      if (next === step) {
        return;
      }
      event.stopImmediatePropagation();
      event.preventDefault();
      if (event.key.toLowerCase() === "j") {
        setRow((r) => Math.min(r + 1, samples.length - 1));
      }
      if (event.key.toLowerCase() === "k") {
        setRow((r) => Math.max(r - 1, 0));
      }
      setStep(next);
      saveStep(next);
    };
    window.addEventListener("keydown", press, { capture: true });

    return () => window.removeEventListener("keydown", press, { capture: true });
  }, [step]);

  const done = step >= practiceSteps.length;

  return (
    <Panel label="Shortcut practice" className="flex-1">
      <PageHeader title="Shortcut practice" />
      <div className="flex flex-col gap-4 px-4 pb-4">
        {done ? (
          <EmptyState
            icon={RiCheckboxCircleLine}
            title="All done"
            description="You know the moves. Press ? whenever you forget."
          />
        ) : (
          <p role="status" className="flex items-center gap-2 text-body-medium text-text-primary">
            <RiKeyboardLine aria-hidden className="size-5 text-foreground-icon-secondary" />
            {practiceSteps[step]?.prompt}
          </p>
        )}
        <ul aria-label="Sample conversations" className="flex flex-col gap-1">
          {samples.map((subject, i) => (
            <li
              key={subject}
              aria-current={i === row ? "true" : undefined}
              className={cx(
                "rounded-xl px-3 py-2 text-body-2-regular text-text-primary",
                i === row ? "bg-background-tertiary-default" : undefined,
              )}
            >
              {subject}
            </li>
          ))}
        </ul>
        <Button
          variant="secondary"
          size="small"
          className="self-start"
          onClick={() => {
            setStep(0);
            saveStep(0);
          }}
        >
          Start over
        </Button>
      </div>
    </Panel>
  );
}
