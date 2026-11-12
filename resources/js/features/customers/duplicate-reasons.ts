/*
 * Why two customer records look like the same person, in words, by the
 * reason codes the duplicates list answers with.
 */
export const duplicateReasons: Record<string, string> = {
  same_email: "same email",
  same_phone: "same phone",
  same_whatsapp: "same WhatsApp number",
  same_name_company: "same name and company",
};
