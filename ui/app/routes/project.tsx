import {
  Anchor,
  Badge,
  SimpleGrid,
  Stack,
  Table,
  Tabs,
  Text,
  Title,
  Group,
  Button,
} from "@mantine/core";
import { Link, useParams } from "react-router";
import { IconRefresh } from "@tabler/icons-react";
import Breadcrumbs from "~/components/Breadcrumbs";
import InfoCard from "~/components/InfoCard";
import StatusBadge from "~/components/StatusBadge";
import NotFound from "~/components/NotFound";
import KubeBadge from "~/components/KubeBadge";
import ProjectLineageGraph from "~/components/ProjectLineageGraph";
import { projects } from "~/mock-data/projects";
import { workspaces } from "~/mock-data/workspaces";
import { variableSets } from "~/mock-data/variable-sets";

export function meta({ params }: { params: { id: string } }) {
  const project = projects.find((p) => p.id === params.id);
  return [{ title: `${project?.name ?? params.id} – magos` }];
}

export default function Project() {
  const { id } = useParams<{ id: string }>();
  const project = projects.find((p) => p.id === id);

  if (!project) {
    return <NotFound message="Project not found." />;
  }

  const projectWorkspaces = workspaces.filter((ws) => project.workspaceIds.includes(ws.id));
  const projectVariableSets = variableSets.filter((vs) => vs.projectRef === project.id);

  return (
    <Stack gap="lg">
      <Breadcrumbs crumbs={[{ label: "Projects", to: "/projects" }, { label: project.name }]} />

      <Group justify="space-between" align="flex-start">
        <Stack gap={4}>
          <Group gap="xs" align="center">
            <Title order={2}>{project.name}</Title>
            <KubeBadge label={project.namespace} />
          </Group>
          <Text size="sm" c="dimmed">
            {project.description}
          </Text>
        </Stack>
        <Button leftSection={<IconRefresh size={16} />} variant="default" size="sm">
          Reconcile
        </Button>
      </Group>

      <Tabs defaultValue="overview">
        <Tabs.List>
          <Tabs.Tab value="overview">Overview</Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="overview" pt="md">
          <Stack gap="md">
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
              <InfoCard label="Workspaces">
                <Text size="sm">{projectWorkspaces.length}</Text>
              </InfoCard>
              <InfoCard label="Variable Sets">
                <Text size="sm">{projectVariableSets.length}</Text>
              </InfoCard>
            </SimpleGrid>

            <SimpleGrid cols={{ base: 1, md: 2 }} spacing="xl">
              <Stack gap="xs" style={{ gridColumn: "1 / -1" }}>
                <Title order={4}>Inheritance Lineage</Title>
                <Text size="sm" c="dimmed">
                  Visual representation of the variable sets applied to this project and inherited
                  by its workspaces.
                </Text>
                <ProjectLineageGraph
                  project={project}
                  projectVariableSets={projectVariableSets}
                  projectWorkspaces={projectWorkspaces}
                />
              </Stack>

              <Stack gap="xs">
                <Title order={4}>Workspaces</Title>
                {projectWorkspaces.length === 0 ? (
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
                      {projectWorkspaces.map((ws) => (
                        <Table.Tr key={ws.id}>
                          <Table.Td>
                            <Anchor component={Link} to={`/workspaces/${ws.id}`} size="sm" fw={500}>
                              {ws.name}
                            </Anchor>
                          </Table.Td>
                          <Table.Td>
                            <StatusBadge status={ws.status} />
                          </Table.Td>
                          <Table.Td>
                            <KubeBadge label={ws.namespace} />
                          </Table.Td>
                        </Table.Tr>
                      ))}
                    </Table.Tbody>
                  </Table>
                )}
              </Stack>

              <Stack gap="xs">
                <Title order={4}>Variable Sets</Title>
                {projectVariableSets.length === 0 ? (
                  <Text size="sm" c="dimmed">
                    No variable sets linked to this project.
                  </Text>
                ) : (
                  <Table highlightOnHover withTableBorder withColumnBorders={false}>
                    <Table.Thead>
                      <Table.Tr>
                        <Table.Th>Name</Table.Th>
                        <Table.Th>Variables</Table.Th>
                        <Table.Th>Kubernetes Namespace</Table.Th>
                      </Table.Tr>
                    </Table.Thead>
                    <Table.Tbody>
                      {projectVariableSets.map((vs) => (
                        <Table.Tr key={vs.id}>
                          <Table.Td>
                            <Anchor
                              component={Link}
                              to={`/variable-sets/${vs.id}`}
                              size="sm"
                              fw={500}
                            >
                              {vs.name}
                            </Anchor>
                          </Table.Td>
                          <Table.Td>
                            <Badge variant="light" color="magos" size="sm">
                              {vs.variables.length}
                            </Badge>
                          </Table.Td>
                          <Table.Td>
                            <KubeBadge label={vs.namespace} />
                          </Table.Td>
                        </Table.Tr>
                      ))}
                    </Table.Tbody>
                  </Table>
                )}
              </Stack>
            </SimpleGrid>
          </Stack>
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}
