import { createFileRoute } from "@tanstack/react-router";

import { ArticleEditor } from "@/features/docs/article-editor";

export const Route = createFileRoute("/d/docs/$article")({
  component: function ArticleRoute() {
    const { article } = Route.useParams();

    return <ArticleEditor key={article} id={article} />;
  },
});
