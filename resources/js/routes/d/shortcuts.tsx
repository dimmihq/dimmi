import { createFileRoute } from "@tanstack/react-router";

import { ShortcutPractice } from "@/features/inbox/keyboard/practice";

export const Route = createFileRoute("/d/shortcuts")({
  component: ShortcutPractice,
});
