/*
 * Pages the visitor opens from a tab: a help collection's articles, an
 * article, and a changelog entry. Articles and entries are long reads, so
 * their views are marked to expand the panel when they open; the messenger
 * shrinks it again when the visitor leaves.
 */
import type { Child } from "widget-jsx/jsx-runtime";

import { BackBar } from "../components/chrome";
import { Feedback, Prose } from "../components/prose";
import { ArticleRow } from "../components/rows";
import { Loadable, SkeletonArticle, SkeletonRows } from "../components/skeleton";
import type { Article, Collection, Content, Entry } from "../content";
import type { Options } from "../options";
import { initials } from "../text";

function Page({ name, expand = false, children }: { name: string; expand?: boolean; children: Child }) {
  return (
    <div className="view" data-view={name} data-expand={expand} data-load>
      {children}
    </div>
  );
}

function CollectionPage({ collection, content }: { collection: Collection; content: Content }) {
  const articles = collection.articles
    .map((slug) => content.articles.find((article) => article.slug === slug))
    .filter((article): article is Article => Boolean(article));

  return (
    <>
      <BackBar title={collection.title} />
      <div className="stack">
        <p className="lead">{collection.description}</p>
        <Loadable skeleton={<SkeletonRows count={3} />}>
          <div className="list">
            {articles.map((article) => (
              <ArticleRow article={article} key={article.slug} />
            ))}
          </div>
        </Loadable>
      </div>
    </>
  );
}

/*
 * ArticlePage shows an article, with a link to the full help center when
 * the messenger links to one; the messenger opens it signed in for a
 * visitor it knows.
 */
function ArticlePage({ article, options, url }: { article: Article; options: Options; url?: string }) {
  return (
    <>
      <BackBar title={options.texts.help} />
      <article className="page">
        <Loadable skeleton={<SkeletonArticle />}>
          <h1>{article.title}</h1>
          <p className="byline">
            <span className="face">{initials(article.author)}</span>
            <span>{article.author}</span>
            <span>·</span>
            <span>{article.updated}</span>
          </p>
          <Prose blocks={article.body} />
          {url ? (
            <p className="byline">
              <a
                href={`${url}/articles/${article.slug}`}
                target="_blank"
                rel="noopener noreferrer"
                data-help-article={article.slug}
              >
                {options.texts.open_in_help_center}
              </a>
            </p>
          ) : null}
          <Feedback options={options} prompt={options.texts.helpful} article={article} />
        </Loadable>
      </article>
    </>
  );
}

function EntryPage({ entry, options }: { entry: Entry; options: Options }) {
  return (
    <>
      <BackBar title={options.texts.changelog} />
      <article className="page">
        <Loadable skeleton={<SkeletonArticle />}>
          {entry.tag ? <em className="badge">{entry.tag}</em> : null}
          <h1>{entry.title}</h1>
          <p className="byline">
            <time>{entry.date}</time>
          </p>
          <Prose blocks={entry.body} />
          <Feedback options={options} prompt={options.texts.update_feedback} />
        </Loadable>
      </article>
    </>
  );
}

/*
 * Pages renders every page the content can open, up front: the sample's
 * in a preview, and in a connected messenger what the server sent for the
 * Help and What's new tabs.
 */
export function Pages({ options, content }: { options: Options; content: Content }) {
  return (
    <>
      {content.collections.map((collection) => (
        <Page name={`collection:${collection.slug}`} key={collection.slug}>
          <CollectionPage collection={collection} content={content} />
        </Page>
      ))}
      {content.articles.map((article) => (
        <Page name={`article:${article.slug}`} expand key={article.slug}>
          <ArticlePage article={article} options={options} url={content.help_url} />
        </Page>
      ))}
      {content.changelog.map((entry, index) => (
        <Page name={`entry:${index}`} expand key={entry.title}>
          <EntryPage entry={entry} options={options} />
        </Page>
      ))}
    </>
  );
}
