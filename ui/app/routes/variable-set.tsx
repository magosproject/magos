import {
  Badge,
  Code,
  Group,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
  Anchor,
  Button,
} from "@mantine/core";
import { IconBox, IconLock, IconRefresh } from "@tabler/icons-react";
import { Link, useParams } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import InfoCard from "../components/InfoCard";
import NotFound from "../components/NotFound";
import KubeBadge from "../components/KubeBadge";
import { variableSets } from "../mock-data/variable-sets";
import { projects } from "../mock-data/projects";

export function meta({ params }: { params: { id: string } }) {
  const vs = variableSets.find((v) => v.id === params.id);
  return [{ title: `${vs?.name ?? params.id} – magos` }];
}

export default function VariableSetDetail() {
  const { id } = useParams<{ id: string }>();
  const vs = variableSets.find((v) => v.id === id);

  if (!vs) {
    return <NotFound message="Variable Set not found." />;
  }

  const project = projects.find((p) => p.id === vs.projectRef);

  return (
    <Stack gap="lg">
      <Breadcrumbs
        crumbs={[{ label: "Variable Sets", to: "/variable-sets" }, { label: vs.name }]}
      />

      <Group justify="space-between" align="flex-start">
        <Stack gap={4}>
          <Group gap="xs" align="center">
            <Title order={2}>{vs.name}</Title>
            <KubeBadge label={vs.namespace} />
          </Group>
        </Stack>
        <Button leftSection={<IconRefresh size={16} />} variant="default" size="sm">
          Reconcile
        </Button>
      </Group>

      <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="md">
        <InfoCard label="Project">
          <Group gap={6} wrap="nowrap">
            <IconBox size={14} />
            {project ? (
              <Anchor component={Link} to={`/projects/${project.id}`} size="sm" truncate>
                {project.name}
              </Anchor>
            ) : (
              <Text size="sm" c="dimmed" fs="italic">
                {vs.projectRef || "None (Direct attachment)"}
              </Text>
            )}
          </Group>
        </InfoCard>
        <InfoCard label="Kubernetes Namespace">
          <KubeBadge label={vs.namespace} />
        </InfoCard>
      </SimpleGrid>

      <Stack gap="xs">
        <Title order={4}>Variables</Title>
        {vs.variables.length === 0 ? (
          <Text size="sm" c="dimmed">
            No variables defined in this set.
          </Text>
        ) : (
          <Table highlightOnHover withTableBorder withColumnBorders={false}>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Key</Table.Th>
                <Table.Th>Value</Table.Th>
                <Table.Th>Category</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {vs.variables.map((variable, idx) => (
                <Table.Tr key={`${variable.key}-${idx}`}>
                  <Table.Td>
                    <Group gap="xs" wrap="nowrap">
                      <Text size="sm" fw={600}>
                        {variable.key}
                      </Text>
                      {variable.sensitive && (
                        <IconLock size={14} style={{ color: "var(--mantine-color-dimmed)" }} />
                      )}
                    </Group>
                  </Table.Td>
                  <Table.Td>
                    {variable.sensitive && variable.valueFrom ? (
                      <KubeBadge label={`${vs.namespace}/${variable.valueFrom.secretRef.name}`} />
                    ) : variable.sensitive ? (
                      <Text size="sm" c="dimmed" fs="italic">
                        Sensitive
                      </Text>
                    ) : (
                      <Code style={{ whiteSpace: "pre-wrap", wordBreak: "break-all" }}>
                        {variable.value}
                      </Code>
                    )}
                  </Table.Td>
                  <Table.Td>
                    <Badge
                      variant="light"
                      color={variable.category === "terraform" ? "blue" : "grape"}
                      size="sm"
                    >
                      {variable.category}
                    </Badge>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Stack>
    </Stack>
  );
}
