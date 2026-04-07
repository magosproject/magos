import type { CSSProperties } from "react";
import { Text } from "@mantine/core";
import ResourceCard from "./ResourceCard";
import { flashColorVar } from "../utils/colors";
import type { Phase } from "../api/types";

export type StepStatus = "completed" | "active" | "failed" | "pending";

const statusConfig: Record<StepStatus, { color: string; label: string }> = {
  completed: { color: "green", label: "Completed" },
  active: { color: "magos", label: "In progress" },
  failed: { color: "red", label: "Failed" },
  pending: { color: "gray", label: "Pending" },
};

interface RolloutStepCardProps {
  name: string;
  index: number;
  status: StepStatus;
  labels: Record<string, string>;
  flash?: boolean;
  phase?: Phase;
}

export default function RolloutStepCard({ name, index, status, labels, flash, phase }: RolloutStepCardProps) {
  const config = statusConfig[status];
  const spinning = status === "active";

  return (
    <ResourceCard
      to="#"
      title={name}
      description={`Step ${index + 1} · ${config.label}`}
      statusColor={config.color}
      borderAll
      width={280}
      badges={[{ label: config.label, color: config.color, spinning }]}
      meta={Object.entries(labels).map(([k, v]) => ({
        label: (
          <Text size="xs" c="dimmed" ff="monospace" truncate>
            <Text size="xs" c="magos.4" component="span" ff="monospace">
              {k.replace("magosproject.io/", "")}
            </Text>
            <Text size="xs" c="dimmed" component="span" ff="monospace">
              ={v}
            </Text>
          </Text>
        ),
      }))}
      flashStyle={
        flash ? ({ "--flash-color": flashColorVar(phase ?? "") } as CSSProperties) : undefined
      }
    />
  );
}


