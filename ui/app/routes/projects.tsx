import { Badge, Stack, Text } from "@mantine/core";
import { useLoaderData } from "react-router";
import { resourceId, resourceName, resourceNamespace } from "../api/resource";
import Breadcrumbs from "../components/Breadcrumbs";
import PageTagline from "../components/PageTagline";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import { apiUrl } from "../api/base";
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
    id: resourceId(p),
    name: resourceName(p),
    namespace: resourceNamespace(p),
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
];

export default function Projects() {
  const initial = useLoaderData<typeof clientLoader>();
  const [projects, changedIds] = useSSEList<Project, ProjectRow>(
    apiUrl("/apis/magosproject.io/v1alpha1/projects/events"),
    initial,
    toProjectRow,
    clientLoader
  );

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Projects" }]} />
      <PageTagline text="// isolated blast radiuses" />
      <ResourceList
        items={projects}
        searchKey="name"
        columns={columns}
        toHref={(p) => `/projects/${p.namespace}/${p.name}`}
        defaultView="row"
        hideViewToggle
        flashIds={changedIds}
      />
    </Stack>
  );
}
