import { Anchor, Group, Stack, Text } from "@mantine/core";
import { IconBox, IconFolder, IconHexagon } from "@tabler/icons-react";
import { type Workspace, workspaces } from "../mock-data/workspaces";
import { projects } from "../mock-data/projects";
import Breadcrumbs from "../components/Breadcrumbs";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import { type ResourceCardProps } from "../components/ResourceCard";
import StatusBadge from "../components/StatusBadge";
import KubeBadge from "../components/KubeBadge";
import { repoIcon } from "../utils/repoIcon";
import { statusColor } from "../utils/colors";

export function meta() {
  return [{ title: "Workspaces – magos" }];
}

const columns: ColumnDef<Workspace>[] = [
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
    render: (ws) => {
      const project = projects.find((p) => p.workspaceIds.includes(ws.id));
      return (
        <Text size="sm" c="dimmed">
          {project?.name ?? "—"}
        </Text>
      );
    },
  },
  {
    key: "status",
    label: "Status",
    sortField: "status",
    render: (ws) => <StatusBadge status={ws.status} />,
  },
  {
    key: "repo",
    label: "Repository",
    render: (ws) => (
      <Anchor
        href={`${ws.repoHost}/${ws.repoSlug}`}
        target="_blank"
        size="sm"
        onClick={(e) => e.stopPropagation()}
      >
        {ws.repoSlug}
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

function toCard(ws: Workspace): ResourceCardProps {
  const project = projects.find((p) => p.workspaceIds.includes(ws.id));

  return {
    to: `/workspaces/${ws.id}`,
    title: ws.name,
    statusColor: statusColor[ws.status] ?? "gray",
    badges: [
      { label: ws.status, color: statusColor[ws.status], spinning: ws.status === "provisioning" },
    ],
    meta: [
      { icon: <IconBox size={16} color="gray" />, label: project?.name ?? "No Project" },
      {
        icon: repoIcon(ws.repoHost),
        label: ws.repoSlug,
        href: `${ws.repoHost}/${ws.repoSlug}`,
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
        toHref={(ws) => `/workspaces/${ws.id}`}
      />
    </Stack>
  );
}
