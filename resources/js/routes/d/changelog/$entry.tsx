import { createFileRoute } from "@tanstack/react-router";

import { EntryEditor } from "@/features/changelog/entry-editor";

export const Route = createFileRoute("/d/changelog/$entry")({
  component: function EntryRoute() {
    const { entry } = Route.useParams();

    return <EntryEditor key={entry} id={entry} />;
  },
});
