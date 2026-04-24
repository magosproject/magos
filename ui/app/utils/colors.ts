import type { Phase } from "../api/types";
import { isPhase } from "./phases";

export const statusColor: Record<Phase, string> = {
  Pending: "gray",
  Reconciling: "yellow",
  Ready: "magos",
  Idle: "gray",
  Planning: "blue",
  Planned: "cyan",
  Applying: "yellow",
  Applied: "green",
  Failed: "red",
  ValidationFailed: "red",
  Deleting: "orange",
};

export function statusColorFor(status: string): string {
  return isPhase(status) ? statusColor[status] : "gray";
}

export function flashColorVar(status: string): string {
  const color = statusColorFor(status);
  return `color-mix(in srgb, var(--mantine-color-${color}-5) 15%, var(--mantine-color-body))`;
}
