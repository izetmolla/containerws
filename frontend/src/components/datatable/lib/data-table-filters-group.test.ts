import { describe, expect, it } from "vitest";

import { getValidAdvancedFilterEntries } from "./data-table";
import { normalizeAdvancedFilterJoinOperators } from "./advanced-filters";
import { serializeServerTableParams } from "./serialize-server-table-params";

describe("getValidAdvancedFilterEntries with groups", () => {
  it("keeps groups with valid nested filters for API serialization", () => {
    const entries = [
      {
        id: "status",
        value: ["active"],
        variant: "multiSelect" as const,
        operator: "inArray" as const,
        filterId: "f1",
      },
      {
        type: "group" as const,
        groupId: "g1",
        joinOperator: "and" as const,
        filters: [
          {
            id: "email",
            value: "molla",
            variant: "text" as const,
            operator: "iLike" as const,
            filterId: "f2",
          },
          {
            id: "email",
            value: "pollo",
            variant: "text" as const,
            operator: "iLike" as const,
            filterId: "f3",
            joinOperator: "or" as const,
          },
        ],
      },
    ];

    const valid = getValidAdvancedFilterEntries(entries);
    expect(valid).toHaveLength(2);

    const params = serializeServerTableParams({
      pagination: { page: 1, perPage: 10 },
      sorting: [],
      columnFilters: [],
      joinOperator: "and",
      filters: normalizeAdvancedFilterJoinOperators(valid, "and"),
    });

    const parsed = JSON.parse(params.filters!) as unknown[];
    expect(parsed).toHaveLength(2);
    expect(parsed[1]).toMatchObject({
      type: "group",
      joinOperator: "and",
      filters: [
        { id: "email", operator: "iLike", value: "molla" },
        { id: "email", operator: "iLike", value: "pollo", joinOperator: "or" },
      ],
    });
  });
});
