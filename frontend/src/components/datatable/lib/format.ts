import { format } from "date-fns";

/**
 * Format a date to a readable string
 * @param date - Date object or timestamp
 * @param formatStr - Optional format string (default: "MMM dd, yyyy")
 * @returns Formatted date string
 */
export function formatDate(date: Date | number, formatStr: string = "MMM dd, yyyy"): string {
  const dateObj = typeof date === "number" ? new Date(date) : date;
  if (isNaN(dateObj.getTime())) {
    return "";
  }
  return format(dateObj, formatStr);
}