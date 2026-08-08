import { describe, expect, it } from "vitest";

import { serializeServerTableParams } from "./serialize-server-table-params";
import type { ServerTableState } from "../types/data-table";

describe("serializeServerTableParams", () => {
  it("serializes pagination", () => {
    const params = serializeServerTableParams({
      pagination: { page: 2, perPage: 25 },
      sorting: [],
      columnFilters: [],
    });

    expect(params).toEqual({
      "pagination[page]": "2",
      "pagination[perPage]": "25",
    });
  });

  it("serializes sorting as JSON", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [{ id: "last_name", desc: true }],
      columnFilters: [],
    });

    expect(JSON.parse(params.sorting!)).toEqual([
      { id: "last_name", desc: true },
    ]);
  });

  it("serializes simple text columnFilters with default iLike operator", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [
        { id: "full_name", value: "izet Molla", variant: "text" },
      ],
    });

    const parsed = JSON.parse(params.columnFilters!);
    expect(parsed).toEqual([
      {
        id: "full_name",
        value: "izet Molla",
        variant: "text",
        operator: "iLike",
      },
    ]);
  });

  it("serializes simple dateRange columnFilters with isBetween operator", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [
        {
          id: "created_at",
          value: ["1700000000000", "1701000000000"],
          variant: "dateRange",
        },
      ],
    });

    const parsed = JSON.parse(params.columnFilters!);
    expect(parsed).toEqual([
      {
        id: "created_at",
        values: ["1700000000000", "1701000000000"],
        variant: "dateRange",
        operator: "isBetween",
      },
    ]);
  });

  it("serializes simple multiSelect columnFilters with default inArray operator", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [
        { id: "status", value: ["active"], variant: "multiSelect" },
      ],
    });

    const parsed = JSON.parse(params.columnFilters!);
    expect(parsed[0].operator).toBe("inArray");
  });

  it("serializes simple columnFilters with multi-select values", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [
        { id: "status", value: ["active", "pending"], variant: "multiSelect" },
      ],
    });

    const parsed = JSON.parse(params.columnFilters!);
    expect(parsed).toEqual([
      {
        id: "status",
        values: ["active", "pending"],
        variant: "multiSelect",
        operator: "inArray",
      },
    ]);
  });

  it("serializes advanced filters with values array and filterFlag", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [],
      joinOperator: "and",
      filters: [
        {
          id: "roles",
          value: ["admin", "editor", "guest"],
          variant: "multiSelect",
          operator: "inArray",
          filterId: "f-roles",
        },
        {
          id: "last_name",
          value: "smith",
          variant: "text",
          operator: "iLike",
          filterId: "f-last",
          joinOperator: "and",
        },
      ],
    });

    expect(params.filterFlag).toBe("advancedFilters");
    expect(params.joinOperator).toBe("and");

    const parsed = JSON.parse(params.filters!);
    expect(parsed).toEqual([
      {
        id: "roles",
        variant: "multiSelect",
        operator: "inArray",
        values: ["admin", "editor", "guest"],
      },
      {
        id: "last_name",
        variant: "text",
        operator: "iLike",
        value: "smith",
        joinOperator: "and",
      },
    ]);
    expect(parsed[0]).not.toHaveProperty("filterId");
    expect(parsed[1]).not.toHaveProperty("filterId");
  });

  it("serializes isEmpty advanced filter without value fields", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [],
      filters: [
        {
          id: "email",
          value: "",
          variant: "text",
          operator: "isEmpty",
          filterId: "f-email",
        },
      ],
    });

    const parsed = JSON.parse(params.filters!);
    expect(parsed[0]).toEqual({
      id: "email",
      variant: "text",
      operator: "isEmpty",
    });
    expect(parsed[0]).not.toHaveProperty("filterId");
    expect(parsed[0]).not.toHaveProperty("value");
    expect(parsed[0]).not.toHaveProperty("values");
  });

  it("serializes filter groups with nested conditions", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [],
      joinOperator: "and",
      filters: [
        {
          id: "status",
          value: ["active"],
          variant: "multiSelect",
          operator: "inArray",
          filterId: "f1",
        },
        {
          type: "group",
          groupId: "g1",
          joinOperator: "or",
          filters: [
            {
              id: "full_name",
              value: "izet",
              variant: "text",
              operator: "iLike",
              filterId: "f2",
            },
            {
              id: "roles",
              value: ["admin"],
              variant: "multiSelect",
              operator: "inArray",
              filterId: "f3",
              joinOperator: "and",
            },
          ],
        },
      ],
    });

    const parsed = JSON.parse(params.filters!);
    expect(parsed[1]).toEqual({
      type: "group",
      joinOperator: "or",
      filters: [
        {
          id: "full_name",
          variant: "text",
          operator: "iLike",
          value: "izet",
        },
        {
          id: "roles",
          variant: "multiSelect",
          operator: "inArray",
          values: ["admin"],
          joinOperator: "and",
        },
      ],
    });
    expect(parsed[1]).not.toHaveProperty("groupId");
  });

  it("does not include advanced keys when filters are absent", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [],
    } satisfies ServerTableState);

    expect(params.filters).toBeUndefined();
    expect(params.filterFlag).toBeUndefined();
    expect(params.joinOperator).toBeUndefined();
  });
});

describe("frontend/backend filter contract", () => {
  it("produces params ExtractQuery can consume for advanced filters", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [],
      joinOperator: "and",
      filters: [
        {
          id: "roles",
          value: ["admin"],
          variant: "multiSelect",
          operator: "inArray",
          filterId: "f1",
        },
      ],
    });

    expect(params.filterFlag).toBe("advancedFilters");
    expect(params.joinOperator).toBe("and");

    const filters = JSON.parse(params.filters!) as Array<{
      id: string;
      values?: string[];
      operator: string;
      variant: string;
    }>;

    expect(filters[0]?.id).toBe("roles");
    expect(filters[0]?.values).toEqual(["admin"]);
    expect(filters[0]?.operator).toBe("inArray");
    expect(filters[0]).not.toHaveProperty("filterId");
  });
});
