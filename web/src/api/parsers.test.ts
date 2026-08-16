import { describe, expect, it, vi } from "vitest";
import { pageParser, parser, taskSchema } from "./parsers";

const task = (over: Record<string, unknown> = {}) => ({
  id: "t1", org_id: "o1", project_id: "p1", title: "Ship it",
  status: "todo", position: 1000,
  created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z",
  ...over,
});

describe("parser", () => {
  it("returns the object when it matches the contract", () => {
    expect(parser(taskSchema, "Task")(task()).title).toBe("Ship it");
  });
  it("throws in dev, naming the field and the reason", () => {
    expect(() => parser(taskSchema, "Task")(task({ position: "high" })))
      .toThrow(/Task did not match the API contract.*position/s);
  });
  it("rejects a status outside the enum the server documents", () => {
    expect(() => parser(taskSchema, "Task")(task({ status: "archived" }))).toThrow();
  });
});

describe("pageParser", () => {
  const parse = pageParser(taskSchema, "Tasks");

  it("reads the shared list envelope", () => {
    const out = parse({ items: [task(), task({ id: "t2" })], limit: 20, offset: 0 });
    expect(out.items).toHaveLength(2);
    expect(out.limit).toBe(20);
    expect(out.dropped).toBe(0);
  });

  /**
   * The policy that differs from parser(): one corrupt row must not blank a
   * board of good ones. It is dropped and counted, never thrown.
   */
  it("drops a bad row and keeps the rest of the page", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const out = parse({ items: [task(), { id: "broken" }, task({ id: "t3" })], limit: 20, offset: 0 });
    expect(out.items).toHaveLength(2);
    expect(out.dropped).toBe(1);
    spy.mockRestore();
    warn.mockRestore();
  });

  it("treats a null items array as an empty page", () => {
    expect(parse({ items: null, limit: 20, offset: 0 }).items).toEqual([]);
  });

  /**
   * Beacon once answered a bare {items} on five of nine list endpoints. The
   * envelope is now uniform server-side; this asserts the client would notice
   * if that regressed.
   */
  it("rejects a response missing limit and offset", () => {
    expect(() => parse({ items: [] })).toThrow(/list envelope/);
  });
});
