/*
 * The changelog view: one card per entry, newest first, each opening the
 * full entry.
 */
import { Bar } from "../components/chrome";
import { Mark } from "../components/icons";
import { Loadable, SkeletonCards } from "../components/skeleton";
import type { Content } from "../content";
import type { Options } from "../options";

export function Changelog({ options, content }: { options: Options; content: Content }) {
  return (
    <>
      <Bar title={options.texts.changelog} />
      <div className="stack">
        <Loadable skeleton={<SkeletonCards count={2} />}>
          {content.changelog.map((entry, index) => (
            <button
              className="card entry"
              type="button"
              data-action="click->visit"
              data-open={`entry:${index}`}
              key={entry.title}
            >
              <span className="art">
                <Mark />
              </span>
              <span className="copy">
                <em className="badge">{entry.tag}</em>
                <strong>{entry.title}</strong>
                <small>{entry.summary}</small>
                <time>{entry.date}</time>
              </span>
            </button>
          ))}
        </Loadable>
      </div>
    </>
  );
}
