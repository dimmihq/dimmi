/*
 * The bold and italics Operator writes inside a sentence, shown as
 * such, so a suggestion's reason never shows its asterisks wherever it
 * is read: on its card, or above the article or procedure under review.
 */
import type { ReactNode } from "react";

export function emphasized(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|\*[^*\s][^*]*\*)/g).map((part, i) => {
    const key = `${i}:${part}`;
    if (part.startsWith("**") && part.endsWith("**") && part.length > 4) {
      return (
        <strong key={key} className="font-medium text-text-primary">
          {part.slice(2, -2)}
        </strong>
      );
    }
    if (part.startsWith("*") && part.endsWith("*") && part.length > 2) {
      return <em key={key}>{part.slice(1, -1)}</em>;
    }
    return part;
  });
}
