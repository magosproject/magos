import type { CSSProperties } from "react";
import { Anchor, Text } from "@mantine/core";
import { IconBox, IconClock, IconFolder, IconGitBranch } from "@tabler/icons-react";
import { resourceId, resourceName, resourceNamespace } from "../api/resource";
import type { Workspace } from "../api/types";
import type { ColumnDef } from "./ResourceList";
import ResourceCard from "./ResourceCard";
import StatusBadge from "./StatusBadge";
import { flashColorVar, statusColorFor } from "../utils/colors";
import { isPhase, SPINNING_PHASES } from "../utils/phases";
import { commitUrl, revisionUrl } from "../utils/repoUrls";
import { repoIcon } from "../utils/repoIcon";

export type WorkspaceItem = Workspace & { id: string };

export function toWorkspaceItem(ws: Workspace): WorkspaceItem {
  return {
    ...ws,
    id: resourceId(ws),
  };
}

interface WorkspaceCardProps {
  workspace: Workspace;
  borderAll?: boolean;
  flash?: boolean;
}

export default function WorkspaceCard({ workspace, borderAll, flash }: WorkspaceCardProps) {
  const ns = resourceNamespace(workspace);
  const name = resourceName(workspace);
  const phase = workspace.status?.phase ?? "";
  const projectRef = workspace.spec?.projectRef?.name ?? "";
  const wsRepoURL = workspace.spec?.source?.repoURL ?? "";
  const wsPath = workspace.spec?.source?.path ?? "";
  const observedRevision = workspace.status?.observedRevision ?? "";
  const isSHA = observedRevision.length === 40;
  const appliedRef = isSHA ? observedRevision.slice(0, 7) : observedRevision;
  const syncInterval =
    workspace.metadata?.annotations?.["magosproject.io/reconcile-interval"] ?? "3m";
  const badgeColor = statusColorFor(phase);

  const meta = [
    {
      icon: <IconBox size={16} color="gray" />,
      label: projectRef || "No Project",
      to: projectRef ? `/projects/${ns}/${projectRef}` : undefined,
    },
    {
      icon: repoIcon(wsRepoURL),
      label: wsRepoURL.replace(/^https?:\/\//, ""),
      href: wsRepoURL,
    },
    {
      icon: <IconGitBranch size={16} color="gray" />,
      label: appliedRef || "—",
      href: appliedRef
        ? isSHA
          ? commitUrl(wsRepoURL, observedRevision) ?? undefined
          : revisionUrl(wsRepoURL, observedRevision) ?? undefined
        : undefined,
    },
    { icon: <IconFolder size={16} color="gray" />, label: wsPath },
    { icon: <IconClock size={16} color="gray" />, label: `Sync every ${syncInterval}` },
  ];

  return (
    <ResourceCard
      to={`/workspaces/${ns}/${name}`}
      title={name}
      statusColor={badgeColor}
      badges={
        phase
          ? [{ label: phase, color: badgeColor, spinning: isPhase(phase) && SPINNING_PHASES.has(phase) }]
          : []
      }
      meta={meta}
      borderAll={borderAll}
      flashStyle={
        flash ? ({ "--flash-color": flashColorVar(phase) } as CSSProperties) : undefined
      }
    />
  );
}

export const workspaceColumns: ColumnDef<WorkspaceItem>[] = [
  {
    key: "name",
    label: "Name",
    sortValue: (ws) => ws.metadata?.name ?? "",
    render: (ws) => (
      <Text size="sm" fw={500}>
        {ws.metadata?.name}
      </Text>
    ),
  },
  {
    key: "project",
    label: "Project",
    render: (ws) => (
      <Text size="sm" c="dimmed">
        {ws.spec?.projectRef?.name || "—"}
      </Text>
    ),
  },
  {
    key: "phase",
    label: "Status",
    sortValue: (ws) => ws.status?.phase ?? "",
    render: (ws) => <StatusBadge status={ws.status?.phase ?? ""} />,
  },
  {
    key: "repo",
    label: "Repository",
    render: (ws) => {
      const url = ws.spec?.source?.repoURL ?? "";
      return (
        <Anchor href={url} target="_blank" size="sm" onClick={(e) => e.stopPropagation()}>
          {url.replace(/^https?:\/\//, "")}
        </Anchor>
      );
    },
  },
  {
    key: "path",
    label: "Path",
    render: (ws) => (
      <Text size="sm" c="dimmed">
        {ws.spec?.source?.path ?? ""}
      </Text>
    ),
  },
];
