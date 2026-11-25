import { createFileRoute } from "@tanstack/react-router";

import { RequestedArticle } from "@/features/help-center/requested-article";

export const Route = createFileRoute("/d/help-center/requests/$changeset")({
  component: function RequestedArticleRoute() {
    const { changeset } = Route.useParams();

    return <RequestedArticle key={changeset} id={changeset} />;
  },
});
