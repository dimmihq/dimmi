/*
 * The messenger protocol end to end, spoken the way a messenger on a
 * customer's site speaks it: plain HTTP with the page's Origin, a bearer
 * session token, and a plain event stream. A teammate's reply reaches the
 * visitor live, a dropped stream recovers what it missed from its
 * position, and behind a proxy that holds the stream back the visitor
 * still gets the reply by polling.
 */
import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import type { APIRequestContext } from "@playwright/test";

import { Stream } from "./support/eventsource";
import { createAccount, expect, test as it, type Person } from "./support/fixtures";

const base = process.env.E2E_BASE_URL ?? `http://localhost:${process.env.E2E_PORT ?? 18090}`;
const site = "https://shop.example";

/*
 * Desk is the team side: the owner's signed-in API and their workspace.
 */
interface Desk {
  request: APIRequestContext;
  workspace: string;
  app: string;
}

/*
 * openDesk creates the person's account and finds their first widget.
 */
async function openDesk(request: APIRequestContext, person: Person): Promise<Desk> {
  await createAccount(request, person, `Shop ${person.email.split("@")[0]?.split(".").at(-1)}`);
  const me = await request.get("/api/v1/me");
  const workspace: string = (await me.json()).data.workspaces[0].id;
  const widgets = await request.get("/api/v1/widget-settings", { headers: { "Dimmi-Workspace": workspace } });
  expect(widgets.status()).toBe(200);

  return { request, workspace, app: (await widgets.json()).data[0].id };
}

/*
 * reply has the owner answer the conversation from the inbox.
 */
async function reply(desk: Desk, conversation: string, body: string) {
  const response = await desk.request.post(`/api/v1/conversations/${conversation}/replies`, {
    headers: { "Dimmi-Workspace": desk.workspace },
    data: { body },
  });
  expect(response.status(), await response.text()).toBe(201);
}

/*
 * visitor calls the messenger API as a browser on the customer's site.
 */
async function visitor(path: string, token: string, init: RequestInit = {}, through = base) {
  const headers: Record<string, string> = { Origin: site, "Content-Type": "application/json" };
  if (token !== "") {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(`${through}/widget/v1${path}`, { ...init, headers });
  const text = await response.text();

  return { status: response.status, body: text === "" ? {} : JSON.parse(text) };
}

/*
 * startSession starts an anonymous session and a conversation on it.
 */
async function startSession(desk: Desk) {
  const session = await visitor("/sessions", "", { method: "POST", body: JSON.stringify({ app_id: desk.app }) });
  expect(session.status).toBe(201);
  const token: string = session.body.data.token;

  const started = await visitor("/conversations", token, {
    method: "POST",
    body: JSON.stringify({ client_id: crypto.randomUUID(), body: "Export fails" }),
  });
  expect(started.status).toBe(201);

  return { token, contact: session.body.data.contact.id as string, conversation: started.body.data.id as string };
}

/*
 * connectToken asks for a fresh stream token.
 */
async function connectToken(token: string): Promise<string> {
  const issued = await visitor("/realtime-tokens", token, { method: "POST" });
  expect(issued.status).toBe(201);

  return issued.body.data.token;
}

/*
 * bufferingProxy forwards every request to the server but answers only
 * once the whole response is in, the way a proxy that buffers does: an
 * event stream never gets through.
 */
async function bufferingProxy(): Promise<{ url: string; server: Server }> {
  const server = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) {
      chunks.push(chunk as Buffer);
    }

    const headers = Object.fromEntries(
      Object.entries(req.headers).flatMap(([name, value]) =>
        typeof value === "string" && name !== "host" && name !== "connection" ? [[name, value]] : [],
      ),
    );
    const body = chunks.length > 0 ? Buffer.concat(chunks) : undefined;

    try {
      const upstream = await fetch(`${base}${req.url}`, { method: req.method, headers, body });
      const content = Buffer.from(await upstream.arrayBuffer());
      res.writeHead(upstream.status, { "content-type": upstream.headers.get("content-type") ?? "" });
      res.end(content);
    } catch {
      res.destroy();
    }
  });
  await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));

  return { url: `http://127.0.0.1:${(server.address() as AddressInfo).port}`, server };
}

it.describe("messenger protocol", () => {
  it("resumes, signs out and starts over without an identity token", async ({ person, request }) => {
    const desk = await openDesk(request, person);
    const { token } = await startSession(desk);

    const resumed = await visitor("/session", token);
    expect(resumed.status).toBe(200);
    expect(resumed.body.data.token).toBeUndefined();

    expect((await visitor("/session", "dmi_ws_unknown")).body.code).toBe("session_invalid");
    expect((await visitor("/session", token, { method: "DELETE" })).status).toBe(204);
    expect((await visitor("/session", token, { method: "DELETE" })).status).toBe(204);
    expect((await visitor("/session", token)).body.code).toBe("session_revoked");

    const fresh = await visitor("/sessions", "", { method: "POST", body: JSON.stringify({ app_id: desk.app }) });
    expect(fresh.status).toBe(201);
    expect(fresh.body.data.token).not.toBe(token);
  });

  it("delivers a teammate's reply live and recovers what a dropped stream missed", async ({ person, request }) => {
    const desk = await openDesk(request, person);
    const { token, contact, conversation } = await startSession(desk);
    const channel = `widget_contact:${contact}`;

    const live = await Stream.open(base, { token: await connectToken(token) }, { Origin: site });
    await expect.poll(() => live.pushes.some((push) => push.connect)).toBe(true);
    const epoch = live.pushes.find((push) => push.connect)?.connect?.subs?.[channel]?.epoch ?? "";

    const sent = Date.now();
    await reply(desk, conversation, "Looking now");
    await expect.poll(() => live.events(), { timeout: 5000 }).toContain("message.created");
    expect(Date.now() - sent).toBeLessThan(300);

    const offset = live.lastOffset();
    live.close();
    await reply(desk, conversation, "Fixed while you were away");

    const back = await Stream.open(
      base,
      { token: await connectToken(token), subs: { [channel]: { recover: true, offset, epoch } } },
      { Origin: site },
    );
    await expect.poll(() => back.pushes.some((push) => push.connect)).toBe(true);
    const sub = back.pushes.find((push) => push.connect)?.connect?.subs?.[channel];
    expect(sub?.recovered).toBe(true);
    expect(sub?.publications?.map((pub) => pub.data.event)).toContain("message.created");
    back.close();
  });

  it("falls back to polling behind a proxy that holds the stream back", async ({ person, request }) => {
    const desk = await openDesk(request, person);
    const { token, conversation } = await startSession(desk);
    const proxy = await bufferingProxy();

    try {
      const held = await Promise.race([
        Stream.open(proxy.url, { token: await connectToken(token) }, { Origin: site }).then(() => "stream"),
        new Promise((done) => setTimeout(() => done("held"), 2000)),
      ]);
      expect(held).toBe("held");

      const first = await visitor("/updates", token, {}, proxy.url);
      expect(first.status).toBe(200);
      await reply(desk, conversation, "Seen by polling");

      const next = await visitor(`/updates?since=${encodeURIComponent(first.body.meta.cursor)}`, token, {}, proxy.url);
      expect(next.status).toBe(200);
      expect(next.body.data.map((item: { event: string }) => item.event)).toContain("message.created");
    } finally {
      proxy.server.closeAllConnections();
      proxy.server.close();
    }
  });
});
