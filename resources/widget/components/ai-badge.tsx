/*
 * The mark beside the AI agent's name, wherever the agent speaks: under
 * its messages, at the top of a conversation it is answering, and on the
 * preview of a message it sent. It is always shown, so a visitor knows
 * from the first reply that they are not talking to a person.
 */
import { type AiTexts, aiTexts } from "../text";

export function AiBadge({ texts = aiTexts }: { texts?: AiTexts }) {
  return (
    <span className="ai-badge" role="img" aria-label={texts.name} title={texts.name}>
      {texts.label}
    </span>
  );
}
