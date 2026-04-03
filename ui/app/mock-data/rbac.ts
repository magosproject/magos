import { type UserRole } from "./user";

export interface RbacUser {
  id: string;
  name: string;
  email: string;
  role: UserRole;
  groupIds: string[];
}

export interface Group {
  id: string;
  name: string;
  description: string;
  projectIds: string[];
  workspaceIds: string[];
}

export const groups: Group[] = [
  {
    id: "g-0001",
    name: "Platform Engineers",
    description: "Full access to cloud infrastructure and platform services.",
    projectIds: ["p-lz"],
    workspaceIds: [],
  },
  {
    id: "g-0002",
    name: "Data Engineers",
    description: "Access to data platform workspaces.",
    projectIds: ["p-data"],
    workspaceIds: [],
  },
  {
    id: "g-0003",
    name: "ML Engineers",
    description: "Access to machine learning infrastructure.",
    projectIds: ["p-ml"],
    workspaceIds: [],
  },
  {
    id: "g-0004",
    name: "Security Team",
    description: "Access to security workspaces.",
    projectIds: [],
    workspaceIds: ["ws-lz-sec"],
  },
  {
    id: "g-0005",
    name: "DevOps",
    description: "Cross-cutting access to all environments for operational tasks.",
    projectIds: ["p-lz", "p-ml", "p-data", "p-prod"],
    workspaceIds: [],
  },
  {
    id: "g-0006",
    name: "Backend Developers",
    description: "Access to product APIs and application workspaces.",
    projectIds: ["p-prod"],
    workspaceIds: ["ws-prod-auth", "ws-prod-pay"],
  },
];

export const rbacUsers: RbacUser[] = [
  {
    id: "u-0001",
    name: "Ramon",
    email: "ramon@magos.dev",
    role: "admin",
    groupIds: ["g-0001", "g-0002", "g-0003", "g-0004", "g-0005"],
  },
  {
    id: "u-0002",
    name: "Alice",
    email: "alice@magos.dev",
    role: "user",
    groupIds: ["g-0001", "g-0005"],
  },
  {
    id: "u-0003",
    name: "Bob",
    email: "bob@magos.dev",
    role: "user",
    groupIds: ["g-0002", "g-0003"],
  },
  {
    id: "u-0004",
    name: "Carol",
    email: "carol@magos.dev",
    role: "user",
    groupIds: ["g-0004"],
  },
  {
    id: "u-0005",
    name: "Dave",
    email: "dave@magos.dev",
    role: "user",
    groupIds: [],
  },
  {
    id: "u-0006",
    name: "Eva",
    email: "eva@magos.dev",
    role: "user",
    groupIds: ["g-0002", "g-0005"],
  },
  {
    id: "u-0007",
    name: "Frank",
    email: "frank@contractor.dev",
    role: "user",
    groupIds: ["g-0006"],
  },
  {
    id: "u-0008",
    name: "Grace",
    email: "grace@magos.dev",
    role: "user",
    groupIds: ["g-0001", "g-0004"],
  },
];
