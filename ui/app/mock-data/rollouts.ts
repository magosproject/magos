export type RolloutPhase = "Pending" | "Reconciling" | "Ready" | "Applied" | "Failed";

export interface RolloutStep {
  name: string;
  selector: Record<string, string>;
}

export interface Rollout {
  id: string;
  name: string;
  namespace: string;
  projectRef: string;
  phase: RolloutPhase;
  currentStep: number;
  reason: string;
  message: string;
  steps: RolloutStep[];
}

export const rollouts: Rollout[] = [
  {
    id: "ro-lz",
    name: "Landing Zone",
    namespace: "landingzone-team",
    projectRef: "p-lz",
    phase: "Reconciling",
    currentStep: 1,
    reason: "StepProgressing",
    message: "Executing step 2 of 3: IAM policies",
    steps: [
      { name: "Networking", selector: { "magosproject.io/layer": "networking" } },
      { name: "IAM Policies", selector: { "magosproject.io/layer": "iam" } },
      { name: "Security Baseline", selector: { "magosproject.io/layer": "security" } },
    ],
  },
  {
    id: "ro-ml",
    name: "ML Platform",
    namespace: "ml-team",
    projectRef: "p-ml",
    phase: "Applied",
    currentStep: 2,
    reason: "RolloutCompleted",
    message: "All steps completed successfully",
    steps: [
      { name: "GPU Cluster", selector: { "magosproject.io/component": "gpu" } },
      { name: "Training Pipeline", selector: { "magosproject.io/component": "training" } },
    ],
  },
  {
    id: "ro-data",
    name: "Data Platform",
    namespace: "data-team",
    projectRef: "p-data",
    phase: "Failed",
    currentStep: 1,
    reason: "StepFailed",
    message: "Workspace airflow-infrastructure failed during apply",
    steps: [
      { name: "Snowflake Warehouse", selector: { "magosproject.io/component": "snowflake" } },
      { name: "Airflow Orchestration", selector: { "magosproject.io/component": "airflow" } },
    ],
  },
  {
    id: "ro-prod",
    name: "Product APIs",
    namespace: "product-team",
    projectRef: "p-prod",
    phase: "Pending",
    currentStep: 0,
    reason: "WaitingForWorkspaces",
    message: "Waiting for workspaces matching step 1 selector",
    steps: [
      { name: "Auth Service", selector: { "magosproject.io/service": "auth" } },
      { name: "Payment Service", selector: { "magosproject.io/service": "payment" } },
    ],
  },
];
