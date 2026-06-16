import type { Phase } from "../api/types";

export const PHASE = {
  Pending: "Pending",
  Reconciling: "Reconciling",
  Ready: "Ready",
  Idle: "Idle",
  Planning: "Planning",
  Planned: "Planned",
  Applying: "Applying",
  Applied: "Applied",
  Failed: "Failed",
  ValidationFailed: "ValidationFailed",
  Rejected: "Rejected",
  Deleting: "Deleting",
} as const satisfies Record<string, Phase>;

export const RECONCILABLE_PHASES = new Set<Phase>([
  PHASE.Applied,
  PHASE.Failed,
  PHASE.Rejected,
  PHASE.ValidationFailed,
  PHASE.Idle,
]);

export const SPINNING_PHASES = new Set<Phase>([
  PHASE.Reconciling,
  PHASE.Planning,
  PHASE.Applying,
  PHASE.Deleting,
]);

export function isPhase(value: string): value is Phase {
  return Object.values(PHASE).includes(value as Phase);
}

export function isApprovalPending(ws: {
  spec?: { autoApply?: boolean };
  status?: { phase?: string };
  metadata?: { annotations?: Record<string, string | undefined> };
}): boolean {
  if (ws.spec?.autoApply) return false;
  if (ws.status?.phase !== PHASE.Planned) return false;
  const annotations = ws.metadata?.annotations;
  if (annotations && "magosproject.io/approval-decision" in annotations) return false;
  return true;
}
