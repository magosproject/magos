import { Badge, Group, Stack, Text } from "@mantine/core";
import { useLoaderData } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import KubeBadge from "../components/KubeBadge";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import apiClient from "../api/client";

export function meta() {
  return [{ title: "Variable Sets – magos" }];
}

type VariableSetRow = {
  id: string;
  name: string;
  namespace: string;
  conditionCount: number;
};

export async function clientLoader() {
  const { data } = await apiClient.GET("/apis/magosproject.io/v1alpha1/variablesets");
  return (data ?? []).map(
    (vs): VariableSetRow => ({
      id: vs.metadata?.uid ?? `${vs.metadata?.namespace}/${vs.metadata?.name}`,
      name: vs.metadata?.name ?? "",
      namespace: vs.metadata?.namespace ?? "",
      conditionCount: vs.status?.conditions?.length ?? 0,
    })
  );
}

const columns: ColumnDef<VariableSetRow>[] = [
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
    key: "conditions",
    label: "Conditions",
    render: (vs) =>
      vs.conditionCount === 0 ? (
        <Text size="sm" c="dimmed">
          —
        </Text>
      ) : (
        <Badge variant="light" color="magos" size="sm">
          {vs.conditionCount}
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
  const variableSets = useLoaderData<typeof clientLoader>();

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
        toHref={(vs) => `/variable-sets/${vs.namespace}/${vs.name}`}
        defaultView="row"
        hideViewToggle
      />
    </Stack>
  );
}
