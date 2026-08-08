import { describe, expect, it } from "vitest";

import { getFiltersStateParser } from "./parsers";
import { normalizeAdvancedFilterJoinOperators } from "./advanced-filters";
import { serializeServerTableParams } from "./serialize-server-table-params";

/** Exact payload shape the users list API expects for first_name + email group. */
const USER_LIST_GROUP_PAYLOAD = [
  {
    id: "first_name",
    variant: "text" as const,
    operator: "iLike" as const,
    value: "izet",
  },
  {
    type: "group" as const,
    joinOperator: "and" as const,
    filters: [
      {
        id: "email",
        variant: "text" as const,
        operator: "iLike" as const,
        value: "pollogati",
      },
    ],
  },
];

describe("filter groups frontend/backend contract", () => {
  it("serializes first_name + nested email group without UI-only fields", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [],
      joinOperator: "and",
      filters: normalizeAdvancedFilterJoinOperators(
        [
          {
            id: "first_name",
            value: "izet",
            variant: "text",
            operator: "iLike",
            filterId: "f-first",
          },
          {
            type: "group",
            groupId: "g-email",
            joinOperator: "and",
            filters: [
              {
                id: "email",
                value: "pollogati",
                variant: "text",
                operator: "iLike",
                filterId: "f-email",
              },
            ],
          },
        ],
        "and",
      ),
    });

    expect(params.filterFlag).toBe("advancedFilters");
    expect(params.joinOperator).toBe("and");

    const parsed = JSON.parse(params.filters!) as unknown[];
    expect(parsed).toEqual(USER_LIST_GROUP_PAYLOAD);
    expect(parsed[1]).not.toHaveProperty("groupId");
    expect(parsed[1]).not.toHaveProperty("filterId");
  });

  it("round-trips grouped filters from URL state to API payload", () => {
    const urlState = [
      {
        id: "first_name",
        value: "izet",
        variant: "text" as const,
        operator: "iLike" as const,
        filterId: "f-first",
      },
      {
        type: "group" as const,
        groupId: "g-email",
        joinOperator: "and" as const,
        filters: [
          {
            id: "email",
            value: "pollogati",
            variant: "text" as const,
            operator: "iLike" as const,
            filterId: "f-email",
          },
        ],
      },
    ];

    const parser = getFiltersStateParser(["first_name", "email"]);
    const parsed = parser.parse(JSON.stringify(urlState));

    expect(parsed).toHaveLength(2);
    expect(parsed?.[1]).toMatchObject({ type: "group", groupId: "g-email" });

    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [],
      joinOperator: "and",
      filters: normalizeAdvancedFilterJoinOperators(parsed!, "and"),
    });

    expect(JSON.parse(params.filters!)).toEqual(USER_LIST_GROUP_PAYLOAD);
  });

  it("preserves nested joinOperator inside groups", () => {
    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [],
      joinOperator: "and",
      filters: normalizeAdvancedFilterJoinOperators(
        [
          {
            type: "group",
            groupId: "g1",
            joinOperator: "or",
            filters: [
              {
                id: "email",
                value: "a@example.com",
                variant: "text",
                operator: "iLike",
                filterId: "f1",
              },
              {
                id: "email",
                value: "b@example.com",
                variant: "text",
                operator: "iLike",
                filterId: "f2",
                joinOperator: "or",
              },
            ],
          },
        ],
        "and",
      ),
    });

    const parsed = JSON.parse(params.filters!) as Array<{
      type: string;
      joinOperator?: string;
      filters: Array<{ joinOperator?: string }>;
    }>;

    expect(parsed[0]?.type).toBe("group");
    expect(parsed[0]?.joinOperator).toBeUndefined();
    expect(parsed[0]?.filters[1]?.joinOperator).toBe("or");
  });
});
