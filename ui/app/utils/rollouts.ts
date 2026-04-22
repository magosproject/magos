import type { LabelSelector, Phase, RolloutStep } from "../api/types";
import { PHASE } from "./phases";
import type { StepStatus } from "../components/RolloutStepCard";

export function selectorLabels(selector?: LabelSelector): Record<string, string> {
  return selector?.matchLabels ?? {};
}

export function groupStepsByLabels(steps: RolloutStep[]): number[][] {
  const toKey = (labels: Record<string, string>) =>
    JSON.stringify(Object.entries(labels).sort((a, b) => a[0].localeCompare(b[0])));

  const groups: number[][] = [];
  const seen = new Map<string, number>();

  for (let i = 0; i < steps.length; i++) {
    const key = toKey(selectorLabels(steps[i].selector));
    const existingGroup = seen.get(key);
    if (existingGroup === undefined) {
      seen.set(key, groups.length);
      groups.push([i]);
      continue;
    }

    groups[existingGroup].push(i);
  }

  return groups;
}

export function rolloutStepStatus(
  index: number,
  currentStep: number,
  phase: Phase | "" | undefined,
  groups: number[][]
): StepStatus {
  if (phase === PHASE.Applied) return "completed";

  const stepGroupIdx = groups.findIndex((group) => group.includes(index));
  const currentGroupIdx = groups.findIndex((group) => group.includes(currentStep));

  if (stepGroupIdx < currentGroupIdx) return "completed";
  if (stepGroupIdx === currentGroupIdx) {
    if (phase === PHASE.Reconciling) return "active";
    if (phase === PHASE.Failed) return "failed";
  }

  return "pending";
}

export function completedRolloutSteps(
  totalSteps: number,
  currentStep: number,
  phase: Phase | "" | undefined
): number {
  return phase === PHASE.Applied ? totalSteps : currentStep;
}

export function isRolloutStepComplete(index: number, currentStep: number, phase: Phase | ""): boolean {
  return index < currentStep || phase === PHASE.Applied;
}

export function isRolloutStepActive(index: number, currentStep: number, phase: Phase | ""): boolean {
  return phase === PHASE.Reconciling && index === currentStep;
}

export function isRolloutStepFailed(index: number, currentStep: number, phase: Phase | ""): boolean {
  return phase === PHASE.Failed && index === currentStep;
}
