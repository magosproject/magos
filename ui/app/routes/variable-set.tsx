import { Badge, Button, Group, SimpleGrid, Stack, Table, Text, Title } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { useLoaderData, useParams } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import InfoCard from "../components/InfoCard";
import KubeBadge from "../components/KubeBadge";
import apiClient from "../api/client";

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
  const vs = useLoaderData<typeof clientLoader>();

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
        <Stack gap="xs">
          <Title order={4}>Conditions</Title>
          <Table withTableBorder withColumnBorders={false}>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Type</Table.Th>
                <Table.Th>Status</Table.Th>
                <Table.Th>Reason</Table.Th>
                <Table.Th>Message</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {vs.status.conditions.map((c) => (
                <Table.Tr key={c.type}>
                  <Table.Td>
                    <Text size="sm" fw={500}>
                      {c.type}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Badge
                      variant="light"
                      color={c.status === "True" ? "green" : c.status === "False" ? "red" : "gray"}
                      size="sm"
                    >
                      {c.status}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm" c="dimmed">
                      {c.reason}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm" c="dimmed">
                      {c.message}
                    </Text>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Stack>
      )}
    </Stack>
  );
}
