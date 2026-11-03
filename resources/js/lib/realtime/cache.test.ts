import { appendRecord, upsertRecord } from "./cache";

const row = { id: "conv_1", status: "open", subject: "Refund" };
const change = { id: "conv_1", status: "closed" };

describe("upsertRecord", () => {
  it("merges the record into the row with its id in a plain list", () => {
    const patched = upsertRecord([row, { id: "conv_2" }], change);

    expect(patched.found).toBe(true);
    expect(patched.data[0]).toEqual({ id: "conv_1", status: "closed", subject: "Refund" });
  });

  it("patches the rows of an API collection and of every page", () => {
    expect(upsertRecord({ data: [row], meta: {} }, change).data).toEqual({
      data: [{ ...row, status: "closed" }],
      meta: {},
    });

    const pages = upsertRecord({ pages: [{ data: [{ id: "conv_9" }] }, { data: [row] }], pageParams: [] }, change);
    expect(pages.found).toBe(true);
    expect(pages.data.pages[1]?.data[0]).toEqual({ ...row, status: "closed" });
  });

  it("merges into a single record with the same id", () => {
    expect(upsertRecord(row, change)).toEqual({ data: { ...row, status: "closed" }, found: true });
  });

  it("leaves data without the record untouched", () => {
    const list = [{ id: "conv_2" }];

    expect(upsertRecord(list, change)).toEqual({ data: list, found: false });
    expect(upsertRecord({ id: "conv_2" }, change).found).toBe(false);
    expect(upsertRecord({ pages: [{ data: list }] }, change).found).toBe(false);
    expect(upsertRecord({ data: list }, change).found).toBe(false);
    expect(upsertRecord(null, change).found).toBe(false);
  });
});

describe("appendRecord", () => {
  const message = { id: "msg_2" };

  it("adds the record at the end of a timeline", () => {
    expect(appendRecord([{ id: "msg_1" }], message).data).toEqual([{ id: "msg_1" }, message]);
    expect(appendRecord({ data: [] }, message).data).toEqual({ data: [message] });
    expect(appendRecord({ pages: [{ data: [] }, { data: [] }] }, message).data).toEqual({
      pages: [{ data: [] }, { data: [message] }],
    });
  });

  it("refuses a shape it cannot append to", () => {
    expect(appendRecord({ id: "conv_1" }, message).found).toBe(false);
    expect(appendRecord({ pages: [{ id: "x" }] }, message).found).toBe(false);
    expect(appendRecord("text", message).found).toBe(false);
  });
});
