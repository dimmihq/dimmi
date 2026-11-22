/*
 * A chip in an agent's guidance is the procedure step's chip, written
 * into the guidance's Markdown as [[kind:ref]], exactly as the server
 * reads it, and read back from it. The server spells each one out by
 * name for the agent, so renaming a procedure or a team never leaves
 * stale words in the guidance.
 */
import { ProcedureToken } from "../automation/procedure-writer/procedure-token";
import { marker } from "../automation/procedure-writer/tokens";

const chip = /^\[\[(action|attribute|article|handoff|tag|ticket_type|sub_procedure):([^\]\s]+)\]\]/;

export const GuidanceToken = ProcedureToken.extend({
  markdownTokenName: "procedureToken",
  markdownTokenizer: {
    name: "procedureToken",
    level: "inline",
    start: (src: string) => src.indexOf("[["),
    tokenize: (src: string) => {
      const found = chip.exec(src);
      if (!found) {
        return undefined;
      }

      return { type: "procedureToken", raw: found[0], kind: found[1], ref: found[2] };
    },
  },
  parseMarkdown: (found, helpers) =>
    helpers.createNode("procedureToken", { kind: found.kind as string, ref: found.ref as string, label: "" }),
  renderMarkdown: (node) => marker({ kind: node.attrs?.kind, ref: String(node.attrs?.ref ?? "") }),
});
