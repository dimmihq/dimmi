/*
 * The flat rows the views are made of, and the headed sections that group
 * them. Rows sit straight on the panel with a rounded highlight on hover;
 * there are no cards around them.
 */
import type { Child } from "widget-jsx/jsx-runtime";

import type { Article, Conversation, Link } from "../content";
import {
  BookIcon,
  CalendarIcon,
  ChatIcon,
  ChevronRightIcon,
  ExternalIcon,
  LinkIcon,
  MailIcon,
  PhoneIcon,
  QuestionIcon,
  VideoIcon,
} from "./icons";

export function Section({
  title,
  more,
  target,
  children,
}: {
  title: string;
  more?: Child;
  target?: string;
  children: Child;
}) {
  return (
    <section className="section" data-target={target}>
      <header className="section-head">
        <h3>{title}</h3>
        {more}
      </header>
      <div className="list">{children}</div>
    </section>
  );
}

export function ConversationRow({ conversation }: { conversation: Conversation }) {
  return (
    <button
      className="row"
      type="button"
      data-action="click->converse"
      data-conversation={conversation.id ?? conversation.number}
      data-unread={String((conversation.unread ?? 0) > 0)}
    >
      <span className="orb" />
      <span className="text">
        <span>
          <span className="number">{conversation.ticket?.reference ?? `#${conversation.number}`}</span>{" "}
          {conversation.ticket?.title ?? conversation.title}
        </span>
        <small>{conversation.preview}</small>
      </span>
      {conversation.ticket ? (
        <span className="pill" data-group={conversation.ticket.state.group}>
          {conversation.ticket.state.label}
        </span>
      ) : null}
      <time>{conversation.ago}</time>
      <span className="unread-dot" />
    </button>
  );
}

export function ArticleRow({ article }: { article: Article }) {
  return (
    <button className="row" type="button" data-action="click->visit" data-open={`article:${article.slug}`}>
      <span className="row-icon">
        <BookIcon />
      </span>
      <span className="text">
        <span>{article.title}</span>
        <small>{article.summary}</small>
      </span>
      <ChevronRightIcon />
    </button>
  );
}

/*
 * linkIcons draws each icon a customer's link may show; an icon this
 * messenger does not know draws as a plain link.
 */
const linkIcons: Record<string, typeof LinkIcon> = {
  calendar: CalendarIcon,
  book: BookIcon,
  mail: MailIcon,
  phone: PhoneIcon,
  chat: ChatIcon,
  video: VideoIcon,
  help: QuestionIcon,
};

function linkIcon(link: Link) {
  const Draw = linkIcons[link.icon] ?? LinkIcon;

  return <Draw />;
}

export function LinkRow({ link }: { link: Link }) {
  return (
    <a className="row" href={link.url} target="_blank" rel="noopener">
      <span className="row-icon">{linkIcon(link)}</span>
      <span className="text">{link.title}</span>
      <ExternalIcon />
    </a>
  );
}
