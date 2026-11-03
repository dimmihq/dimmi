/*
 * The help view: a search field that narrows the collections as the
 * visitor types, the collections themselves, and the most read articles.
 */
import { Bar } from "../components/chrome";
import { BookIcon, ChevronRightIcon, SearchIcon } from "../components/icons";
import { ArticleRow, Section } from "../components/rows";
import { Loadable, SkeletonRows, SkeletonSection } from "../components/skeleton";
import type { Article, Content } from "../content";
import type { Options } from "../options";

export function Help({ options, content }: { options: Options; content: Content }) {
  const { texts } = options;
  const popular = content.popular
    .map((slug) => content.articles.find((article) => article.slug === slug))
    .filter((article): article is Article => Boolean(article));

  return (
    <>
      <Bar title={texts.help} />
      <div className="stack" data-controller="search">
        <label className="field">
          <SearchIcon />
          <input
            type="search"
            placeholder={texts.search_placeholder}
            aria-label={texts.search_placeholder}
            data-action="input->filter"
          />
        </label>
        <Loadable
          skeleton={
            <>
              <SkeletonRows count={3} />
              <SkeletonSection count={3} />
            </>
          }
        >
          <ul className="list">
            {content.collections.map((collection) => (
              <li data-target="row" key={collection.slug}>
                <button
                  className="row"
                  type="button"
                  data-action="click->visit"
                  data-open={`collection:${collection.slug}`}
                >
                  <span className="row-icon">
                    <BookIcon />
                  </span>
                  <span className="text">
                    <strong>{collection.title}</strong>
                    <small>{collection.articles.length} articles</small>
                  </span>
                  <ChevronRightIcon />
                </button>
              </li>
            ))}
          </ul>
          <p className="none" data-target="none" hidden>
            No results
          </p>
          {popular.length > 0 && (
            <Section title={texts.popular}>
              {popular.map((article) => (
                <ArticleRow article={article} key={article.slug} />
              ))}
            </Section>
          )}
        </Loadable>
      </div>
    </>
  );
}
