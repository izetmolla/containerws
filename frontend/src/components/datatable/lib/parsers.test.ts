import { describe, expect, it } from "vitest";

import type { ExtendedColumnSort } from "../types/data-table";
import { getFiltersStateParser, getSortingStateParser } from "./parsers";

type SortingRow = { name: string; created_at: string };

describe("getSortingStateParser", () => {
  const parser = getSortingStateParser<SortingRow>(["name", "created_at"]);

  it("parses valid sorting JSON", () => {
    const value = JSON.stringify([
      { id: "name", desc: true },
      { id: "created_at", desc: false },
    ]);

    expect(parser.parse(value)).toEqual([
      { id: "name", desc: true },
      { id: "created_at", desc: false },
    ]);
  });

  it("rejects unknown column ids", () => {
    const value = JSON.stringify([{ id: "unknown", desc: true }]);
    expect(parser.parse(value)).toBeNull();
  });

  it("returns null for invalid JSON", () => {
    expect(parser.parse("not-json")).toBeNull();
  });

  it("round-trips via serialize", () => {
    const sorting: ExtendedColumnSort<SortingRow>[] = [
      { id: "name", desc: false },
    ];
    const serialized = parser.serialize(sorting);
    expect(parser.parse(serialized)).toEqual(sorting);
  });
});

describe("getFiltersStateParser", () => {
  const parser = getFiltersStateParser(["roles", "last_name"]);

  it("parses advanced filter with multi-select value array", () => {
    const value = JSON.stringify([
      {
        id: "roles",
        value: ["admin", "editor"],
        variant: "multiSelect",
        operator: "inArray",
        filterId: "f1",
      },
    ]);

    expect(parser.parse(value)).toEqual([
      {
        id: "roles",
        value: ["admin", "editor"],
        variant: "multiSelect",
        operator: "inArray",
        filterId: "f1",
      },
    ]);
  });

  it("parses text filter with iLike operator", () => {
    const value = JSON.stringify([
      {
        id: "last_name",
        value: "smith",
        variant: "text",
        operator: "iLike",
        filterId: "f2",
      },
    ]);

    expect(parser.parse(value)).toEqual([
      {
        id: "last_name",
        value: "smith",
        variant: "text",
        operator: "iLike",
        filterId: "f2",
      },
    ]);
  });

  it("parses isEmpty filter without value", () => {
    const value = JSON.stringify([
      {
        id: "last_name",
        variant: "text",
        operator: "isEmpty",
        filterId: "f3",
      },
    ]);

    expect(parser.parse(value)).toEqual([
      {
        id: "last_name",
        variant: "text",
        operator: "isEmpty",
        filterId: "f3",
      },
    ]);
  });

  it("parses isEmpty filter with empty string value", () => {
    const value = JSON.stringify([
      {
        id: "last_name",
        value: "",
        variant: "text",
        operator: "isEmpty",
        filterId: "f3b",
      },
    ]);

    expect(parser.parse(value)).toEqual([
      {
        id: "last_name",
        value: "",
        variant: "text",
        operator: "isEmpty",
        filterId: "f3b",
      },
    ]);
  });

  it("accepts filters for columns not in the optional id list (validated server-side)", () => {
    const value = JSON.stringify([
      {
        id: "email",
        value: "a@b.com",
        variant: "text",
        operator: "eq",
        filterId: "f4",
      },
    ]);

    expect(parser.parse(value)).toEqual([
      {
        id: "email",
        value: "a@b.com",
        variant: "text",
        operator: "eq",
        filterId: "f4",
      },
    ]);
  });

  it("rejects invalid operator values", () => {
    const value = JSON.stringify([
      {
        id: "roles",
        value: ["admin"],
        variant: "multiSelect",
        operator: "contains",
        filterId: "f5",
      },
    ]);

    expect(parser.parse(value)).toBeNull();
  });

  it("parses filters when column ids list is empty (before columns load)", () => {
    const parser = getFiltersStateParser([]);
    const value = JSON.stringify([
      {
        id: "roles",
        value: ["admin"],
        variant: "multiSelect",
        operator: "inArray",
        filterId: "f1",
      },
    ]);

    expect(parser.parse(value)).toEqual([
      {
        id: "roles",
        value: ["admin"],
        variant: "multiSelect",
        operator: "inArray",
        filterId: "f1",
      },
    ]);
  });

  it("parses in-progress filters with empty values from URL", () => {
    const value = JSON.stringify([
      {
        id: "last_name",
        value: "",
        variant: "text",
        operator: "iLike",
        filterId: "f7",
      },
    ]);

    expect(parser.parse(value)).toEqual([
      {
        id: "last_name",
        value: "",
        variant: "text",
        operator: "iLike",
        filterId: "f7",
      },
    ]);
  });

  it("parses filter groups from URL", () => {
    const value = JSON.stringify([
      {
        type: "group",
        groupId: "g1",
        filters: [
          {
            id: "roles",
            value: ["admin"],
            variant: "multiSelect",
            operator: "inArray",
            filterId: "f1",
          },
          {
            id: "last_name",
            value: "smith",
            variant: "text",
            operator: "iLike",
            filterId: "f2",
            joinOperator: "and",
          },
        ],
      },
    ]);

    expect(parser.parse(value)).toEqual([
      {
        type: "group",
        groupId: "g1",
        filters: [
          {
            id: "roles",
            value: ["admin"],
            variant: "multiSelect",
            operator: "inArray",
            filterId: "f1",
          },
          {
            id: "last_name",
            value: "smith",
            variant: "text",
            operator: "iLike",
            filterId: "f2",
            joinOperator: "and",
          },
        ],
      },
    ]);
  });

  it("parses user URL payload with status filter and email group", () => {
    const value = JSON.stringify([
      {
        id: "status",
        value: ["active"],
        variant: "multiSelect",
        operator: "inArray",
        filterId: "qxJdFjtm",
      },
      {
        type: "group",
        groupId: "UALXTmcM",
        joinOperator: "and",
        filters: [
          {
            id: "email",
            value: "molla",
            variant: "text",
            operator: "iLike",
            filterId: "5CmIocb1",
          },
          {
            id: "email",
            value: "pollo",
            variant: "text",
            operator: "iLike",
            filterId: "VxbWIZL2",
            joinOperator: "or",
          },
        ],
      },
    ]);

    const parser = getFiltersStateParser([
      "status",
      "email",
      "roles",
      "full_name",
    ]);
    const parsed = parser.parse(value);
    expect(parsed).toHaveLength(2);
    expect(parsed?.[1]).toMatchObject({ type: "group", groupId: "UALXTmcM" });
    if (parsed?.[1] && "filters" in parsed[1]) {
      expect(parsed[1].filters).toHaveLength(2);
    }
  });

  it("round-trips advanced filters via serialize", () => {
    const filters = [
      {
        id: "roles",
        value: ["admin"],
        variant: "multiSelect" as const,
        operator: "inArray" as const,
        filterId: "f6",
      },
    ];

    const serialized = parser.serialize(filters);
    expect(parser.parse(serialized)).toEqual(filters);
  });
});
