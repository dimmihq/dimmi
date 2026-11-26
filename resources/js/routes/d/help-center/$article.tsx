import { createFileRoute } from "@tanstack/react-router";

import { ArticleEditor } from "@/features/help-center/article-editor";
import { TranslationEditor } from "@/features/help-center/translation-editor";

interface ArticleSearch {
  lang?: string;
}

export const Route = createFileRoute("/d/help-center/$article")({
  validateSearch: (search: Record<string, unknown>): ArticleSearch =>
    typeof search.lang === "string" && search.lang !== "" ? { lang: search.lang } : {},
  component: function ArticleRoute() {
    const { article } = Route.useParams();
    const { lang } = Route.useSearch();

    return lang ? (
      <TranslationEditor key={`${article}:${lang}`} id={article} locale={lang} />
    ) : (
      <ArticleEditor key={article} id={article} />
    );
  },
});
