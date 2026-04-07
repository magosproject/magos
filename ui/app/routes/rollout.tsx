import { useCallback, useEffect, useMemo, useRef } from "react";
import {
  Box,
  Button,
  Group,
  SimpleGrid,
  Stack,
  Text,
  Title,
  ThemeIcon,
  useMantineTheme,
} from "@mantine/core";
import { useLoaderData, useParams } from "react-router";
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
import KubeBadge from "~/components/KubeBadge";
import { Tabs } from "@mantine/core";
import apiClient from "~/api/client";
import type { Rollout as RolloutType } from "~/api/types";
import { useSSEItem } from "~/hooks/useSSEItem";

export function meta({ params }: { params: { namespace: string; name: string } }) {
  return [{ title: `${params.name} – magos` }];
}

export async function clientLoader({
  params,
}: {
  params: { namespace: string; name: string };
}) {
  const { data } = await apiClient.GET(
    "/apis/magosproject.io/v1alpha1/rollouts/{namespace}/{name}",
    { params: { path: { namespace: params.namespace, name: params.name } } }
  );
  if (!data) throw new Response("Not found", { status: 404 });
  return data;
}

type StepStatus = "completed" | "active" | "failed" | "pending";

function stepStatus(index: number, currentStep: number, phase: string, groups: number[][]): StepStatus {
  if (phase === "Applied") return "completed";

  const stepGroupIdx = groups.findIndex((g) => g.includes(index));
  const currentGroupIdx = groups.findIndex((g) => g.includes(currentStep));

  if (stepGroupIdx < currentGroupIdx) return "completed";
  if (stepGroupIdx === currentGroupIdx) {
    if (phase === "Reconciling") return "active";
    if (phase === "Failed") return "failed";
  }
  return "pending";
}

