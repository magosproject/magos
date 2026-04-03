export interface Variable {
  key: string;
  value?: string;
  category: "terraform" | "env";
  sensitive?: boolean;
  valueFrom?: {
    secretRef: {
      name: string;
      key: string;
    };
  };
}

export interface VariableSet {
  id: string;
  name: string;
  namespace: string;
  projectRef?: string; // ID of the project it belongs to (optional, can be attached directly to workspace)
  variables: Variable[];
}

export const variableSets: VariableSet[] = [
  {
    id: "vs-lz-aws",
    name: "global-aws-config",
    namespace: "landingzone-team",
    projectRef: "p-lz",
    variables: [
      { key: "aws_region", value: "eu-west-1", category: "terraform" },
      {
        key: "AWS_ACCESS_KEY_ID",
        category: "env",
        sensitive: true,
        valueFrom: { secretRef: { name: "aws-credentials", key: "access_key_id" } },
      },
      {
        key: "AWS_SECRET_ACCESS_KEY",
        category: "env",
        sensitive: true,
        valueFrom: { secretRef: { name: "aws-credentials", key: "secret_access_key" } },
      },
    ],
  },
  {
    id: "vs-ml-aws",
    name: "ml-aws-credentials",
    namespace: "ml-team",
    projectRef: "p-ml",
    variables: [
      { key: "aws_region", value: "us-east-1", category: "terraform" },
      { key: "gpu_instance_type", value: "p4d.24xlarge", category: "terraform" },
      {
        key: "AWS_ACCESS_KEY_ID",
        category: "env",
        sensitive: true,
        valueFrom: { secretRef: { name: "ml-aws-credentials", key: "access_key_id" } },
      },
    ],
  },
  {
    id: "vs-ml-hf",
    name: "huggingface-token",
    namespace: "ml-team",
    variables: [
      {
        key: "HF_TOKEN",
        category: "env",
        sensitive: true,
        valueFrom: { secretRef: { name: "hf-credentials", key: "token" } },
      },
    ],
  },
  {
    id: "vs-data-sf",
    name: "snowflake-config",
    namespace: "data-team",
    projectRef: "p-data",
    variables: [
      { key: "snowflake_account", value: "xy12345.eu-west-1", category: "terraform" },
      {
        key: "SNOWFLAKE_USER",
        category: "env",
        sensitive: true,
        valueFrom: { secretRef: { name: "snowflake-credentials", key: "username" } },
      },
      {
        key: "SNOWFLAKE_PASSWORD",
        category: "env",
        sensitive: true,
        valueFrom: { secretRef: { name: "snowflake-credentials", key: "password" } },
      },
    ],
  },
  {
    id: "vs-prod-db",
    name: "shared-db-config",
    namespace: "product-team",
    projectRef: "p-prod",
    variables: [
      { key: "db_host", value: "rds-cluster-prod.magos.internal", category: "terraform" },
      {
        key: "DB_PASSWORD",
        category: "env",
        sensitive: true,
        valueFrom: { secretRef: { name: "db-credentials", key: "password" } },
      },
    ],
  },
  {
    id: "vs-prod-stripe",
    name: "stripe-api-keys",
    namespace: "product-team",
    variables: [
      {
        key: "STRIPE_SECRET_KEY",
        category: "env",
        sensitive: true,
        valueFrom: { secretRef: { name: "stripe-credentials", key: "secret_key" } },
      },
    ],
  },
];
