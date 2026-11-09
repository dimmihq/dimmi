import { createFileRoute } from "@tanstack/react-router";

import { ArticleEditor } from "@/features/help-center/article-editor";

export const Route = createFileRoute("/d/help-center/$article")({
  component: function ArticleRoute() {
    const { article } = Route.useParams();

    return <ArticleEditor key={article} id={article} />;
  },
});
