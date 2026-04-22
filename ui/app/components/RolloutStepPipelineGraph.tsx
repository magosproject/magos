import { useMemo } from "react";
import { useMantineTheme } from "@mantine/core";
import { type Edge, MarkerType, type Node, Position } from "@xyflow/react";
import type { Phase, RolloutStep } from "../api/types";
import type { LineageNodeData } from "./LineageNode";
import LineageNode from "./LineageNode";
import FlowGraph from "./FlowGraph";
import RolloutStepCard from "./RolloutStepCard";
import { groupStepsByLabels, rolloutStepStatus, selectorLabels } from "../utils/rollouts";

const nodeTypes = { lineageNode: LineageNode };

interface RolloutStepPipelineGraphProps {
  steps: RolloutStep[];
  currentStep: number;
  phase?: Phase;
  flash?: boolean;
}

export default function RolloutStepPipelineGraph({
  steps,
  currentStep,
  phase,
  flash,
}: RolloutStepPipelineGraphProps) {
  const theme = useMantineTheme();

  const nodeWidth = 280;
  const colWidth = 380;
  const rowHeight = 150;

  const groups = useMemo(() => groupStepsByLabels(steps), [steps]);
  const graphHeight = Math.max(180, Math.max(...groups.map((group) => group.length)) * rowHeight + 40);

  const nodes = useMemo<Node<LineageNodeData>[]>(() => {
    return steps.map((step, index) => {
      const groupIdx = groups.findIndex((group) => group.includes(index));
      const posInGroup = groups[groupIdx].indexOf(index);
      const groupSize = groups[groupIdx].length;
      const y = (posInGroup - (groupSize - 1) / 2) * rowHeight;
      const status = rolloutStepStatus(index, currentStep, phase, groups);
      const isActive = status === "active" || status === "failed";

      return {
        id: `step-${index}`,
        type: "lineageNode",
        position: { x: groupIdx * colWidth, y },
        sourcePosition: Position.Right,
        targetPosition: Position.Left,
        draggable: false,
        width: nodeWidth,
        data: {
          kindLabel: `Step ${index + 1}`,
          content: (
            <RolloutStepCard
              name={step.name ?? ""}
              index={index}
              status={status}
              labels={selectorLabels(step.selector)}
              flash={flash && isActive}
              phase={phase}
            />
          ),
        },
      };
    });
  }, [steps, currentStep, phase, groups, flash]);

  const edges = useMemo<Edge[]>(() => {
    const result: Edge[] = [];

    for (let groupIdx = 0; groupIdx < groups.length - 1; groupIdx++) {
      for (const srcIdx of groups[groupIdx]) {
        for (const dstIdx of groups[groupIdx + 1]) {
          const srcStatus = rolloutStepStatus(srcIdx, currentStep, phase, groups);
          const dstStatus = rolloutStepStatus(dstIdx, currentStep, phase, groups);
          const isFlowing = srcStatus === "completed" || srcStatus === "active";
          const isFailed = srcStatus === "failed";
          const isActive = dstStatus === "active";
          const strokeColor = isFailed
            ? theme.colors.red[6]
            : isActive
              ? theme.colors.magos[5]
              : isFlowing
                ? theme.colors.green[6]
                : theme.colors.gray[5];

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
