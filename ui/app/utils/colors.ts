export const statusColor: Record<string, string> = {
  active: "magos",
  provisioning: "yellow",
  error: "red",

  // Rollout / Workspace phases (from the CRD Phase enum)
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
  return `color-mix(in srgb, var(--mantine-color-${color}-5) 15%, transparent)`;
}

