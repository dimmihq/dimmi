/*
 * The bar that floats over a settings page while it holds unsaved
 * changes: what is pending, or why the last save failed, with Discard
 * and Save. Settings change only when Save is pressed, never on the
 * click that changed them. It goes in a SettingsPage's footer.
 *
 * It moves the BoardUI way: it condenses into place from a slight
 * scale, fade and blur as it appears (220ms, ease-out), and leaves
 * faster (150ms, ease-in), staying mounted until its exit finishes.
 * Reduced motion shows and hides it without the transition.
 */
import { useEffect, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

/*
 * exitMs is how long the bar takes to leave, kept mounted meanwhile.
 */
const exitMs = 150;

/*
 * usePresence keeps the bar mounted while it leaves and reports when it
 * should be in its shown state, a frame after mounting so the entrance
 * plays.
 */
function usePresence(visible: boolean) {
  const [mounted, setMounted] = useState(visible);
  const [shown, setShown] = useState(false);

  useEffect(() => {
    if (visible) {
      setMounted(true);
      let second = 0;
      const first = requestAnimationFrame(() => {
        second = requestAnimationFrame(() => setShown(true));
      });
      return () => {
        cancelAnimationFrame(first);
        cancelAnimationFrame(second);
      };
    }
    setShown(false);
    const timer = window.setTimeout(() => setMounted(false), exitMs);
    return () => window.clearTimeout(timer);
  }, [visible]);

  return { mounted, shown };
}

export function SaveBar({
  dirty,
  saving,
  error,
  onSave,
  onDiscard,
  message = "Unsaved changes",
  saveLabel = "Save",
  discardLabel = "Discard",
  canSave = dirty,
  hidden = false,
}: {
  dirty: boolean;
  saving: boolean;
  error?: string | null;
  onSave: () => void;
  onDiscard?: () => void;
  message?: string;
  saveLabel?: string;
  discardLabel?: string;
  canSave?: boolean;
  hidden?: boolean;
}) {
  const { mounted, shown } = usePresence(!hidden && (dirty || Boolean(error)));
  if (!mounted) {
    return null;
  }

  return (
    <div
      className={cx(
        "flex origin-bottom transform-gpu items-center gap-3 rounded-2xl border border-border-button-default bg-background-primary-default py-2 ps-4 pe-2 shadow-dropdown will-change-[opacity,transform,filter]",
        "transition-[opacity,transform,filter] motion-reduce:transition-none",
        shown
          ? "scale-100 opacity-100 blur-0 duration-[220ms] ease-[cubic-bezier(0.22,1,0.36,1)]"
          : "pointer-events-none scale-95 opacity-0 blur-[2px] duration-150 ease-in",
      )}
    >
      <span
        role={error ? "alert" : "status"}
        className={
          error
            ? "min-w-0 flex-1 truncate text-body-2-regular text-text-error-primary"
            : "min-w-0 flex-1 truncate text-body-2-regular text-text-secondary"
        }
      >
        {error ?? message}
      </span>
      {onDiscard ? (
        <Button size="small" variant="secondary" disabled={saving} onClick={onDiscard}>
          {discardLabel}
        </Button>
      ) : null}
      <Button size="small" loading={saving} disabled={!canSave} onClick={onSave}>
        {saveLabel}
      </Button>
    </div>
  );
}
