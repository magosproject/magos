export type ExecutionStatus = "success" | "failed" | "running" | "pending_approval";

export interface ExecutionLog {
  timestamp: string;
  level: "info" | "warn" | "error";
  message: string;
}

export interface Execution {
  id: string;
  workspaceId: string;
  status: ExecutionStatus;
  triggeredBy: string;
  startedAt: string;
  finishedAt: string | null;
  logs: ExecutionLog[];
}

export interface WorkspaceSchedule {
  workspaceId: string;
  lastInvocation: string;
  nextInvocation: string;
}

export const schedules: WorkspaceSchedule[] = [
  {
    workspaceId: "ws-lz-net",
    lastInvocation: "2026-03-27T06:00:00Z",
    nextInvocation: "2026-03-28T06:00:00Z",
  },
  {
    workspaceId: "ws-ml-gpu",
    lastInvocation: "2026-03-27T06:00:00Z",
    nextInvocation: "2026-03-28T06:00:00Z",
  },
  {
    workspaceId: "ws-data-sf",
    lastInvocation: "2026-03-26T12:00:00Z",
    nextInvocation: "2026-03-27T12:00:00Z",
  },
  {
    workspaceId: "ws-prod-pay",
    lastInvocation: "2026-03-27T04:00:00Z",
    nextInvocation: "2026-03-28T04:00:00Z",
  },
];

export const executions: Execution[] = [
  {
    id: "exec-001",
    workspaceId: "ws-lz-net",
    status: "success",
    triggeredBy: "scheduler",
    startedAt: "2026-03-27T06:00:00Z",
    finishedAt: "2026-03-27T06:02:34Z",
    logs: [
      {
        timestamp: "2026-03-27T06:00:00Z",
        level: "info",
        message: "Initializing Terraform workspace for AWS Networking",
      },
      {
        timestamp: "2026-03-27T06:00:05Z",
        level: "info",
        message: "terraform init completed successfully",
      },
      {
        timestamp: "2026-03-27T06:00:12Z",
        level: "info",
        message: "terraform plan: 2 to add, 0 to change, 0 to destroy",
      },
      { timestamp: "2026-03-27T06:02:30Z", level: "info", message: "terraform apply complete" },
      {
        timestamp: "2026-03-27T06:02:34Z",
        level: "info",
        message: "Execution finished successfully",
      },
    ],
  },
  {
    id: "exec-002",
    workspaceId: "ws-lz-net",
    status: "success",
    triggeredBy: "scheduler",
    startedAt: "2026-03-26T06:00:00Z",
    finishedAt: "2026-03-26T06:01:58Z",
    logs: [
      {
        timestamp: "2026-03-26T06:00:00Z",
        level: "info",
        message: "Initializing Terraform workspace for AWS Networking",
      },
      {
        timestamp: "2026-03-26T06:00:06Z",
        level: "info",
        message: "terraform init completed successfully",
      },
      {
        timestamp: "2026-03-26T06:00:14Z",
        level: "info",
        message: "terraform plan: no changes detected",
      },
      {
        timestamp: "2026-03-26T06:01:58Z",
        level: "info",
        message: "Execution finished successfully",
      },
    ],
  },
  {
    id: "exec-003",
    workspaceId: "ws-data-af",
    status: "failed",
    triggeredBy: "scheduler",
    startedAt: "2026-03-27T05:00:00Z",
    finishedAt: "2026-03-27T05:01:12Z",
    logs: [
      {
        timestamp: "2026-03-27T05:00:00Z",
        level: "info",
        message: "Initializing Terraform workspace",
      },
      {
        timestamp: "2026-03-27T05:00:05Z",
        level: "info",
        message: "terraform init completed successfully",
      },
      {
        timestamp: "2026-03-27T05:00:11Z",
        level: "warn",
        message: "Provider version constraint not pinned",
      },
      {
        timestamp: "2026-03-27T05:00:45Z",
        level: "error",
        message: 'Error: Invalid resource type "helm_release"',
      },
      {
        timestamp: "2026-03-27T05:01:12Z",
        level: "error",
        message: "Execution failed with exit code 1",
      },
    ],
  },
  {
    id: "exec-004",
    workspaceId: "ws-ml-gpu",
    status: "running",
    triggeredBy: "manual",
    startedAt: "2026-03-27T17:55:00Z",
    finishedAt: null,
    logs: [
      {
        timestamp: "2026-03-27T17:55:00Z",
        level: "info",
        message: "Initializing Terraform workspace for ML GPU clusters",
      },
      {
        timestamp: "2026-03-27T17:55:06Z",
        level: "info",
        message: "terraform init completed successfully",
      },
      {
        timestamp: "2026-03-27T17:55:14Z",
        level: "info",
        message: "terraform plan: 5 to add, 1 to change, 2 to destroy",
      },
      {
        timestamp: "2026-03-27T17:55:30Z",
        level: "info",
        message: "terraform apply in progress...",
      },
    ],
  },
  {
    id: "exec-005",
    workspaceId: "ws-prod-pay",
    status: "pending_approval",
    triggeredBy: "git-commit",
    startedAt: "2026-03-27T18:00:00Z",
    finishedAt: null,
    logs: [
      {
        timestamp: "2026-03-27T18:00:00Z",
        level: "info",
        message: "Initializing Terraform workspace for Production Payments",
      },
      {
        timestamp: "2026-03-27T18:00:05Z",
        level: "info",
        message: "terraform init completed successfully",
      },
      {
        timestamp: "2026-03-27T18:00:15Z",
        level: "info",
        message: "terraform plan: 1 to add, 1 to change, 0 to destroy",
      },
      {
        timestamp: "2026-03-27T18:00:20Z",
        level: "info",
        message: "Plan requires approval before apply",
      },
    ],
  },
];
