import { ApiError, request } from "./client";

function respond(status: number, body?: unknown) {
  const text = body === undefined ? null : typeof body === "string" ? body : JSON.stringify(body);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(text, { status })));
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("request", () => {
  it("sends JSON to the API with the session cookie and returns the data", async () => {
    respond(201, { data: { user: { name: "Ada" } } });

    const data = await request<{ user: { name: string } }>("POST", "/users", { name: "Ada" });

    expect(data.user.name).toBe("Ada");
    const [url, init] = vi.mocked(fetch).mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/users");
    expect(init.credentials).toBe("same-origin");
    expect(init.body).toBe('{"name":"Ada"}');
    expect((init.headers as Record<string, string>)["Content-Type"]).toBe("application/json");
  });

  it("returns nothing for a response without a body", async () => {
    respond(204);

    await expect(request("DELETE", "/sessions/current")).resolves.toBeUndefined();
  });

  it("turns the error envelope into an ApiError with the first message of each field", async () => {
    respond(422, {
      code: "validation_failed",
      message: "The given data was invalid.",
      errors: { password: ["The password is too common.", "Another."] },
    });

    const error = await request("POST", "/users", {}).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 422,
      code: "validation_failed",
      fields: { password: "The password is too common." },
    });
  });

  it("still fails with an ApiError when the body is not the envelope", async () => {
    respond(502, "<html>Bad gateway</html>");

    const error = await request("GET", "/me").catch((e: unknown) => e);

    expect(error).toMatchObject({ status: 502, code: "unexpected_response" });
  });
});
