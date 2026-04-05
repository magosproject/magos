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
