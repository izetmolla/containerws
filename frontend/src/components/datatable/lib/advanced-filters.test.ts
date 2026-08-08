import { describe, expect, it } from "vitest";

import { normalizeAdvancedFilterJoinOperators } from "./advanced-filters";

describe("normalizeAdvancedFilterJoinOperators", () => {
  it("strips joinOperator from the first filter", () => {
    const result = normalizeAdvancedFilterJoinOperators([
      {
        id: "status",
        value: ["active"],
        variant: "multiSelect",
        operator: "inArray",
        filterId: "f1",
        joinOperator: "or",
      },
      {
        id: "last_name",
        value: "smith",
        variant: "text",
        operator: "iLike",
        filterId: "f2",
        joinOperator: "and",
      },
    ]);

    expect(result[0]).not.toHaveProperty("joinOperator");
    expect(result[1]?.joinOperator).toBe("and");
  });

  it("applies fallback join operator to filters after the first", () => {
    const result = normalizeAdvancedFilterJoinOperators(
      [
        {
          id: "status",
          value: ["active"],
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
        },
      ],
      "or",
    );

    expect(result[1]).toMatchObject({ joinOperator: "or" });
  });

  it("normalizes join operators inside groups", () => {
    const result = normalizeAdvancedFilterJoinOperators([
      {
        type: "group",
        groupId: "g1",
        filters: [
          {
            id: "full_name",
            value: "izet",
            variant: "text",
            operator: "iLike",
            filterId: "f1",
            joinOperator: "or",
          },
          {
            id: "email",
            value: "test",
            variant: "text",
            operator: "iLike",
            filterId: "f2",
          },
        ],
      },
    ]);

    const group = result[0];
    expect(group).toMatchObject({ type: "group" });
    if (group && "filters" in group) {
      expect(group.filters[0]).not.toHaveProperty("joinOperator");
      expect(group.filters[1]?.joinOperator).toBe("and");
    }
  });
});
