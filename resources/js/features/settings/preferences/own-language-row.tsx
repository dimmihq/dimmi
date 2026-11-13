/*
 * The person's language row with a draft of its own, as the Preferences
 * page gives it, for rendering the row by itself in tests.
 */
import { useLanguageDraft } from "./language-draft";
import { LanguageRow } from "./language-row";

/*
 * OwnLanguageRow is the person's row with a draft of its own, as the
 * Preferences page gives it.
 */
export function OwnLanguageRow() {
  return <LanguageRow draft={useLanguageDraft()} />;
}
