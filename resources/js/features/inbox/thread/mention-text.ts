/*
 * A note's mention tokens as the thread renders them: each <@mem_...> or
 * <@team_...> becomes a link in a scheme of its own, which the message
 * body draws as an @Name chip.
 */

export const mentionScheme = "mention:";

interface Named {
  id: string;
  name: string;
}

/*
 * withMentions turns each mention token into a chip link naming who it
 * mentions, @teammate or @team when the note does not say.
 */
export function withMentions(text: string, mentions: Named[]): string {
  return text.replaceAll(/<@((?:mem|team)_[0-9A-Za-z]{1,40})>/g, (_, id: string) => {
    const named = mentions.find((mention) => mention.id === id)?.name;
    const name = (named ?? (id.startsWith("team_") ? "team" : "teammate")).replaceAll(/[[\]\\]/g, "");

    return `[@${name}](${mentionScheme}${id})`;
  });
}
