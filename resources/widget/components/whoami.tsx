/*
 * Who the visitor is talking to, at the top of a conversation: the team's
 * faces when the workspace shows them, otherwise its logo or initial, then
 * its name, the AI mark while the agent is the one answering, and how soon
 * the team usually replies once that is known.
 */
import type { Content } from "../content";
import { type AiTexts, aiTexts, initials } from "../text";
import { AiBadge } from "./ai-badge";
import { Faces } from "./faces";

function CompanyMark({ content }: { content: Content }) {
  if (content.logo) {
    return <img className="company-mark" src={content.logo} alt="" />;
  }

  return <span className="company-mark">{initials(content.company)}</span>;
}

export function Whoami({ content, ai = aiTexts }: { content: Content; ai?: AiTexts }) {
  return (
    <span className="whoami" data-target="whoami">
      {content.team.length > 0 ? <Faces team={content.team} className="small" /> : <CompanyMark content={content} />}
      <span className="text">
        <span className="whoami-name">
          <strong>{content.company}</strong>
          {content.ai ? <AiBadge texts={ai} /> : null}
        </span>
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
