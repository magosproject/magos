import { Anchor, Group, Stack, Text } from "@mantine/core";
import { IconBox, IconFolder, IconHexagon } from "@tabler/icons-react";
import { useLoaderData } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import { type ResourceCardProps } from "../components/ResourceCard";
import StatusBadge from "../components/StatusBadge";
import KubeBadge from "../components/KubeBadge";
import { repoIcon } from "../utils/repoIcon";
import { statusColor } from "../utils/colors";
import apiClient from "../api/client";
import type { Workspace } from "../api/types";
import { useSSEList } from "../hooks/useSSEList";

export function meta() {
  return [{ title: "Workspaces – magos" }];
}

type WorkspaceRow = {
  id: string;
  name: string;
  namespace: string;
  phase: string;
  projectRef: string;
  repoURL: string;
  path: string;
};

export async function clientLoader() {
  const { data } = await apiClient.GET("/apis/magosproject.io/v1alpha1/workspaces");
  return (data ?? []).map(toWorkspaceRow);
}

function toWorkspaceRow(ws: Workspace): WorkspaceRow {
  return {
    id: ws.metadata?.uid ?? `${ws.metadata?.namespace}/${ws.metadata?.name}`,
    name: ws.metadata?.name ?? "",
    namespace: ws.metadata?.namespace ?? "",
    phase: ws.status?.phase ?? "",
    projectRef: ws.spec?.projectRef?.name ?? "",
    repoURL: ws.spec?.source?.repoURL ?? "",
    path: ws.spec?.source?.path ?? "",
  };
}

const columns: ColumnDef<WorkspaceRow>[] = [
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
  {
    key: "namespace",
    label: "Kubernetes Namespace",
    sortField: "namespace",
    render: (ws) => <KubeBadge label={ws.namespace} />,
  },
];

function toCard(ws: WorkspaceRow): ResourceCardProps {
  return {
    to: `/workspaces/${ws.namespace}/${ws.name}`,
    title: ws.name,
    statusColor: statusColor[ws.phase] ?? "gray",
    badges: [{ label: ws.phase, color: statusColor[ws.phase] }],
    meta: [
      { icon: <IconBox size={16} color="gray" />, label: ws.projectRef || "No Project" },
      {
        icon: repoIcon(ws.repoURL),
        label: ws.repoURL.replace(/^https?:\/\//, ""),
        href: ws.repoURL,
      },
      { icon: <IconFolder size={16} color="gray" />, label: ws.path },
      {
        icon: <IconHexagon size={16} color="var(--mantine-color-blue-filled)" />,
        label: <span style={{ color: "var(--mantine-color-blue-text)" }}>{ws.namespace}</span>,
      },
    ],
  };
}

export default function Workspaces() {
  const initial = useLoaderData<typeof clientLoader>();
  const workspaces = useSSEList<Workspace, WorkspaceRow>(
    "/apis/magosproject.io/v1alpha1/workspaces/events",
    initial,
    toWorkspaceRow,
    clientLoader
  );

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Workspaces" }]} />
      <Group justify="space-between" align="center">
        <Group gap={4} align="center">
          <Text
            size="xl"
            fw={700}
            variant="gradient"
            gradient={{ from: "magos.4", to: "magos.7", deg: 45 }}
            style={{ fontFamily: "monospace", letterSpacing: -0.5 }}
          >
            // where states mutate
          </Text>
          <Text
            className="blinking-cursor"
            size="xl"
            fw={700}
            c="magos.5"
            style={{ fontFamily: "monospace" }}
          >
            _
          </Text>
        </Group>
      </Group>
      <ResourceList
        items={workspaces}
        searchKey="name"
        columns={columns}
        toCard={toCard}
        toHref={(ws) => `/workspaces/${ws.namespace}/${ws.name}`}
      />
    </Stack>
  );
}