interface StepNodeData {
  name: string;
  index: number;
  status: StepStatus;
  labels: Record<string, string>;
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
  const { name, index, status, labels } = data;
  return (
    <>
      <Handle type="target" position={Position.Left} style={{ visibility: "hidden" }} />
      <div className="step-pipeline-node" data-status={status}>
        <Stack gap={8}>
          <Group gap={6} wrap="nowrap" align="center">
            <StepStatusIcon status={status} />
            <Stack gap={0} style={{ flex: 1, minWidth: 0 }}>
              <Text size="xs" fw={600} truncate>
                {name}
              </Text>
              <Text size="xs" c="dimmed">
                Step {index + 1} &middot; {statusLabel[status]}
              </Text>
            </Stack>
          </Group>
          <Group gap={4} wrap="wrap">
            {Object.entries(labels).map(([k, v]) => {
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

function groupStepsByLabels(steps: { name: string; labels: Record<string, string> }[]): number[][] {
  const toKey = (labels: Record<string, string>) =>
    JSON.stringify(Object.entries(labels).sort((a, b) => a[0].localeCompare(b[0])));

  const groups: number[][] = [];
  const seen = new Map<string, number>();

  for (let i = 0; i < steps.length; i++) {
    const key = toKey(steps[i].labels);
    if (!seen.has(key)) {
      seen.set(key, groups.length);
      groups.push([i]);
    } else {
      groups[seen.get(key)!].push(i);
    }
  }

  return groups;
}

function StepPipelineGraph({
  steps,
  currentStep,
  phase,
}: {
  steps: { name: string; labels: Record<string, string> }[];
  currentStep: number;
  phase: string;
}) {
  const theme = useMantineTheme();
  const { fitView } = useReactFlow();
  const wrapperRef = useRef<HTMLDivElement>(null);

  const colWidth = 320;
  const rowHeight = 140;

  const groups = useMemo(() => groupStepsByLabels(steps), [steps]);

  const graphHeight = Math.max(180, Math.max(...groups.map((g) => g.length)) * rowHeight + 40);

  const computedNodes = useMemo<Node<StepNodeData>[]>(() => {
    return steps.map((step, i) => {
      const groupIdx = groups.findIndex((g) => g.includes(i));
      const posInGroup = groups[groupIdx].indexOf(i);
      const groupSize = groups[groupIdx].length;
      const y = (posInGroup - (groupSize - 1) / 2) * rowHeight;
      return {
        id: `step-${i}`,
        type: "stepNode",
        position: { x: groupIdx * colWidth, y },
        sourcePosition: Position.Right,
        targetPosition: Position.Left,
        draggable: false,
        data: { name: step.name, index: i, status: stepStatus(i, currentStep, phase, groups), labels: step.labels },
      };
    });
  }, [steps, currentStep, phase, groups]);

  const computedEdges = useMemo<Edge[]>(() => {
    const edges: Edge[] = [];
    for (let gi = 0; gi < groups.length - 1; gi++) {
      for (const srcIdx of groups[gi]) {
        for (const dstIdx of groups[gi + 1]) {
          const srcStatus = stepStatus(srcIdx, currentStep, phase, groups);
          const isFlowing = srcStatus === "completed" || srcStatus === "active";
          const isFailed = srcStatus === "failed";
          const strokeColor = isFailed
            ? theme.colors.red[6]
            : isFlowing
              ? theme.colors.green[6]
              : theme.colors.dark[4];
          edges.push({
            id: `e-${srcIdx}-${dstIdx}`,
            source: `step-${srcIdx}`,
            target: `step-${dstIdx}`,
            type: "smoothstep",
            animated: isFlowing,
            markerEnd: { type: MarkerType.ArrowClosed, color: strokeColor },
            style: { stroke: strokeColor, strokeWidth: 2 },
          });
        }
      }
    }
    return edges;
  }, [steps, currentStep, phase, theme, groups]);

  const [nodes, setNodes, onNodesChange] = useNodesState(computedNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(computedEdges);

  useEffect(() => {
    setNodes(computedNodes);
  }, [computedNodes, setNodes]);

  useEffect(() => {
    setEdges(computedEdges);
  }, [computedEdges, setEdges]);

  const handleFit = useCallback(() => {
    fitView({ padding: 0.3, minZoom: 0.5, maxZoom: 1.5, duration: 600 });
  }, [fitView]);

  useEffect(() => {
    if (!wrapperRef.current) return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.width > 0 && entry.contentRect.height > 0) {
          window.requestAnimationFrame(handleFit);
        }
      }
    });
    observer.observe(wrapperRef.current);
    return () => observer.disconnect();
  }, [handleFit]);

  return (
    <Box
      ref={wrapperRef}
      h={graphHeight}
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

export default function Rollout() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const initial = useLoaderData<typeof clientLoader>();
  const rollout = useSSEItem<RolloutType>(
    "/apis/magosproject.io/v1alpha1/rollouts/events",
    initial,
    (obj) => obj.metadata?.namespace === namespace && obj.metadata?.name === name
  );

  const steps = (rollout.spec?.strategy?.steps ?? []).map((s) => ({
    name: s.name ?? "",
    labels: s.selector?.matchLabels ?? {},
  }));
  const currentStep = rollout.status?.currentStep ?? 0;
  const phase = rollout.status?.phase ?? "";
  const totalSteps = steps.length;

  const completedSteps = phase === "Applied" ? totalSteps : currentStep;

  return (
    <Stack gap="lg">
      <Breadcrumbs crumbs={[{ label: "Rollouts", to: "/rollouts" }, { label: name! }]} />

      <Group justify="space-between" align="center">
        <Group gap="xs" align="center">
          <Title order={2}>{name}</Title>
          <KubeBadge label={namespace!} />
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
                <StatusBadge status={phase} size="md" />
              </InfoCard>
              <InfoCard label="Project">
                <Text size="sm" c="dimmed">
                  {rollout.spec?.projectRef ?? "—"}
                </Text>
              </InfoCard>
              <InfoCard label="Progress">
                <Text size="sm">
                  {completedSteps}/{totalSteps} completed
                </Text>
              </InfoCard>
              {rollout.status?.reason && (
                <InfoCard label="Reason">
                  <Text size="sm" c="dimmed">
                    {rollout.status.reason}
                  </Text>
                </InfoCard>
              )}
            </SimpleGrid>
            {rollout.status?.message && (
              <Text size="sm" c="dimmed" fs="italic">
                {rollout.status.message}
              </Text>
            )}
          </Stack>
        </Tabs.Panel>

        <Tabs.Panel value="steps" pt="md">
          <ReactFlowProvider>
            <StepPipelineGraph steps={steps} currentStep={currentStep} phase={phase} />
          </ReactFlowProvider>
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}
