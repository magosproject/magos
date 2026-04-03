import { Badge, Group, Stack, Text } from "@mantine/core";
import Breadcrumbs from "../components/Breadcrumbs";
import KubeBadge from "../components/KubeBadge";
import { type VariableSet, variableSets } from "../mock-data/variable-sets";
import { projects } from "../mock-data/projects";
import ResourceList, { type ColumnDef } from "../components/ResourceList";

export function meta() {
  return [{ title: "Variable Sets – magos" }];
}

const columns: ColumnDef<VariableSet>[] = [
  {
    key: "name",
    label: "Name",
    sortField: "name",
    render: (vs) => (
      <Text size="sm" fw={500}>
        {vs.name}
      </Text>
    ),
  },
  {
    key: "project",
    label: "Project",
    render: (vs) => {
      if (!vs.projectRef) {
        return (
          <Text size="sm" c="dimmed" fs="italic">
            None (Direct attachment)
          </Text>
        );
      }
      const project = projects.find((p) => p.id === vs.projectRef);
      return (
        <Text size="sm" c="dimmed">
          {project?.name ?? vs.projectRef}
        </Text>
      );
    },
  },
  {
    key: "variables",
    label: "Variables",
    render: (vs) => (
      <Badge variant="light" color="magos" size="sm">
        {vs.variables.length}
      </Badge>
    ),
  },
  {
    key: "namespace",
    label: "Kubernetes Namespace",
    sortField: "namespace",
    render: (vs) => <KubeBadge label={vs.namespace} />,
  },
];

export default function VariableSets() {
  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Variable Sets" }]} />
      <Group gap={4} align="center">
        <Text
          size="xl"
          fw={700}
          variant="gradient"
          gradient={{ from: "magos.4", to: "magos.7", deg: 45 }}
          style={{ fontFamily: "monospace", letterSpacing: -0.5 }}
        >
          // hardcoded no more
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
        items={variableSets}
        searchKey="name"
        columns={columns}
        toHref={(vs) => `/variable-sets/${vs.id}`}
        defaultView="row"
        hideViewToggle
      />
    </Stack>
  );
}
