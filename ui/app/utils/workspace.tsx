import { Anchor, Text } from "@mantine/core";
import { IconBox, IconClock, IconFolder } from "@tabler/icons-react";
import type { CSSProperties } from "react";
import type { ColumnDef } from "../components/ResourceList";
import type { ResourceCardProps } from "../components/ResourceCard";
import StatusBadge, { spinningStatuses } from "../components/StatusBadge";
import { repoIcon } from "./repoIcon";
import { statusColor, flashColorVar } from "./colors";
import type { Workspace } from "../api/types";

export type WorkspaceRow = {
  id: string;
  name: string;
  namespace: string;
  phase: string;
  projectRef: string;
  repoURL: string;
  path: string;
  syncInterval: string;
};

export function toWorkspaceRow(ws: Workspace): WorkspaceRow {
  return {
    id: ws.metadata?.uid ?? `${ws.metadata?.namespace}/${ws.metadata?.name}`,
    name: ws.metadata?.name ?? "",
    namespace: ws.metadata?.namespace ?? "",
    phase: ws.status?.phase ?? "",
    projectRef: ws.spec?.projectRef?.name ?? "",
    repoURL: ws.spec?.source?.repoURL ?? "",
    path: ws.spec?.source?.path ?? "",
    syncInterval: ws.metadata?.annotations?.["magosproject.io/reconcile-interval"] ?? "3m",
  };
}

export const workspaceColumns: ColumnDef<WorkspaceRow>[] = [
  {
    key: "name",
    label: "Name",
    sortField: "name",
    render: (ws) => (
      <Text size="sm" fw={500}>
        {ws.name}
      </Text>
    ),
  },
  {
    key: "project",
    label: "Project",
    render: (ws) => (
      <Text size="sm" c="dimmed">
        {ws.projectRef || "—"}
      </Text>
    ),
  },
  {
    key: "phase",
    label: "Status",
    sortField: "phase",
    render: (ws) => <StatusBadge status={ws.phase} />,
  },
  {
    key: "repo",
    label: "Repository",
    render: (ws) => (
      <Anchor href={ws.repoURL} target="_blank" size="sm" onClick={(e) => e.stopPropagation()}>
        {ws.repoURL.replace(/^https?:\/\//, "")}
      </Anchor>
    ),
  },
  {
    key: "path",
    label: "Path",
    render: (ws) => (
      <Text size="sm" c="dimmed">
        {ws.path}
      </Text>
    ),
  },
];

export function workspaceToCard(ws: WorkspaceRow): ResourceCardProps {
  return {
    to: `/workspaces/${ws.namespace}/${ws.name}`,
    title: ws.name,
    statusColor: statusColor[ws.phase] ?? "gray",
    badges: [{ label: ws.phase, color: statusColor[ws.phase], spinning: spinningStatuses.has(ws.phase) }],
    meta: [
      {
        icon: <IconBox size={16} color="gray" />,
        label: ws.projectRef || "No Project",
        to: ws.projectRef ? `/projects/${ws.namespace}/${ws.projectRef}` : undefined,
      },
      {
        icon: repoIcon(ws.repoURL),
        label: ws.repoURL.replace(/^https?:\/\//, ""),
        href: ws.repoURL,
      },
      { icon: <IconFolder size={16} color="gray" />, label: ws.path },
      { icon: <IconClock size={16} color="gray" />, label: `Sync every ${ws.syncInterval}` },
    ],
  };
}

export function workspaceToHref(ws: WorkspaceRow): string {
  return `/workspaces/${ws.namespace}/${ws.name}`;
}

export function workspaceFlashStyle(ws: WorkspaceRow): CSSProperties {
  return { "--flash-color": flashColorVar(ws.phase) } as CSSProperties;
}

