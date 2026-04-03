import { Badge, Group, Stack, Text } from "@mantine/core";
import Breadcrumbs from "../components/Breadcrumbs";
import { type Project, projects } from "../mock-data/projects";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import KubeBadge from "../components/KubeBadge";

export function meta() {
  return [{ title: "Projects – magos" }];
}

const columns: ColumnDef<Project>[] = [
  {
    key: "name",
    label: "Name",
    sortField: "name",
    render: (p) => (
      <Text size="sm" fw={500}>
        {p.name}
      </Text>
    ),
  },
  {
    key: "description",
    label: "Description",
    render: (p) => (
      <Text size="sm" c="dimmed">
        {p.description}
      </Text>
    ),
  },
  {
    key: "workspaces",
    label: "Workspaces",
    render: (p) =>
      p.workspaceIds.length === 0 ? (
        <Text size="sm" c="dimmed">
          —
        </Text>
      ) : (
        <Badge variant="light" color="magos" size="sm">
          {p.workspaceIds.length}
        </Badge>
      ),
  },
  {
    key: "namespace",
    label: "Kubernetes Namespace",
    sortField: "namespace",
    render: (p) => <KubeBadge label={p.namespace} />,
  },
];

export default function Projects() {
  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Projects" }]} />
      <Group gap={4} align="center">
        <Text
          size="xl"
          fw={700}
          variant="gradient"
          gradient={{ from: "magos.4", to: "magos.7", deg: 45 }}
          style={{ fontFamily: "monospace", letterSpacing: -0.5 }}
        >
          // isolated blast radiuses
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
      <ResourceList
        items={projects}
        searchKey="name"
        columns={columns}
        toHref={(p) => `/projects/${p.id}`}
        defaultView="row"
        hideViewToggle
      />
    </Stack>
  );
}
