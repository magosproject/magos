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
  Deleting: "Deleting",
} as const satisfies Record<string, Phase>;

export const RECONCILABLE_PHASES = new Set<Phase>([PHASE.Applied, PHASE.Failed, PHASE.Idle]);

export const SPINNING_PHASES = new Set<Phase>([
  PHASE.Reconciling,
  PHASE.Planning,
  PHASE.Applying,
  PHASE.Deleting,
]);

export function isPhase(value: string): value is Phase {
  return Object.values(PHASE).includes(value as Phase);
}
