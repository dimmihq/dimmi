/*
 * Finishing the ChatGPT sign-in on the server.
 */
import { request } from "@/lib/api/client";

/*
 * completeChatGPTConnect hands the code OpenAI returned to the server,
 * which stores the subscription as a new connection.
 */
export function completeChatGPTConnect(workspace: string, answer: { state: string; code: string }) {
  return request<{ key_id: string; account_id: string }>("POST", "/ai/chatgpt/callback", answer, { workspace });
}
