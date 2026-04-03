import { Badge, Code, Stack, Text, Title, Paper, Group, SimpleGrid } from "@mantine/core";
import Breadcrumbs from "../components/Breadcrumbs";
import { IconGitBranch, IconServer, IconClock, IconBox } from "@tabler/icons-react";

export function meta() {
  return [{ title: "Settings – magos" }];
}

export default function Settings() {
  return (
    <Stack gap="lg">
      <Breadcrumbs crumbs={[{ label: "Settings" }]} />

      <Stack gap={4}>
        <Title order={2}>Platform Settings</Title>
        <Text size="sm" c="dimmed">
          Global configuration for the magos GitOps operator. These settings are read-only and
          managed by the cluster administrator.
        </Text>
      </Stack>

      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
        <Paper withBorder p="md" radius="md">
          <Group wrap="nowrap" align="flex-start">
            <IconGitBranch size={24} style={{ color: "var(--mantine-color-magos-5)" }} />
            <Stack gap="xs">
              <Text fw={600} size="sm">
                GitOps Source
              </Text>
              <Text size="sm" c="dimmed" mb="xs">
                The primary Git repository where the operator reads state.
              </Text>
              <Code block>https://github.com/magos/infrastructure-state.git</Code>
              <Group gap="xs" mt={4}>
                <Badge variant="light" color="blue" size="sm">
                  Branch: main
                </Badge>
                <Badge variant="light" color="green" size="sm">
                  Path: /clusters/prod-eu-west-1
                </Badge>
              </Group>
            </Stack>
          </Group>
        </Paper>

        <Paper withBorder p="md" radius="md">
          <Group wrap="nowrap" align="flex-start">
            <IconServer size={24} style={{ color: "var(--mantine-color-magos-5)" }} />
            <Stack gap="xs">
              <Text fw={600} size="sm">
                Target Cluster
              </Text>
              <Text size="sm" c="dimmed" mb="xs">
                The Kubernetes cluster being managed by this operator instance.
              </Text>
              <Code block>arn:aws:eks:eu-west-1:123456789012:cluster/prod-cluster</Code>
              <Group gap="xs" mt={4}>
                <Badge variant="light" color="magos" size="sm">
                  Context: admin@prod-cluster
                </Badge>
              </Group>
            </Stack>
          </Group>
        </Paper>

        <Paper withBorder p="md" radius="md">
          <Group wrap="nowrap" align="flex-start">
            <IconClock size={24} style={{ color: "var(--mantine-color-magos-5)" }} />
            <Stack gap="xs">
              <Text fw={600} size="sm">
                Reconciliation
              </Text>
              <Text size="sm" c="dimmed" mb="xs">
                Sync settings and intervals for configuration drift detection.
              </Text>
              <Group gap="md">
                <Stack gap={0}>
                  <Text size="xs" fw={500}>
                    Sync Interval
                  </Text>
                  <Text size="sm" c="dimmed">
                    3m0s
                  </Text>
                </Stack>
                <Stack gap={0}>
                  <Text size="xs" fw={500}>
                    Retry Interval
                  </Text>
                  <Text size="sm" c="dimmed">
                    1m0s
                  </Text>
                </Stack>
                <Stack gap={0}>
                  <Text size="xs" fw={500}>
                    Timeout
                  </Text>
                  <Text size="sm" c="dimmed">
                    5m0s
                  </Text>
                </Stack>
              </Group>
            </Stack>
          </Group>
        </Paper>

        <Paper withBorder p="md" radius="md">
          <Group wrap="nowrap" align="flex-start">
            <IconBox size={24} style={{ color: "var(--mantine-color-magos-5)" }} />
            <Stack gap="xs">
              <Text fw={600} size="sm">
                Operator Version
              </Text>
              <Text size="sm" c="dimmed" mb="xs">
                Information about the current deployed controller.
              </Text>
              <Group gap="xs">
                <Badge variant="outline" color="gray" size="sm">
                  v1.4.2
                </Badge>
                <Badge variant="light" color="green" size="sm">
                  Up to date
                </Badge>
              </Group>
            </Stack>
          </Group>
        </Paper>
      </SimpleGrid>
    </Stack>
  );
}
