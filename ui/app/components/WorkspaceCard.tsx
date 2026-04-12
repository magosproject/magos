import type { CSSProperties } from "react";
import { Anchor, Text } from "@mantine/core";
import { IconBox, IconClock, IconFolder, IconGitBranch } from "@tabler/icons-react";
import { resourceId, resourceName, resourceNamespace } from "../api/resource";
import type { Workspace } from "../api/types";
import type { ColumnDef } from "./ResourceList";
import ResourceCard from "./ResourceCard";
import StatusBadge, { spinningStatuses } from "./StatusBadge";
import { statusColor, flashColorVar } from "../utils/colors";
import { repoIcon } from "../utils/repoIcon";

function commitURL(repoURL: string, sha: string): string {
  const base = repoURL.replace(/\.git$/, "");
  if (base.includes("gitlab")) return `${base}/-/commit/${sha}`;
  if (base.includes("bitbucket")) return `${base}/commits/${sha}`;
  return `${base}/commit/${sha}`;
}

function revisionURL(repoURL: string, revision: string): string | undefined {
  if (!repoURL || !revision) return undefined;
  const base = repoURL.replace(/\.git$/, "");
  if (base.includes("github.com") || base.includes("gitlab.com") || base.includes("gitlab."))
    return `${base}/tree/${revision}`;
  if (base.includes("bitbucket.org")) return `${base}/src/${revision}`;
  return undefined;
}

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
          ? commitURL(wsRepoURL, observedRevision)
          : revisionURL(wsRepoURL, observedRevision)
        : undefined,
    },
    { icon: <IconFolder size={16} color="gray" />, label: wsPath },
    { icon: <IconClock size={16} color="gray" />, label: `Sync every ${syncInterval}` },
  ];

  return (
    <ResourceCard
      to={`/workspaces/${ns}/${name}`}
      title={name}
      statusColor={statusColor[phase] ?? "gray"}
      badges={
        phase
          ? [{ label: phase, color: statusColor[phase] ?? "gray", spinning: spinningStatuses.has(phase) }]
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

