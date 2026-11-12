/*
 * The home view: the brand hero with the team and a greeting, the way into
 * a conversation lifted onto its edge, then recent conversations and the
 * customer's links as flat lists. The row with the logo and the expand and
 * close buttons stays pinned while the rest scrolls under it.
 */
import { Tools } from "../components/chrome";
import { Faces } from "../components/faces";
import { ArrowRightIcon, ChevronRightIcon, Mark } from "../components/icons";
import { ConversationRow, LinkRow, Section } from "../components/rows";
import { Loadable, SkeletonSection } from "../components/skeleton";
import type { Content } from "../content";
import { tickets } from "../lib/tickets";
import type { Options } from "../options";

export function Home({ options, content }: { options: Options; content: Content }) {
  const { texts } = options;
  const recent = content.conversations.slice(0, 2);
  const showAll = (
    <button className="link" type="button" data-action="click->select" data-tab="messages">
      {texts.show_all}
      <ChevronRightIcon />
    </button>
  );

  return (
    <>
      <div className="top">
        <span className="tile">
          <Mark />
        </span>
        <Faces team={content.team} className="hero-faces" />
        <Tools />
      </div>
      <header className="hero">
        <h1>
          <span>{texts.greeting_intro}</span>
          {texts.greeting}
        </h1>
      </header>
      <div className="stack lifted">
        <button className="card chat" type="button" data-action="click->converse">
          <span className="text">
            <strong>{texts.chat_title}</strong>
            <small>{texts.chat_body}</small>
          </span>
          <span className="go">
            <ArrowRightIcon />
          </span>
        </button>
        {tickets()?.HomeCards({
          types: content.ticket_types ?? [],
          incidents: content.incidents ?? [],
          texts: options.texts,
        }) ?? null}
        <Loadable
          skeleton={
            <>
              <SkeletonSection count={2} round />
              <SkeletonSection count={2} />
            </>
          }
        >
          {recent.length > 0 && (
            <Section title={texts.recent_messages} more={showAll}>
              {recent.map((conversation) => (
                <ConversationRow conversation={conversation} key={conversation.number} />
              ))}
            </Section>
          )}
          {content.links.length > 0 && (
            <Section title={texts.links}>
              {content.links.map((link) => (
                <LinkRow link={link} key={link.title} />
              ))}
            </Section>
          )}
          {options.show_branding ? (
            <a className="credit" href={options.branding_url} target="_blank" rel="noopener">
              <Mark />
              Powered by dimmi
            </a>
          ) : null}
        </Loadable>
      </div>
    </>
  );
}
