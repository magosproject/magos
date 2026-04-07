import { useMemo, type CSSProperties } from "react";
import {
  Anchor,
  Button,
  Group,
  SimpleGrid,
  Stack,
  Text,
  Title,
  useMantineTheme,
} from "@mantine/core";
import { useLoaderData, useParams, Link } from "react-router";
import { IconRefresh } from "@tabler/icons-react";
import { type Edge, type Node, Position, MarkerType } from "@xyflow/react";
import Breadcrumbs from "~/components/Breadcrumbs";
import InfoCard from "~/components/InfoCard";
import StatusBadge from "~/components/StatusBadge";
import KubeBadge from "~/components/KubeBadge";
import FlowGraph from "~/components/FlowGraph";
import LineageNode, { type LineageNodeData } from "~/components/LineageNode";
import RolloutStepCard, { type StepStatus } from "~/components/RolloutStepCard";
import { Tabs } from "@mantine/core";
import apiClient from "~/api/client";
import type { LabelSelector, Rollout as RolloutType, RolloutStep, Phase } from "~/api/types";
import { useSSEItem } from "~/hooks/useSSEItem";
import { useFlashOnChange } from "~/hooks/useFlashOnChange";
import { flashColorVar } from "~/utils/colors";

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

function stepStatus(index: number, currentStep: number, phase: Phase | undefined, groups: number[][]): StepStatus {
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

function selectorLabels(selector?: LabelSelector): Record<string, string> {
  return selector?.matchLabels ?? {};
}

function groupStepsByLabels(steps: RolloutStep[]): number[][] {
  const toKey = (labels: Record<string, string>) =>
    JSON.stringify(Object.entries(labels).sort((a, b) => a[0].localeCompare(b[0])));

  const groups: number[][] = [];
  const seen = new Map<string, number>();

  for (let i = 0; i < steps.length; i++) {
    const key = toKey(selectorLabels(steps[i].selector));
    if (!seen.has(key)) {
      seen.set(key, groups.length);
      groups.push([i]);
    } else {
      groups[seen.get(key)!].push(i);
    }
  }

  return groups;
}

const nodeTypes = { lineageNode: LineageNode };

function StepPipelineGraph({
  steps,
  currentStep,
  phase,
  flash,
}: {
  steps: RolloutStep[];
  currentStep: number;
  phase?: Phase;
  flash?: boolean;
}) {
  const theme = useMantineTheme();

  const nodeWidth = 280;
  const colWidth = 380;
  const rowHeight = 150;

  const groups = useMemo(() => groupStepsByLabels(steps), [steps]);
  const graphHeight = Math.max(180, Math.max(...groups.map((g) => g.length)) * rowHeight + 40);

  const nodes = useMemo<Node<LineageNodeData>[]>(() => {
    return steps.map((step, i) => {
      const groupIdx = groups.findIndex((g) => g.includes(i));
      const posInGroup = groups[groupIdx].indexOf(i);
      const groupSize = groups[groupIdx].length;
      const y = (posInGroup - (groupSize - 1) / 2) * rowHeight;
      const status = stepStatus(i, currentStep, phase, groups);
      const isActive = status === "active" || status === "failed";
      return {
        id: `step-${i}`,
        type: "lineageNode",
        position: { x: groupIdx * colWidth, y },
        sourcePosition: Position.Right,
        targetPosition: Position.Left,
        draggable: false,
        width: nodeWidth,
        data: {
          kindLabel: `Step ${i + 1}`,
          content: (
            <RolloutStepCard
              name={step.name ?? ""}
              index={i}
              status={status}
              labels={selectorLabels(step.selector)}
              flash={flash && isActive}
              phase={phase}
            />
          ),
        },
      };
    });
  }, [steps, currentStep, phase, groups, flash, nodeWidth, colWidth, rowHeight]);

  const edges = useMemo<Edge[]>(() => {
    const result: Edge[] = [];
    for (let gi = 0; gi < groups.length - 1; gi++) {
      for (const srcIdx of groups[gi]) {
        for (const dstIdx of groups[gi + 1]) {
          const srcStatus = stepStatus(srcIdx, currentStep, phase, groups);
          const dstStatus = stepStatus(dstIdx, currentStep, phase, groups);
          const isFlowing = srcStatus === "completed" || srcStatus === "active";
          const isFailed = srcStatus === "failed";
          const isActive = dstStatus === "active";
          const strokeColor = isFailed
            ? theme.colors.red[6]
            : isActive
              ? theme.colors.magos[5]
              : isFlowing
                ? theme.colors.green[6]
                : theme.colors.dark[4];
          result.push({
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
    return result;
  }, [currentStep, phase, theme, groups]);

  return <FlowGraph nodes={nodes} edges={edges} nodeTypes={nodeTypes} height={graphHeight} />;
}

export default function Rollout() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const initial = useLoaderData<typeof clientLoader>();
  const rollout = useSSEItem<RolloutType>(
    "/apis/magosproject.io/v1alpha1/rollouts/events",
    initial,
    (obj) => obj.metadata?.namespace === namespace && obj.metadata?.name === name
  );

  const steps = rollout.spec?.strategy?.steps ?? [];
  const currentStep = rollout.status?.currentStep ?? 0;
  const phase = rollout.status?.phase;
  const totalSteps = steps.length;

  const completedSteps = phase === "Applied" ? totalSteps : currentStep;
  const flash = useFlashOnChange(`${phase}-${currentStep}`);
  const flashStyle = { "--flash-color": flashColorVar(phase ?? "") } as CSSProperties;

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
              <InfoCard label="Phase" className={flash ? "flash-highlight" : undefined} style={flashStyle}>
                <StatusBadge status={phase ?? ""} size="md" />
              </InfoCard>
              <InfoCard label="Project">
                {rollout.spec?.projectRef ? (
                  <Anchor component={Link} to={`/projects/${namespace}/${rollout.spec.projectRef}`} size="sm">
                    {rollout.spec.projectRef}
                  </Anchor>
                ) : (
                  <Text size="sm" c="dimmed">—</Text>
                )}
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
          <StepPipelineGraph steps={steps} currentStep={currentStep} phase={phase} flash={flash} />
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}
