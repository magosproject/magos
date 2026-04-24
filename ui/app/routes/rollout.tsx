import type { CSSProperties } from "react";
import { Button, Group, Stack, Tabs, Title, Tooltip } from "@mantine/core";
import { useLoaderData, useParams } from "react-router";
import { IconRefresh } from "@tabler/icons-react";
import Breadcrumbs from "~/components/Breadcrumbs";
import KubeBadge from "~/components/KubeBadge";
import RolloutOverview from "~/components/RolloutOverview";
import RolloutStepPipelineGraph from "~/components/RolloutStepPipelineGraph";
import { apiUrl } from "~/api/base";
import apiClient from "~/api/client";
import type { Rollout as RolloutType } from "~/api/types";
import { useSSEItem } from "~/hooks/useSSEItem";
import { useFlashOnChange } from "~/hooks/useFlashOnChange";
import { flashColorVar } from "~/utils/colors";
import { completedRolloutSteps } from "~/utils/rollouts";

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

export default function Rollout() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const initial = useLoaderData<typeof clientLoader>();
  const rollout = useSSEItem<RolloutType>(
    apiUrl("/apis/magosproject.io/v1alpha1/rollouts/events"),
    initial,
    (obj) => obj.metadata?.namespace === namespace && obj.metadata?.name === name
  );

  const steps = rollout.spec?.strategy?.steps ?? [];
  const currentStep = rollout.status?.currentStep ?? 0;
  const phase = rollout.status?.phase;
  const totalSteps = steps.length;
  const completedSteps = completedRolloutSteps(totalSteps, currentStep, phase);
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
        <Tooltip label="Only workspace reconcile is supported right now">
          <span>
            <Button leftSection={<IconRefresh size={16} />} variant="default" size="sm" disabled>
              Reconcile
            </Button>
          </span>
        </Tooltip>
      </Group>

      <Tabs defaultValue="overview">
        <Tabs.List>
          <Tabs.Tab value="overview">Overview</Tabs.Tab>
          <Tabs.Tab value="steps">Steps</Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="overview" pt="md">
          <RolloutOverview
            namespace={namespace!}
            rollout={rollout}
            completedSteps={completedSteps}
            totalSteps={totalSteps}
            flash={flash}
            flashStyle={flashStyle}
          />
        </Tabs.Panel>

        <Tabs.Panel value="steps" pt="md">
          <RolloutStepPipelineGraph
            steps={steps}
            currentStep={currentStep}
            phase={phase}
            flash={flash}
          />
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}
