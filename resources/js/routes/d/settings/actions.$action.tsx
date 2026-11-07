import { createFileRoute } from "@tanstack/react-router";

import { ActionEditorPage } from "@/features/ai/actions/editor/action-editor";

export const Route = createFileRoute("/d/settings/actions/$action")({
  validateSearch: (search: Record<string, unknown>): { connection?: string } =>
    typeof search.connection === "string" ? { connection: search.connection } : {},
  component: ActionRoute,
});

function ActionRoute() {
  const { action } = Route.useParams();
  const { connection } = Route.useSearch();

  return (
    <ActionEditorPage
      key={action === "new" ? `new-${connection ?? ""}` : "saved"}
      action={action}
      connection={connection}
    />
  );
}
