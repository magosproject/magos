import { Box, Group, Stack, Text, Tooltip } from "@mantine/core";
import type { CSSProperties } from "react";
import { useLoaderData } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import StatusBadge from "../components/StatusBadge";
import KubeBadge from "../components/KubeBadge";
import { statusColor } from "../utils/colors";
import apiClient from "../api/client";

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
  steps: { name: string }[];
};

export async function clientLoader() {
  const { data } = await apiClient.GET("/apis/magosproject.io/v1alpha1/rollouts");
  return (data ?? []).map(
    (ro): RolloutRow => ({
      id: ro.metadata?.uid ?? `${ro.metadata?.namespace}/${ro.metadata?.name}`,
      name: ro.metadata?.name ?? "",
      namespace: ro.metadata?.namespace ?? "",
      phase: ro.status?.phase ?? "",
      projectRef: ro.spec?.projectRef ?? "",
      currentStep: ro.status?.currentStep ?? 0,
      totalSteps: ro.spec?.strategy?.steps?.length ?? 0,
      steps: ro.spec?.strategy?.steps?.map((s) => ({ name: s.name ?? "" })) ?? [],
    })
  );
}

function StepPipeline({ rollout }: { rollout: RolloutRow }) {
  return (
    <Group gap={0} wrap="nowrap" align="center">
      {rollout.steps.map((step, i) => {
        const isComplete = i < rollout.currentStep || rollout.phase === "Applied";
        const isActive = rollout.phase === "Reconciling" && i === rollout.currentStep;
        const isFailed = rollout.phase === "Failed" && i === rollout.currentStep;

        let color = "var(--mantine-color-dark-4)";
        if (isComplete) color = "var(--mantine-color-green-6)";
        if (isActive) color = `var(--mantine-color-${statusColor[rollout.phase]}-6)`;
        if (isFailed) color = "var(--mantine-color-red-6)";

        const connectorColor =
          i > 0 && (i <= rollout.currentStep || rollout.phase === "Applied")
            ? "var(--mantine-color-green-6)"
            : "var(--mantine-color-dark-4)";

        return (
          <Group key={step.name} gap={0} wrap="nowrap" align="center">
            {i > 0 && (
              <Box className="step-connector" style={{ backgroundColor: connectorColor }} />
            )}
            <Tooltip label={step.name} withArrow position="top">
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
          {Math.min(ro.currentStep + 1, ro.totalSteps)}/{ro.totalSteps}
        </Text>
      </Group>
    ),
  },
  {
    key: "namespace",
    label: "Kubernetes Namespace",
    sortField: "namespace",
    render: (ro) => <KubeBadge label={ro.namespace} />,
  },
];

export default function Rollouts() {
  const rollouts = useLoaderData<typeof clientLoader>();

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Rollouts" }]} />
      <Group gap={4} align="center">
        <Text
          size="xl"
          fw={700}
          variant="gradient"
          gradient={{ from: "magos.4", to: "magos.7", deg: 45 }}
          style={{ fontFamily: "monospace", letterSpacing: -0.5 }}
        >
          // sequential precision
        </Text>
        <Text
          className="blinking-cursor"
          size="xl"
          fw={700}
          c="magos.5"
          style={{ fontFamily: "monospace" }}
        >
          _
        </Text>
      </Group>
      <ResourceList
        items={rollouts}
        searchKey="name"
        columns={columns}
        toHref={(ro) => `/rollouts/${ro.namespace}/${ro.name}`}
        defaultView="row"
        hideViewToggle
      />
    </Stack>
  );
}
