import { useCallback, useEffect, useMemo, useRef } from "react";
import {
  Anchor,
  Box,
  SimpleGrid,
  Stack,
  Table,
  Tabs,
  Text,
  Title,
  Group,
  Button,
  ThemeIcon,
  useMantineTheme,
} from "@mantine/core";
import { Link, useParams } from "react-router";
import { IconRefresh, IconCheck, IconX, IconClock, IconPlayerPlay } from "@tabler/icons-react";
import {
  ReactFlow,
  Controls,
  type Edge,
  type Node,
  type NodeProps,
  Position,
  MarkerType,
  useNodesState,
  useEdgesState,
  useReactFlow,
  ReactFlowProvider,
  Handle,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import Breadcrumbs from "~/components/Breadcrumbs";
import InfoCard from "~/components/InfoCard";
import StatusBadge from "~/components/StatusBadge";
import NotFound from "~/components/NotFound";
import KubeBadge from "~/components/KubeBadge";
import { rollouts, type RolloutStep } from "~/mock-data/rollouts";
import { projects } from "~/mock-data/projects";
import { workspaces } from "~/mock-data/workspaces";

export function meta({ params }: { params: { id: string } }) {
  const rollout = rollouts.find((ro) => ro.id === params.id);
  return [{ title: `${rollout?.name ?? params.id} – magos` }];
}

// ---------------------------------------------------------------------------
// Step status helpers
// ---------------------------------------------------------------------------

type StepStatus = "completed" | "active" | "failed" | "pending";

function stepStatus(index: number, currentStep: number, phase: string): StepStatus {
  if (index < currentStep || phase === "Applied") return "completed";
  if (index === currentStep && phase === "Reconciling") return "active";
  if (index === currentStep && phase === "Failed") return "failed";
  return "pending";
}

// ---------------------------------------------------------------------------
// Custom ReactFlow node for a pipeline step
// ---------------------------------------------------------------------------

interface StepNodeData {
  step: RolloutStep;
  index: number;
  status: StepStatus;
  [key: string]: unknown;
}

function StepStatusIcon({ status }: { status: StepStatus }) {
  if (status === "completed")
    return (
      <ThemeIcon size={20} radius="xl" color="green" variant="filled">
        <IconCheck size={11} />
      </ThemeIcon>
    );
  if (status === "active")
    return (
      <ThemeIcon size={20} radius="xl" color="magos" variant="filled" className="pulse">
        <IconPlayerPlay size={11} />
      </ThemeIcon>
    );
  if (status === "failed")
    return (
      <ThemeIcon size={20} radius="xl" color="red" variant="filled">
        <IconX size={11} />
      </ThemeIcon>
    );
  return (
    <ThemeIcon size={20} radius="xl" color="dark.4" variant="filled">
      <IconClock size={11} />
    </ThemeIcon>
  );
}

const statusLabel: Record<StepStatus, string> = {
  completed: "Completed",
  active: "In progress",
  failed: "Failed",
  pending: "Pending",
};

function StepPipelineNode({ data }: NodeProps<Node<StepNodeData>>) {
  const { step, index, status } = data;

  return (
    <>
      <Handle type="target" position={Position.Left} style={{ visibility: "hidden" }} />
      <div className="step-pipeline-node" data-status={status}>
        <Stack gap={8}>
          <Group gap={6} wrap="nowrap" align="center">
            <StepStatusIcon status={status} />
            <Stack gap={0} style={{ flex: 1, minWidth: 0 }}>
              <Text size="xs" fw={600} truncate>
                {step.name}
              </Text>
              <Text size="xs" c="dimmed">
                Step {index + 1} &middot; {statusLabel[status]}
              </Text>
            </Stack>
          </Group>
          <Group gap={4} wrap="wrap">
            {Object.entries(step.selector).map(([k, v]) => {
              const shortKey = k.replace("magosproject.io/", "");
              return (
                <span key={k} className="label-chip">
                  <span style={{ color: "var(--mantine-color-magos-4)" }}>{shortKey}</span>
                  <span style={{ opacity: 0.35 }}>=</span>
                  <span style={{ color: "var(--mantine-color-dimmed)" }}>{v}</span>
                </span>
              );
            })}
          </Group>
        </Stack>
      </div>
      <Handle type="source" position={Position.Right} style={{ visibility: "hidden" }} />
    </>
  );
}

const nodeTypes = { stepNode: StepPipelineNode };

// ---------------------------------------------------------------------------
// Pipeline graph (horizontal flow between step nodes)
// ---------------------------------------------------------------------------

function StepPipelineGraph({
  steps,
  currentStep,
  phase,
}: {
  steps: RolloutStep[];
  currentStep: number;
  phase: string;
}) {
  const theme = useMantineTheme();
  const { fitView } = useReactFlow();
  const wrapperRef = useRef<HTMLDivElement>(null);

  const initialNodes = useMemo<Node<StepNodeData>[]>(() => {
    const xSpacing = 320;
    return steps.map((step, i) => ({
      id: `step-${i}`,
      type: "stepNode",
      position: { x: i * xSpacing, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      draggable: false,
      data: {
        step,
        index: i,
        status: stepStatus(i, currentStep, phase),
      },
    }));
  }, [steps, currentStep, phase]);

  const initialEdges = useMemo<Edge[]>(() => {
    return steps.slice(1).map((_, i) => {
      const srcStatus = stepStatus(i, currentStep, phase);
      const isFlowing = srcStatus === "completed" || srcStatus === "active";
      const isFailed = stepStatus(i + 1, currentStep, phase) === "failed";

      let strokeColor = theme.colors.dark[4];
      if (isFailed) strokeColor = theme.colors.red[6];
      else if (isFlowing) strokeColor = theme.colors.green[6];

      return {
        id: `e-${i}-${i + 1}`,
        source: `step-${i}`,
        target: `step-${i + 1}`,
        type: "smoothstep",
        animated: isFlowing,
        markerEnd: { type: MarkerType.ArrowClosed, color: strokeColor },
        style: { stroke: strokeColor, strokeWidth: 2 },
      };
    });
  }, [steps, currentStep, phase, theme]);

  const [nodes, , onNodesChange] = useNodesState(initialNodes);
  const [edges, , onEdgesChange] = useEdgesState(initialEdges);

  useEffect(() => {
    if (!wrapperRef.current) return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.width > 0 && entry.contentRect.height > 0) {
          window.requestAnimationFrame(() => {
            fitView({ padding: 0.3, minZoom: 0.5, maxZoom: 1.5, duration: 600 });
          });
        }
      }
    });
    observer.observe(wrapperRef.current);
    return () => observer.disconnect();
  }, [fitView]);

  return (
    <Box
      ref={wrapperRef}
      h={180}
      w="100%"
      style={{
        border: "1px solid var(--mantine-color-default-border)",
        borderRadius: "var(--mantine-radius-md)",
      }}
    >
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        fitView
        panOnDrag
        zoomOnScroll={false}
        zoomOnPinch
        preventScrolling={false}
        proOptions={{ hideAttribution: true }}
      >
        <Controls showInteractive={false} />
      </ReactFlow>
    </Box>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function Rollout() {
  const { id } = useParams<{ id: string }>();
  const rollout = rollouts.find((ro) => ro.id === id);

  if (!rollout) {
    return <NotFound message="Rollout not found." />;
  }

  const project = projects.find((p) => p.id === rollout.projectRef);
  const projectWorkspaces = project
    ? workspaces.filter((ws) => project.workspaceIds.includes(ws.id))
    : [];

  return (
    <Stack gap="lg">
      <Breadcrumbs crumbs={[{ label: "Rollouts", to: "/rollouts" }, { label: rollout.name }]} />

      <Group justify="space-between" align="center">
        <Group gap="xs" align="center">
          <Title order={2}>{rollout.name}</Title>
          <KubeBadge label={rollout.namespace} />
        </Group>
        <Button leftSection={<IconRefresh size={16} />} variant="default" size="sm">
          Reconcile
        </Button>
      </Group>

      <Tabs defaultValue="overview">
        <Tabs.List>
          <Tabs.Tab value="overview">Overview</Tabs.Tab>
          <Tabs.Tab value="steps">Steps</Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="overview" pt="md">
          <Stack gap="md">
            <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="md">
              <InfoCard label="Phase">
                <StatusBadge status={rollout.phase} size="md" />
              </InfoCard>
              <InfoCard label="Project">
                {project ? (
                  <Anchor component={Link} to={`/projects/${project.id}`} size="sm" c="dimmed">
                    {project.name}
                  </Anchor>
                ) : (
                  <Text size="sm" c="dimmed">
                    {rollout.projectRef}
                  </Text>
                )}
              </InfoCard>
              <InfoCard label="Progress">
                <Text size="sm">
                  Step {Math.min(rollout.currentStep + 1, rollout.steps.length)} of{" "}
                  {rollout.steps.length}
                </Text>
              </InfoCard>
              <InfoCard label="Reason">
                <Text size="sm" c="dimmed">
                  {rollout.reason}
                </Text>
              </InfoCard>
              <InfoCard label="Kubernetes Namespace">
                <KubeBadge label={rollout.namespace} />
              </InfoCard>
            </SimpleGrid>

            <Stack gap="xs">
              <Title order={4}>Workspaces</Title>
              {projectWorkspaces.length === 0 ? (
                <Text size="sm" c="dimmed">
                  No workspaces linked to this rollout's project.
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
          </Stack>
        </Tabs.Panel>

        <Tabs.Panel value="steps" pt="md">
          <ReactFlowProvider>
            <StepPipelineGraph
              steps={rollout.steps}
              currentStep={rollout.currentStep}
              phase={rollout.phase}
            />
          </ReactFlowProvider>
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}
