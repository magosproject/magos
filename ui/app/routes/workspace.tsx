import { useState } from "react";
import {
  ActionIcon,
  Anchor,
  Badge,
  Box,
  Button,
  Code,
  Drawer,
  Group,
  ScrollArea,
  SimpleGrid,
  Stack,
  Table,
  Tabs,
  Text,
  Title,
} from "@mantine/core";
import {
  IconCalendarClock,
  IconCalendarEvent,
  IconChevronDown,
  IconChevronUp,
  IconFolder,
  IconRefresh,
  IconUserCog,
  IconBox,
  IconLock,
} from "@tabler/icons-react";
import { Link, useParams } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import InfoCard from "../components/InfoCard";
import StatusBadge from "../components/StatusBadge";
import NotFound from "../components/NotFound";
import KubeBadge from "../components/KubeBadge";
import { repoIcon } from "../utils/repoIcon";
import { workspaces } from "../mock-data/workspaces";
import { projects } from "../mock-data/projects";
import { variableSets } from "../mock-data/variable-sets";
import { executions, schedules, type Execution } from "../mock-data/executions";

import VariableLineageGraph from "../components/VariableLineageGraph";

export function meta({ params }: { params: { id: string } }) {
  const ws = workspaces.find((w) => w.id === params.id);
  return [{ title: `${ws?.name ?? params.id} – magos` }];
}

const executionStatusColor: Record<string, string> = {
  success: "green",
  failed: "red",
  running: "yellow",
  pending_approval: "blue",
};

const logLevelColor: Record<string, string> = {
  info: "dimmed",
  warn: "yellow",
  error: "red",
};

