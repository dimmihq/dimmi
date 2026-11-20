/*
 * How WhatsApp numbers look in settings: WhatsApp's brand colour for
 * their mark, and Meta's quality rating in words and a colour.
 */

export const whatsappGreen = "#25D366";

/*
 * qualityLook names a number's quality rating as Meta reports it.
 */
export function qualityLook(rating: string): { label: string; color: "lime" | "yellow" | "rose" | "neutral" } {
  switch (rating) {
    case "GREEN":
      return { label: "High quality", color: "lime" };
    case "YELLOW":
      return { label: "Medium quality", color: "yellow" };
    case "RED":
      return { label: "Low quality", color: "rose" };
    default:
      return { label: "Quality pending", color: "neutral" };
  }
}
