import {
  Anchor,
  Badge,
  Button,
  Group,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import { Link, useLoaderData, useParams } from "react-router";
import { IconRefresh } from "@tabler/icons-react";
import Breadcrumbs from "~/components/Breadcrumbs";
import InfoCard from "~/components/InfoCard";
import StatusBadge from "~/components/StatusBadge";
import KubeBadge from "~/components/KubeBadge";
import apiClient from "~/api/client";

export function meta({ params }: { params: { namespace: string; name: string } }) {
  return [{ title: `${params.name} – magos` }];
}

export async function clientLoader({
  params,
}: {
  params: { namespace: string; name: string };
}) {
  const [projectRes, workspacesRes] = await Promise.all([
    apiClient.GET("/apis/magosproject.io/v1alpha1/projects/{namespace}/{name}", {
      params: { path: { namespace: params.namespace, name: params.name } },
    }),
    apiClient.GET("/apis/magosproject.io/v1alpha1/workspaces"),
  ]);

  if (!projectRes.data) throw new Response("Not found", { status: 404 });

  const projectWorkspaces = (workspacesRes.data ?? []).filter(
    (ws) => ws.spec?.projectRef?.name === params.name
  );

  return { project: projectRes.data, workspaces: projectWorkspaces };
}

export default function Project() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const { project, workspaces } = useLoaderData<typeof clientLoader>();

  return (
    <Stack gap="lg">
      <Breadcrumbs crumbs={[{ label: "Projects", to: "/projects" }, { label: name! }]} />

      <Group justify="space-between" align="flex-start">
        <Stack gap={4}>
          <Group gap="xs" align="center">
            <Title order={2}>{name}</Title>
            <KubeBadge label={namespace!} />
          </Group>
          {project.spec?.description && (
            <Text size="sm" c="dimmed">
              {project.spec.description}
            </Text>
          )}
        </Stack>
        <Button leftSection={<IconRefresh size={16} />} variant="default" size="sm">
          Reconcile
        </Button>
      </Group>

      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
        <InfoCard label="Status">
          <StatusBadge status={project.status?.phase ?? ""} size="md" />
        </InfoCard>
        <InfoCard label="Workspaces">
          <Text size="sm">{workspaces.length}</Text>
        </InfoCard>
      </SimpleGrid>

      <Stack gap="xs">
        <Title order={4}>Workspaces</Title>
        {workspaces.length === 0 ? (
          <Text size="sm" c="dimmed">
            No workspaces linked to this project.
          </Text>
        ) : (
          <Table highlightOnHover withTableBorder withColumnBorders={false}>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Status</Table.Th>
                <Table.Th>Kubernetes Namespace</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {workspaces.map((ws) => (
                <Table.Tr key={ws.metadata?.uid ?? ws.metadata?.name}>
                  <Table.Td>
                    <Anchor
                      component={Link}
                      to={`/workspaces/${ws.metadata?.namespace}/${ws.metadata?.name}`}
                      size="sm"
                      fw={500}
                    >
                      {ws.metadata?.name}
                    </Anchor>
                  </Table.Td>
                  <Table.Td>
                    <StatusBadge status={ws.status?.phase ?? ""} />
                  </Table.Td>
                  <Table.Td>
                    <KubeBadge label={ws.metadata?.namespace ?? ""} />
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Stack>

      {project.spec?.variableSetRef && project.spec.variableSetRef.length > 0 && (
        <Stack gap="xs">
          <Title order={4}>Variable Sets</Title>
          <Group gap="xs">
            {project.spec.variableSetRef.map((ref) => (
              <Badge key={ref.name} variant="light" color="magos" size="sm">
                {ref.name}
              </Badge>
            ))}
          </Group>
        </Stack>
      )}
    </Stack>
  );
}
