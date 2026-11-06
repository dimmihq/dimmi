/*
 * Customer records for the customer tests, written through the API the
 * signed-in page's own session uses, so each test starts from the data a
 * team would already have.
 */
import { expect, type Page } from "@playwright/test";

/*
 * api sends one call in the signed-in person's workspace and returns the
 * data of its answer.
 */
export async function api<T>(page: Page, method: "GET" | "POST" | "PATCH", path: string, data?: unknown): Promise<T> {
  const me = await page.request.get("/api/v1/me");
  const workspace = ((await me.json()) as { data: { workspaces: { id: string }[] } }).data.workspaces[0]?.id ?? "";
  const response = await page.request.fetch(`/api/v1${path}`, {
    method,
    headers: { "Dimmi-Workspace": workspace },
    data,
  });
  expect(response.status(), await response.text()).toBeLessThan(300);

  return ((await response.json()) as { data: T }).data;
}
