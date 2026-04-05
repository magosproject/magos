import { Box, Group, Stack, Text, Tooltip } from "@mantine/core";
import Breadcrumbs from "../components/Breadcrumbs";
import { type Rollout, rollouts } from "../mock-data/rollouts";
import { projects } from "../mock-data/projects";
import ResourceList, { type ColumnDef } from "../components/ResourceList";
import StatusBadge from "../components/StatusBadge";
import KubeBadge from "../components/KubeBadge";
import { statusColor } from "../utils/colors";

export function meta() {
  return [{ title: "Rollouts – magos" }];
}

/** Small inline pipeline visualization showing step progress as colored dots. */
function StepPipeline({ rollout }: { rollout: Rollout }) {
  return (
    <Group gap={0} wrap="nowrap" align="center">
      {rollout.steps.map((step, i) => {
        const isComplete = i < rollout.currentStep || rollout.phase === "Applied";
        const isActive = rollout.phase === "Reconciling" && i === rollout.currentStep;
        const isFailed = rollout.phase === "Failed" && i === rollout.currentStep;

        let color = "var(--mantine-color-dark-4)"; // pending
        if (isComplete) color = "var(--mantine-color-green-6)";
        if (isActive) color = `var(--mantine-color-${statusColor[rollout.phase]}-6)`;
        if (isFailed) color = "var(--mantine-color-red-6)";

        // Connector color: solid if left node is complete, dim otherwise
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
                  } as React.CSSProperties
                }
              />
            </Tooltip>
          </Group>
        );
      })}
    </Group>
  );
}

const columns: ColumnDef<Rollout>[] = [
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
    render: (ro) => {
      const project = projects.find((p) => p.id === ro.projectRef);
      return (
        <Text size="sm" c="dimmed">
          {project?.name ?? ro.projectRef}
        </Text>
      );
    },
  },
  {
    key: "pipeline",
    label: "Steps",
    render: (ro) => (
      <Group gap="sm" align="center" wrap="nowrap">
        <StepPipeline rollout={ro} />
        <Text size="xs" c="dimmed" style={{ whiteSpace: "nowrap" }}>
          {Math.min(ro.currentStep + 1, ro.steps.length)}/{ro.steps.length}
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
        toHref={(ro) => `/rollouts/${ro.id}`}
        defaultView="row"
        hideViewToggle
      />
    </Stack>
  );
}
