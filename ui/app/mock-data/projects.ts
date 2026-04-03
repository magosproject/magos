export interface Project {
  id: string;
  name: string;
  namespace: string;
  description: string;
  workspaceIds: string[];
  configMap: Record<string, string>;
}

export const projects: Project[] = [
  {
    id: "p-lz",
    name: "Landing Zone",
    namespace: "landingzone-team",
    description: "Core AWS infrastructure, networking, and global IAM.",
    workspaceIds: ["ws-lz-net", "ws-lz-iam", "ws-lz-sec"],
    configMap: {},
  },
  {
    id: "p-ml",
    name: "ML Platform",
    namespace: "ml-team",
    description: "Machine learning infrastructure and GPU compute clusters.",
    workspaceIds: ["ws-ml-gpu", "ws-ml-train"],
    configMap: {},
  },
  {
    id: "p-data",
    name: "Data Platform",
    namespace: "data-team",
    description: "Data ingestion, warehousing, and orchestration.",
    workspaceIds: ["ws-data-sf", "ws-data-af"],
    configMap: {},
  },
  {
    id: "p-prod",
    name: "Product APIs",
    namespace: "product-team",
    description: "Backend microservices and application infrastructure.",
    workspaceIds: ["ws-prod-auth", "ws-prod-pay"],
    configMap: {},
  },
];
