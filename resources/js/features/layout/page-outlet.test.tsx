import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";

import { PageOutlet } from "./page-outlet";

beforeEach(() => {
  vi.spyOn(window, "scrollTo").mockReturnValue(undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

function setup() {
  const root = createRootRoute({ component: () => <PageOutlet className="frame" /> });
  const page = (path: string, text: string) =>
    createRoute({ getParentRoute: () => root, path, component: () => <p>{text}</p> });
  const router = createRouter({
    routeTree: root.addChildren([page("/", "Home"), page("/next", "Next")]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  render(<RouterProvider router={router} />);

  return router;
}

describe("PageOutlet", () => {
  it("shows the first page without a transition", async () => {
    setup();

    const page = (await screen.findByText("Home")).parentElement;
    expect(page?.className).toBe("frame");
  });

  it("reveals every page it navigates to", async () => {
    const router = setup();
    await screen.findByText("Home");

    act(() => router.history.push("/next"));
    expect((await screen.findByText("Next")).parentElement?.className).toBe("animate-page-in frame");

    act(() => router.history.push("/"));
    expect((await screen.findByText("Home")).parentElement?.className).toBe("animate-page-in frame");
  });
});
