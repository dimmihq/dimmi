import { createFileRoute } from "@tanstack/react-router";

import { SourcePage } from "@/features/knowledge/source-page";

export const Route = createFileRoute("/d/settings/knowledge/$source")({
  component: KnowledgeSourceRoute,
});

function KnowledgeSourceRoute() {
  const { source } = Route.useParams();

  return <SourcePage key={source} source={source} />;
}
