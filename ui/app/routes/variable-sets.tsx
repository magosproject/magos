import { Badge, Stack, Text } from "@mantine/core";
import { useLoaderData } from "react-router";
import { resourceId, resourceName, resourceNamespace } from "../api/resource";
import Breadcrumbs from "../components/Breadcrumbs";
import PageTagline from "../components/PageTagline";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import { apiUrl } from "../api/base";
import apiClient from "../api/client";
import type { VariableSet } from "../api/types";
import { useSSEList } from "../hooks/useSSEList";

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
  return (data ?? []).map(toVariableSetRow);
}

function toVariableSetRow(vs: VariableSet): VariableSetRow {
  return {
    id: resourceId(vs),
    name: resourceName(vs),
    namespace: resourceNamespace(vs),
    conditionCount: vs.status?.conditions?.length ?? 0,
  };
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
];

export default function VariableSets() {
  const initial = useLoaderData<typeof clientLoader>();
  const [variableSets, changedIds] = useSSEList<VariableSet, VariableSetRow>(
    apiUrl("/apis/magosproject.io/v1alpha1/variablesets/events"),
    initial,
    toVariableSetRow,
    clientLoader
  );

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Variable Sets" }]} />
      <PageTagline text="// hardcoded no more" />
      <ResourceList
        items={variableSets}
        searchKey="name"
        columns={columns}
        toHref={(vs) => `/variable-sets/${vs.namespace}/${vs.name}`}
        defaultView="row"
        hideViewToggle
        flashIds={changedIds}
      />
    </Stack>
  );
}
