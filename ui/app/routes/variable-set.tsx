import { Button, Group, SimpleGrid, Stack, Title } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { useLoaderData, useParams } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import InfoCard from "../components/InfoCard";
import KubeBadge from "../components/KubeBadge";
import ConditionsTable from "../components/ConditionsTable";
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
    "/apis/magosproject.io/v1alpha1/variablesets/events",
    initial,
    (obj) => obj.metadata?.namespace === namespace && obj.metadata?.name === name
  );

  return (
    <Stack gap="lg">
      <Breadcrumbs
        crumbs={[{ label: "Variable Sets", to: "/variable-sets" }, { label: name! }]}
      />

      <Group justify="space-between" align="flex-start">
        <Group gap="xs" align="center">
          <Title order={2}>{name}</Title>
          <KubeBadge label={namespace!} />
        </Group>
        <Button leftSection={<IconRefresh size={16} />} variant="default" size="sm">
          Reconcile
        </Button>
      </Group>

      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
        <InfoCard label="Kubernetes Namespace">
          <KubeBadge label={namespace!} />
        </InfoCard>
      </SimpleGrid>

      {vs.status?.conditions && vs.status.conditions.length > 0 && (
        <ConditionsTable conditions={vs.status.conditions} />
      )}
    </Stack>
  );
}
