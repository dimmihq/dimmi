/*
 * The settings of a step that answers with a help article: which article,
 * picked from the ones the workspace has published, and the line the
 * workflow says before it. The messenger opens the article in place;
 * every other channel reads it as a link.
 */
import { Row, Section } from "@/features/settings/layout";

import { type EditorProps, Note, Words } from "./field-parts";
import { Pick } from "./pick";
import { text, useSet } from "./step-settings";

export function ArticleFields({ step, lists, onEdit }: EditorProps) {
  const set = useSet(onEdit);

  return (
    <>
      <Section title="Article" description="Customers read it in the messenger, without leaving the conversation.">
        {lists.articles.length === 0 ? (
          <Note>Publish an article in a help center and it can be sent from here.</Note>
        ) : (
          <Row label="Send">
            <Pick
              key={`articles-${lists.articles.length}`}
              label="Article to send"
              choices={lists.articles}
              value={text(step.settings, "article")}
              onChange={(id) => set({ article: id })}
            />
          </Row>
        )}
      </Section>
      <Section title="Before the article" description="Leave it empty to send the article on its own.">
        <Words label="Message" value={text(step.settings, "text")} onChange={(value) => set({ text: value })} />
      </Section>
    </>
  );
}
