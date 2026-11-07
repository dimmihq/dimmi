import { authjs } from "./authjs";
import { betterAuth } from "./better-auth";
import { defineDimmi } from "./config";
import { createPagesHandler, createTokenRoute, identityToken } from "./server";

vi.mock("next/headers", () => ({
  headers: async () => new Headers({ cookie: "session=1" }),
  cookies: async () => ({ getAll: () => [] }),
}));

const secret = "dmi_idsec_0123456789abcdefghijklmnopqrstuvwxyzABCDEFG";

function claims(token: string | null) {
  return JSON.parse(Buffer.from(String(token).split(".")[1], "base64url").toString());
}

describe("next.js identity", () => {
  it("signs a token for the signed-in user from a server action or a route", async () => {
    const config = defineDimmi({ identify: () => ({ id: "u1", email: "a@b.test" }), secret });
    expect(claims(await identityToken(config)).sub).toBe("u1");
    const res = await createTokenRoute(config)();
    expect(res.headers.get("Cache-Control")).toBe("no-store, private");
    expect(claims((await res.json()).token).email).toBe("a@b.test");
    const sent: { headers: Record<string, string>; body?: string } = { headers: {} };
    await createPagesHandler(defineDimmi({ identify: () => null, secret }))(
      {},
      {
        setHeader: (name, value) => {
          sent.headers[name] = value;
        },
        status: () => ({
          send: (body: string) => {
            sent.body = body;
          },
        }),
      },
    );
    expect(sent.body).toBe('{"token":null}');
    expect(sent.headers["Cache-Control"]).toBe("no-store, private");
  });

  it("maps Auth.js sessions, trusting the email only when extend says so", async () => {
    const identify = authjs(async () => ({ user: { id: "u2", email: "c@d.test", name: "Cy", verified: true } }), {
      extend: (session) => ({ emailVerified: session.user?.verified === true, attributes: { plan: "pro" } }),
    });
    expect(await identify()).toEqual({
      id: "u2",
      email: "c@d.test",
      emailVerified: true,
      name: "Cy",
      avatarUrl: undefined,
      attributes: { plan: "pro" },
    });
    await expect(authjs(async () => ({ user: { email: "x@y.test" } }))()).rejects.toThrow(/identify_user_invalid/);
    expect(await authjs(async () => null)()).toBeNull();
  });

  it("maps Better Auth sessions with the active organization as the company", async () => {
    const auth = {
      api: {
        getSession: async ({ headers }: { headers: Headers }) =>
          headers.get("cookie")
            ? { user: { id: "u3", email: "e@f.test", emailVerified: true }, session: { activeOrganizationId: "org_1" } }
            : null,
      },
    };
    const user = await betterAuth(auth)();
    expect(user?.company).toEqual({ id: "org_1" });
    expect(user?.emailVerified).toBe(true);
    expect(await betterAuth(auth, { extend: () => null })()).toBeNull();
  });
});
