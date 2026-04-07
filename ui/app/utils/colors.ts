import type { Phase } from "../api/types";

export const statusColor: Record<Phase | string, string> = {
  Pending: "gray",
  Reconciling: "yellow",
  Ready: "magos",
  Idle: "gray",
  Planning: "blue",
  Planned: "cyan",
  Applying: "yellow",
  Applied: "green",
  Failed: "red",
  Deleting: "orange",
};

export function flashColorVar(status: string): string {
  const color = statusColor[status] ?? "gray";
  return `color-mix(in srgb, var(--mantine-color-${color}-5) 15%, var(--mantine-color-body))`;
}

