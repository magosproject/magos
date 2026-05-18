import { Group, Stack, Text, Title } from "@mantine/core";
import { useLoaderData, useParams } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import KubeBadge from "../components/KubeBadge";
import StatusBadge from "../components/StatusBadge";
import ConditionsTable from "../components/ConditionsTable";
import VariablesTable from "../components/VariablesTable";
import UnresolvedReferencesTable from "../components/UnresolvedReferencesTable";
import { apiUrl } from "../api/base";
import apiClient from "../api/client";
import type { VariableSet } from "../api/types";
import { useSSEItem } from "../hooks/useSSEItem";

export function meta({ params }: { params: { namespace: string; name: string } }) {
  return [{ title: `${params.name} – magos` }];
}

export async function clientLoader({
  params,
}: {
  params: { namespace: string; name: string };
}) {
  const { data } = await apiClient.GET(
    "/apis/magosproject.io/v1alpha1/variablesets/{namespace}/{name}",
    { params: { path: { namespace: params.namespace, name: params.name } } }
  );
  if (!data) throw new Response("Not found", { status: 404 });
  return data;
}

export default function VariableSetDetail() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const initial = useLoaderData<typeof clientLoader>();
  const vs = useSSEItem<VariableSet>(
    apiUrl("/apis/magosproject.io/v1alpha1/variablesets/events"),
    initial,
    (obj) => obj.metadata?.namespace === namespace && obj.metadata?.name === name
  );

  const description = vs.spec?.description;
  const variables = vs.spec?.variables ?? [];
  const phase = vs.status?.phase ?? "";
  const unresolved = vs.status?.unresolvedReferences ?? [];

  return (
    <Stack gap="lg">
      <Breadcrumbs
        crumbs={[{ label: "Variable Sets", to: "/variable-sets" }, { label: name! }]}
      />

      <Stack gap={4}>
        <Group gap="xs" align="center">
          <Title order={2}>{name}</Title>
          <KubeBadge label={namespace!} />
          {phase && <StatusBadge status={phase} size="md" />}
        </Group>
        {description && (
          <Text size="sm" c="dimmed">
            {description}
          </Text>
        )}
      </Stack>

      <UnresolvedReferencesTable references={unresolved} />

      <VariablesTable variables={variables} />

      {vs.status?.conditions && vs.status.conditions.length > 0 && (
        <ConditionsTable conditions={vs.status.conditions} />
      )}
    </Stack>
  );
}
