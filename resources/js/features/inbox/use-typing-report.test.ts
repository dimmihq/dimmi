import { reporter, typingInterval } from "./use-typing-report";

describe("reporter", () => {
  it("sends a start at most once per interval and one stop after it", () => {
    const sent: string[] = [];
    let clock = 10_000;
    const report = reporter(
      (state) => sent.push(state),
      () => clock,
    );

    report(true);
    clock += 1000;
    report(true);
    clock += typingInterval;
    report(true);
    report(false);
    report(false);

    expect(sent).toEqual(["start", "start", "stop"]);
  });

  it("sends no stop when typing never started", () => {
    const sent: string[] = [];
    reporter((state) => sent.push(state))(false);

    expect(sent).toEqual([]);
  });
});
