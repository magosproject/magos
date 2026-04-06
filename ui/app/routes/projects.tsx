import { Badge, Group, Stack, Text } from "@mantine/core";
import { useLoaderData } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import KubeBadge from "../components/KubeBadge";
import apiClient from "../api/client";
import type { Project } from "../api/types";
import { useSSEList } from "../hooks/useSSEList";

export function meta() {
  return [{ title: "Projects – magos" }];
}

type ProjectRow = {
  id: string;
  name: string;
  namespace: string;
  description: string;
  variableSetCount: number;
};

export async function clientLoader() {
  const { data } = await apiClient.GET("/apis/magosproject.io/v1alpha1/projects");
  return (data ?? []).map(toProjectRow);
}

function toProjectRow(p: Project): ProjectRow {
  return {
    id: p.metadata?.uid ?? `${p.metadata?.namespace}/${p.metadata?.name}`,
    name: p.metadata?.name ?? "",
    namespace: p.metadata?.namespace ?? "",
    description: p.spec?.description ?? "",
    variableSetCount: p.spec?.variableSetRef?.length ?? 0,
  };
}

const columns: ColumnDef<ProjectRow>[] = [
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
        {p.description || "—"}
      </Text>
    ),
  },
  {
    key: "variableSets",
    label: "Variable Sets",
    render: (p) =>
      p.variableSetCount === 0 ? (
        <Text size="sm" c="dimmed">
          —
        </Text>
      ) : (
        <Badge variant="light" color="magos" size="sm">
          {p.variableSetCount}
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
  const initial = useLoaderData<typeof clientLoader>();
  const projects = useSSEList<Project, ProjectRow>(
    "/apis/magosproject.io/v1alpha1/projects/events",
    initial,
    toProjectRow,
    clientLoader
  );

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
        toHref={(p) => `/projects/${p.namespace}/${p.name}`}
        defaultView="row"
        hideViewToggle
      />
    </Stack>
  );
}
