/*
 * The help view: a search field, the collections, and the most read
 * articles. Typing searches every article the messenger holds, by title,
 * summary and the words of the article itself, and shows the matches in
 * place of the collections.
 */
import { Bar } from "../components/chrome";
import { BookIcon, ChevronRightIcon, SearchIcon } from "../components/icons";
import { ArticleRow, Section } from "../components/rows";
import { Loadable, SkeletonRows, SkeletonSection } from "../components/skeleton";
import type { Article, Block, Content } from "../content";
import type { Options } from "../options";

/*
 * words is everything in an article a search looks through: its title,
 * its summary and its own text.
 */
function words(article: Article): string {
  const text = (block: Block) => [block.text ?? "", ...(block.items ?? [])].join(" ");

  return [article.title, article.summary, ...article.body.map(text)].join(" ").toLowerCase();
}

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
          <ul className="list" data-target="browse">
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
          <ul className="list" data-target="results" hidden>
            {content.articles.map((article) => (
              <li data-target="hit" data-words={words(article)} hidden key={article.slug}>
                <ArticleRow article={article} />
              </li>
            ))}
          </ul>
          <p className="none" data-target="none" hidden>
            {texts.search_none}
          </p>
          {popular.length > 0 && (
            <Section title={texts.popular} target="browse">
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
