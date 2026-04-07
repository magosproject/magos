import { Box, Group, Stack, Text, Tooltip } from "@mantine/core";
import type { CSSProperties } from "react";
import { useLoaderData } from "react-router";
import { resourceId, resourceName, resourceNamespace } from "../api/resource";
import Breadcrumbs from "../components/Breadcrumbs";
import PageTagline from "../components/PageTagline";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import StatusBadge from "../components/StatusBadge";
import { statusColor, flashColorVar } from "../utils/colors";
import apiClient from "../api/client";
import type { Rollout, RolloutStep } from "../api/types";
import { useSSEList } from "../hooks/useSSEList";

export function meta() {
  return [{ title: "Rollouts – magos" }];
}

type RolloutRow = {
  id: string;
  name: string;
  namespace: string;
  phase: string;
  projectRef: string;
  currentStep: number;
  totalSteps: number;
  steps: RolloutStep[];
};

export async function clientLoader() {
  const { data } = await apiClient.GET("/apis/magosproject.io/v1alpha1/rollouts");
  return (data ?? []).map(toRolloutRow);
}

function toRolloutRow(ro: Rollout): RolloutRow {
  return {
    id: resourceId(ro),
    name: resourceName(ro),
    namespace: resourceNamespace(ro),
    phase: ro.status?.phase ?? "",
    projectRef: ro.spec?.projectRef ?? "",
    currentStep: ro.status?.currentStep ?? 0,
    totalSteps: ro.spec?.strategy?.steps?.length ?? 0,
    steps: ro.spec?.strategy?.steps ?? [],
  };
}

function StepPipeline({ rollout }: { rollout: RolloutRow }) {
  return (
    <Group gap={0} wrap="nowrap" align="center">
      {rollout.steps.map((step, i) => {
        const isComplete = i < rollout.currentStep || rollout.phase === "Applied";
        const isActive = rollout.phase === "Reconciling" && i === rollout.currentStep;
        const isFailed = rollout.phase === "Failed" && i === rollout.currentStep;
        const stepName = step.name ?? `Step ${i + 1}`;

        let color = "var(--mantine-color-dark-4)";
        if (isComplete) color = "var(--mantine-color-green-6)";
        if (isActive) color = `var(--mantine-color-${statusColor[rollout.phase]}-6)`;
        if (isFailed) color = "var(--mantine-color-red-6)";

        const connectorColor =
          i > 0 && (i <= rollout.currentStep || rollout.phase === "Applied")
            ? "var(--mantine-color-green-6)"
            : "var(--mantine-color-dark-4)";

        return (
          <Group key={`${stepName}-${i}`} gap={0} wrap="nowrap" align="center">
            {i > 0 && (
              <Box className="step-connector" style={{ backgroundColor: connectorColor }} />
            )}
            <Tooltip label={stepName} withArrow position="top">
              <Box
                className={`step-node${isActive ? " pulse" : ""}`}
                style={
                  {
                    backgroundColor: color,
                    "--pulse-color": isActive ? color : undefined,
                  } as CSSProperties
                }
              />
            </Tooltip>
          </Group>
        );
      })}
    </Group>
  );
}

const columns: ColumnDef<RolloutRow>[] = [
  {
    key: "name",
    label: "Name",
    sortField: "name",
    render: (ro) => (
      <Text size="sm" fw={500}>
        {ro.name}
      </Text>
    ),
  },
  {
    key: "phase",
    label: "Phase",
    render: (ro) => <StatusBadge status={ro.phase} />,
  },
  {
    key: "project",
    label: "Project",
    render: (ro) => (
      <Text size="sm" c="dimmed">
        {ro.projectRef || "—"}
      </Text>
    ),
  },
  {
    key: "pipeline",
    label: "Steps",
    render: (ro) => (
      <Group gap="sm" align="center" wrap="nowrap">
        <StepPipeline rollout={ro} />
        <Text size="xs" c="dimmed" style={{ whiteSpace: "nowrap" }}>
          {ro.phase === "Applied" ? ro.totalSteps : ro.currentStep}/{ro.totalSteps}
        </Text>
      </Group>
    ),
  },
];

export default function Rollouts() {
  const initial = useLoaderData<typeof clientLoader>();
  const [rollouts, changedIds] = useSSEList<Rollout, RolloutRow>(
    "/apis/magosproject.io/v1alpha1/rollouts/events",
    initial,
    toRolloutRow,
    clientLoader
  );

  const getFlashStyle = (ro: RolloutRow) =>
    ({ "--flash-color": flashColorVar(ro.phase) }) as CSSProperties;

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Rollouts" }]} />
      <PageTagline text="// sequential precision" />
      <ResourceList
        items={rollouts}
        searchKey="name"
        columns={columns}
        toHref={(ro) => `/rollouts/${ro.namespace}/${ro.name}`}
        defaultView="row"
        hideViewToggle
        flashIds={changedIds}
        getFlashStyle={getFlashStyle}
      />
    </Stack>
  );
}
