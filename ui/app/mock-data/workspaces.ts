export type WorkspaceStatus = "active" | "provisioning" | "error";

export interface Workspace {
  id: string;
  name: string;
  namespace: string;
  environment: string;
  status: WorkspaceStatus;
  repoHost: string;
  repoSlug: string;
  path: string;
  serviceAccount: string;
  reconciliationEnabled: boolean;
  autoApply: boolean;
  variableSetRefs?: string[];
}

export const workspaces: Workspace[] = [
  {
    id: "ws-lz-net",
    name: "aws-networking",
    namespace: "landingzone-team",
    environment: "production",
    status: "active",
    repoHost: "https://github.com",
    repoSlug: "magos-org/landingzone-infra",
    path: "terraform/networking",
    serviceAccount: "lz-deploy-sa",
    reconciliationEnabled: true,
    autoApply: true,
  },
  {
    id: "ws-lz-iam",
    name: "aws-iam",
    namespace: "landingzone-team",
    environment: "production",
    status: "active",
    repoHost: "https://github.com",
    repoSlug: "magos-org/landingzone-infra",
    path: "terraform/iam",
    serviceAccount: "lz-deploy-sa",
    reconciliationEnabled: true,
    autoApply: true,
  },
  {
    id: "ws-lz-sec",
    name: "aws-security",
    namespace: "landingzone-team",
    environment: "production",
    status: "provisioning",
    repoHost: "https://github.com",
    repoSlug: "magos-org/landingzone-infra",
    path: "terraform/security",
    serviceAccount: "lz-deploy-sa",
    reconciliationEnabled: false,
    autoApply: false,
  },
  {
    id: "ws-ml-gpu",
    name: "gpu-cluster",
    namespace: "ml-team",
    environment: "production",
    status: "active",
    repoHost: "https://gitlab.com",
    repoSlug: "magos-org/ml-platform",
    path: "terraform/eks-gpu",
    serviceAccount: "ml-deploy-sa",
    reconciliationEnabled: true,
    autoApply: false,
  },
  {
    id: "ws-ml-train",
    name: "model-training-pipeline",
    namespace: "ml-team",
    environment: "production",
    status: "active",
    repoHost: "https://gitlab.com",
    repoSlug: "magos-org/ml-platform",
    path: "terraform/training-pipeline",
    serviceAccount: "ml-deploy-sa",
    reconciliationEnabled: true,
    autoApply: true,
    variableSetRefs: ["vs-ml-hf"],
  },
  {
    id: "ws-data-sf",
    name: "snowflake-cluster",
    namespace: "data-team",
    environment: "production",
    status: "active",
    repoHost: "https://github.com",
    repoSlug: "magos-org/data-platform",
    path: "terraform/snowflake",
    serviceAccount: "data-deploy-sa",
    reconciliationEnabled: true,
    autoApply: false,
  },
  {
    id: "ws-data-af",
    name: "airflow-infrastructure",
    namespace: "data-team",
    environment: "production",
    status: "error",
    repoHost: "https://github.com",
    repoSlug: "magos-org/data-platform",
    path: "terraform/airflow",
    serviceAccount: "data-deploy-sa",
    reconciliationEnabled: true,
    autoApply: true,
  },
  {
    id: "ws-prod-auth",
    name: "auth-service",
    namespace: "product-team",
    environment: "production",
    status: "active",
    repoHost: "https://github.com",
    repoSlug: "magos-org/backend-apis",
    path: "terraform/auth-service",
    serviceAccount: "prod-deploy-sa",
    reconciliationEnabled: true,
    autoApply: false,
  },
  {
    id: "ws-prod-pay",
    name: "payment-service",
    namespace: "product-team",
    environment: "production",
    status: "active",
    repoHost: "https://github.com",
    repoSlug: "magos-org/backend-apis",
    path: "terraform/payment-service",
    serviceAccount: "prod-deploy-sa",
    reconciliationEnabled: true,
    autoApply: false,
    variableSetRefs: ["vs-prod-stripe"],
  },
];
