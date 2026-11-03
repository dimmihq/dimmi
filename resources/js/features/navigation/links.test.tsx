import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { fireEvent, render, screen } from "@testing-library/react";

import { ButtonLink, LinkButton } from "./links";

beforeEach(() => {
  vi.spyOn(window, "scrollTo").mockReturnValue(undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

function setup() {
  const root = createRootRoute();
  const home = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: () => (
      <>
        <LinkButton to="/login">Sign in</LinkButton>
        <LinkButton href="/auth/github/redirect">GitHub</LinkButton>
        <LinkButton>Resend</LinkButton>
        <ButtonLink to="/d">Open the app</ButtonLink>
        <ButtonLink href="/download">Download</ButtonLink>
      </>
    ),
  });
  const page = (path: string, text: string) =>
    createRoute({ getParentRoute: () => root, path, component: () => <p>{text}</p> });
  const router = createRouter({
    routeTree: root.addChildren([home, page("/login", "Login page"), page("/d", "App page")]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  render(<RouterProvider router={router} />);

  return router;
}

describe("links", () => {
  it("renders a router link for a route and a plain anchor for an address", async () => {
    setup();

    expect((await screen.findByRole("link", { name: "Sign in" })).getAttribute("href")).toBe("/login");
    expect(screen.getByRole("link", { name: "GitHub" }).getAttribute("href")).toBe("/auth/github/redirect");
    expect(screen.getByRole("button", { name: "Resend" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Open the app" }).getAttribute("href")).toBe("/d");
    expect(screen.getByRole("link", { name: "Download" }).getAttribute("href")).toBe("/download");
  });

  it("changes page inside the app when given a route", async () => {
    const router = setup();

    expect(fireEvent.click(await screen.findByRole("link", { name: "Sign in" }))).toBe(false);
    expect(await screen.findByText("Login page")).toBeTruthy();
    expect(router.state.location.pathname).toBe("/login");
  });

  it("leaves a plain anchor to the browser", async () => {
    setup();

    expect(fireEvent.click(await screen.findByRole("link", { name: "GitHub" }))).toBe(true);
    expect(fireEvent.click(screen.getByRole("link", { name: "Download" }))).toBe(true);
  });
});
