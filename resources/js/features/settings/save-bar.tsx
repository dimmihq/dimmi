/*
 * The bar that floats over a settings page while it holds unsaved
 * changes: what is pending, or why the last save failed, with Discard
 * and Save. Settings change only when Save is pressed, never on the
 * click that changed them. It goes in a SettingsPage's footer.
 */
import { Button } from "@/components/base/buttons/button";

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
  if (hidden || !(dirty || error)) {
    return null;
  }

  return (
    <div className="flex items-center gap-3 rounded-2xl border border-border-button-default bg-background-primary-default py-2 ps-4 pe-2 shadow-dropdown">
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
