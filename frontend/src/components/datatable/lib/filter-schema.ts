import { z } from "zod";

import { dataTableConfig } from "../config/data-table";

const sortingItemSchema = z.object({
  id: z.string(),
  desc: z.boolean(),
});

export { sortingItemSchema };

const joinOperatorSchema = z.enum(dataTableConfig.joinOperators);

const filterConditionSchema = z.object({
  id: z.string(),
  value: z.union([z.string(), z.array(z.string())]).optional(),
  variant: z.enum(dataTableConfig.filterVariants),
  operator: z.enum(dataTableConfig.operators),
  filterId: z.string(),
  joinOperator: joinOperatorSchema.optional(),
});

const filterGroupSchema = z.object({
  type: z.literal("group"),
  groupId: z.string(),
  joinOperator: joinOperatorSchema.optional(),
  filters: z.array(filterConditionSchema).min(1),
});

export const filterEntrySchema = z.union([filterGroupSchema, filterConditionSchema]);

export type FilterItemSchema = z.infer<typeof filterConditionSchema>;
export type FilterGroupSchema = z.infer<typeof filterGroupSchema>;
export type FilterEntrySchema = z.infer<typeof filterEntrySchema>;

export { filterConditionSchema, filterGroupSchema, joinOperatorSchema };
