/*
 * Who the visitor is talking to, at the top of a conversation: the team's
 * faces when the workspace shows them, otherwise its logo or initial, then
 * its name and how soon it usually replies once that is known.
 */
import type { Content } from "../content";
import { initials } from "../text";
import { Faces } from "./faces";

function CompanyMark({ content }: { content: Content }) {
  if (content.logo) {
    return <img className="company-mark" src={content.logo} alt="" />;
  }

  return <span className="company-mark">{initials(content.company)}</span>;
}

export function Whoami({ content }: { content: Content }) {
  return (
    <span className="whoami" data-target="whoami">
      {content.team.length > 0 ? <Faces team={content.team} className="small" /> : <CompanyMark content={content} />}
      <span className="text">
        <strong>{content.company}</strong>
        {content.reply_time ? (
          <small>
            <i className="online" />
            {content.reply_time}
          </small>
        ) : null}
      </span>
    </span>
  );
}