function formatDate(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function duration(start: string, end: string | null) {
  if (!end) return "running…";
  const ms = new Date(end).getTime() - new Date(start).getTime();
  const s = Math.floor(ms / 1000);
  return s < 60 ? `${s}s` : `${Math.floor(s / 60)}m ${s % 60}s`;
}

export default function Workspace() {
  const { id } = useParams<{ id: string }>();
  const [selectedExecution, setSelectedExecution] = useState<Execution | null>(null);
  const [logsExpanded, setLogsExpanded] = useState(false);

  const ws = workspaces.find((w) => w.id === id);
  const schedule = schedules.find((s) => s.workspaceId === id);
  const wsExecutions = executions.filter((e) => e.workspaceId === id);
  const pendingExecutions = wsExecutions.filter((e) => e.status === "pending_approval");
  const historyExecutions = wsExecutions.filter((e) => e.status !== "pending_approval");
  const project = projects.find((p) => p.workspaceIds.includes(id ?? ""));

  if (!ws) {
    return <NotFound message="Workspace not found." />;
  }

  // Find VariableSets assigned to the Workspace's Project
  const projectVariableSets = project
    ? variableSets.filter((vs) => vs.projectRef === project.id)
    : [];

  // Find VariableSets directly assigned to the Workspace
  const directVariableSets = ws.variableSetRefs
    ? variableSets.filter((vs) => ws.variableSetRefs?.includes(vs.id))
    : [];

  const allLinkedVariableSets = [...projectVariableSets, ...directVariableSets];

  return (
    <>
      <Stack gap="lg">
        <Breadcrumbs crumbs={[{ label: "Workspaces", to: "/" }, { label: ws.name }]} />

        <Group justify="space-between" align="flex-start">
          <Stack gap={4}>
            <Group gap="xs" align="center">
              <Title order={2}>{ws.name}</Title>
              <KubeBadge label={ws.namespace} />
            </Group>
          </Stack>
          <Button leftSection={<IconRefresh size={16} />} variant="default" size="sm">
            Reconcile
          </Button>
        </Group>

        <Tabs defaultValue="overview">
          <Tabs.List>
            <Tabs.Tab value="overview">Overview</Tabs.Tab>
            <Tabs.Tab value="variables">Variables</Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="overview" pt="md">
            <Stack gap="lg">
              <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="md">
                <InfoCard label="Status">
                  <StatusBadge status={ws.status} size="md" />
                </InfoCard>
                <InfoCard label="Project">
                  <Group gap={6} wrap="nowrap">
                    <IconBox size={14} />
                    {project ? (
                      <Anchor component={Link} to={`/projects/${project.id}`} size="sm" c="dimmed">
                        {project.name}
                      </Anchor>
                    ) : (
                      <Text size="sm" c="dimmed">
                        No Project
                      </Text>
                    )}
                  </Group>
                </InfoCard>
                <InfoCard label="Repository">
                  <Group gap={6} wrap="nowrap">
                    {repoIcon(ws.repoHost, 14)}
                    <Anchor
                      href={`${ws.repoHost}/${ws.repoSlug}`}
                      target="_blank"
                      size="sm"
                      truncate
                    >
                      {ws.repoSlug}
                    </Anchor>
                  </Group>
                </InfoCard>
                <InfoCard label="Path">
                  <Group gap={6} wrap="nowrap">
                    <IconFolder size={14} />
                    <Text size="sm" c="dimmed" truncate>
                      {ws.path}
                    </Text>
                  </Group>
                </InfoCard>
                <InfoCard label="Kubernetes Namespace">
                  <KubeBadge label={ws.namespace} />
                </InfoCard>
                <InfoCard label="Service account">
                  <Group gap={6} wrap="nowrap">
                    <IconUserCog size={14} />
                    <Text size="sm" c="dimmed" truncate>
                      {ws.serviceAccount}
                    </Text>
                  </Group>
                </InfoCard>
                <InfoCard label="Reconciliation">
                  <Group gap={6} wrap="nowrap">
                    <IconRefresh size={14} />
                    <Badge
                      color={ws.reconciliationEnabled ? "magos" : "gray"}
                      variant="light"
                      size="sm"
                    >
                      {ws.reconciliationEnabled ? "enabled" : "disabled"}
                    </Badge>
                  </Group>
                </InfoCard>
                {schedule && (
                  <>
                    <InfoCard label="Last invocation">
                      <Group gap={6} wrap="nowrap">
                        <IconCalendarEvent size={14} />
                        <Text size="sm" c="dimmed">
                          {formatDate(schedule.lastInvocation)}
                        </Text>
                      </Group>
                    </InfoCard>
                    <InfoCard label="Next invocation">
                      <Group gap={6} wrap="nowrap">
                        <IconCalendarClock size={14} />
                        <Text size="sm" c="dimmed">
                          {formatDate(schedule.nextInvocation)}
                        </Text>
                      </Group>
                    </InfoCard>
                  </>
                )}
              </SimpleGrid>

              {!ws.autoApply && pendingExecutions.length > 0 && (
                <Stack gap="xs">
                  <Title order={4}>Pending Approvals</Title>
                  <Table highlightOnHover withTableBorder withColumnBorders={false}>
                    <Table.Thead>
                      <Table.Tr>
                        <Table.Th>Status</Table.Th>
                        <Table.Th>Started</Table.Th>
                        <Table.Th>Triggered by</Table.Th>
                      </Table.Tr>
                    </Table.Thead>
                    <Table.Tbody>
                      {pendingExecutions.map((ex) => (
                        <Table.Tr
                          key={ex.id}
                          onClick={() => setSelectedExecution(ex)}
                          style={{ cursor: "pointer" }}
                        >
                          <Table.Td>
                            <Badge
                              color={executionStatusColor[ex.status]}
                              variant="light"
                              size="sm"
                            >
                              {ex.status.replace("_", " ")}
                            </Badge>
                          </Table.Td>
                          <Table.Td>
                            <Text size="sm" c="dimmed">
                              {formatDate(ex.startedAt)}
                            </Text>
                          </Table.Td>
                          <Table.Td>
                            <Text size="sm" c="dimmed">
                              {ex.triggeredBy}
                            </Text>
                          </Table.Td>
                        </Table.Tr>
                      ))}
                    </Table.Tbody>
                  </Table>
                </Stack>
              )}

              <Stack gap="xs">
                <Title order={4}>Execution history</Title>
                {historyExecutions.length === 0 ? (
                  <Text size="sm" c="dimmed">
                    No executions recorded.
                  </Text>
                ) : (
                  <Table highlightOnHover withTableBorder withColumnBorders={false}>
                    <Table.Thead>
                      <Table.Tr>
                        <Table.Th>Status</Table.Th>
                        <Table.Th>Started</Table.Th>
                        <Table.Th>Duration</Table.Th>
                        <Table.Th>Triggered by</Table.Th>
                      </Table.Tr>
                    </Table.Thead>
                    <Table.Tbody>
                      {historyExecutions.map((ex) => (
                        <Table.Tr
                          key={ex.id}
                          onClick={() => setSelectedExecution(ex)}
                          style={{ cursor: "pointer" }}
                        >
                          <Table.Td>
                            <Badge
                              color={executionStatusColor[ex.status]}
                              variant="light"
                              size="sm"
                            >
                              {ex.status.replace("_", " ")}
                            </Badge>
                          </Table.Td>
                          <Table.Td>
                            <Text size="sm" c="dimmed">
                              {formatDate(ex.startedAt)}
                            </Text>
                          </Table.Td>
                          <Table.Td>
                            <Text size="sm" c="dimmed">
                              {duration(ex.startedAt, ex.finishedAt)}
                            </Text>
                          </Table.Td>
                          <Table.Td>
                            <Text size="sm" c="dimmed">
                              {ex.triggeredBy}
                            </Text>
                          </Table.Td>
                        </Table.Tr>
                      ))}
                    </Table.Tbody>
                  </Table>
                )}
              </Stack>
            </Stack>
          </Tabs.Panel>

          <Tabs.Panel value="variables" pt="md">
            <Stack gap="xl">
              <Stack gap="xs">
                <Title order={4}>Inheritance Lineage</Title>
                <Text size="sm" c="dimmed">
                  Visual representation of how variables are inherited down to this workspace.
                </Text>
                <VariableLineageGraph
                  workspace={ws}
                  project={project}
                  projectVariableSets={projectVariableSets}
                  directVariableSets={directVariableSets}
                />
              </Stack>

              {allLinkedVariableSets.length === 0 ? (
                <Text size="sm" c="dimmed">
                  No variables inherited from any VariableSets.
                </Text>
              ) : (
                allLinkedVariableSets.map((vs) => (
                  <Stack key={vs.id} gap="xs">
                    <Group gap="xs" align="center">
                      <Title order={5}>
                        <Anchor
                          component={Link}
                          to={`/variable-sets/${vs.id}`}
                          c="inherit"
                          underline="hover"
                        >
                          {vs.name}
                        </Anchor>
                      </Title>
                      {projectVariableSets.some((pvs) => pvs.id === vs.id) ? (
                        <Badge
                          variant="light"
                          size="sm"
                          color="blue"
                          leftSection={<IconBox size={12} />}
                        >
                          Inherited from Project: {project?.name}
                        </Badge>
                      ) : (
                        <Badge
                          variant="light"
                          size="sm"
                          color="magos"
                          leftSection={<IconFolder size={12} />}
                        >
                          Directly attached to Workspace
                        </Badge>
                      )}
                    </Group>

                    {vs.variables.length === 0 ? (
                      <Text size="sm" c="dimmed">
                        No variables defined in this set.
                      </Text>
                    ) : (
                      <Table highlightOnHover withTableBorder withColumnBorders={false}>
                        <Table.Thead>
                          <Table.Tr>
                            <Table.Th w="35%">Key</Table.Th>
                            <Table.Th w="50%">Value</Table.Th>
                            <Table.Th w="15%">Category</Table.Th>
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
                                    <IconLock
                                      size={14}
                                      style={{ color: "var(--mantine-color-dimmed)" }}
                                    />
                                  )}
                                </Group>
                              </Table.Td>
                              <Table.Td>
                                {variable.sensitive && variable.valueFrom ? (
                                  <KubeBadge
                                    label={`${vs.namespace}/${variable.valueFrom.secretRef.name}`}
                                  />
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
                ))
              )}
            </Stack>
          </Tabs.Panel>
        </Tabs>
      </Stack>

      <Drawer
        opened={selectedExecution !== null}
        onClose={() => {
          setSelectedExecution(null);
          setLogsExpanded(false);
        }}
        title={
          <Group justify="space-between" w="100%" pr="md">
            <Group gap="xs">
              <Text fw={600} size="sm">
                Execution logs
              </Text>
              {selectedExecution && (
                <Badge
                  color={executionStatusColor[selectedExecution.status]}
                  variant="light"
                  size="sm"
                >
                  {selectedExecution.status.replace("_", " ")}
                </Badge>
              )}
              {selectedExecution?.status === "pending_approval" && (
                <Button size="compact-sm" variant="filled" color="magos" ml="md">
                  Approve & Apply
                </Button>
              )}
            </Group>
            <ActionIcon
              variant="subtle"
              color="gray"
              size="sm"
              onClick={() => setLogsExpanded((e) => !e)}
            >
              {logsExpanded ? <IconChevronDown size={16} /> : <IconChevronUp size={16} />}
            </ActionIcon>
          </Group>
        }
        position="bottom"
        size={logsExpanded ? "100%" : "50%"}
      >
        {selectedExecution && (
          <ScrollArea h="100%">
            <Stack gap={4}>
              {selectedExecution.logs.map((log, i) => (
                <Box key={i}>
                  <Code block style={{ whiteSpace: "pre-wrap", wordBreak: "break-all" }}>
                    <Text span size="xs" c="dimmed">
                      {formatDate(log.timestamp)}
                    </Text>
                    {"  "}
                    <Text
                      span
                      size="xs"
                      c={logLevelColor[log.level]}
                      fw={log.level !== "info" ? 600 : undefined}
                    >
                      [{log.level.toUpperCase()}]
                    </Text>
                    {"  "}
                    <Text span size="xs">
                      {log.message}
                    </Text>
                  </Code>
                </Box>
              ))}
            </Stack>
          </ScrollArea>
        )}
      </Drawer>
    </>
  );
}
