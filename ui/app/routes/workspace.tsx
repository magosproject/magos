import { Button, Group, Stack, Tabs, Text, Title } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { useMemo, useState, type CSSProperties } from "react";
import { useLoaderData, useParams } from "react-router";
import { resourceId } from "../api/resource";
import Breadcrumbs from "../components/Breadcrumbs";
import KubeBadge from "../components/KubeBadge";
import ConditionsTable from "../components/ConditionsTable";
import PolicyViolationsTable from "../components/PolicyViolationsTable";
import ProjectLineageGraph from "../components/ProjectLineageGraph";
import WorkspaceRunHistory from "../components/WorkspaceRunHistory";
import WorkspaceLiveConsole from "../components/WorkspaceLiveConsole";
import WorkspaceOverview from "../components/WorkspaceOverview";
import { apiUrl } from "../api/base";
import apiClient from "../api/client";
import type { Phase, Project, ReconcileRunListResponse, Workspace as WorkspaceType } from "../api/types";
import { useSSEItem } from "../hooks/useSSEItem";
import { useFlashOnChange } from "../hooks/useFlashOnChange";
import { flashColorVar } from "../utils/colors";
import { RECONCILABLE_PHASES } from "../utils/phases";

export function meta({ params }: { params: { namespace: string; name: string } }) {
  return [{ title: `${params.name} – magos` }];
}

export async function clientLoader({ params }: { params: { namespace: string; name: string } }) {
  const { data: ws } = await apiClient.GET(
    "/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}",
    { params: { path: { namespace: params.namespace, name: params.name } } }
  );
  if (!ws) throw new Response("Not found", { status: 404 });

  let project: Project | undefined;
  const projectRef = ws.spec?.projectRef?.name;
  if (projectRef) {
    const { data } = await apiClient.GET(
      "/apis/magosproject.io/v1alpha1/projects/{namespace}/{name}",
      { params: { path: { namespace: params.namespace, name: projectRef } } }
    );
    project = data;
  }

  const { data: runs } = await apiClient.GET(
    "/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs",
    {
      params: {
        path: { namespace: params.namespace, name: params.name },
        query: { limit: 20 },
      },
    }
  );
  const initialRuns: ReconcileRunListResponse = runs ?? { items: [] };

  return { workspace: ws, project, initialRuns };
}

export default function Workspace() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const initial = useLoaderData<typeof clientLoader>();
  const [isSubmittingReconcile, setIsSubmittingReconcile] = useState(false);
  const ws = useSSEItem<WorkspaceType>(
    apiUrl("/apis/magosproject.io/v1alpha1/workspaces/events"),
    initial.workspace,
    (obj) => obj.metadata?.namespace === namespace && obj.metadata?.name === name
  );

  const project = initial.project;
  const variableSetRefs = (project?.spec?.variableSetRef ?? []).map((ref) => ref.name ?? "");

  const projectName = ws.spec?.projectRef?.name ?? "";
  const phase: Phase | undefined = ws.status?.phase;
  const phaseLabel = phase ?? "";
  const flash = useFlashOnChange(phase);
  const flashStyle = { "--flash-color": flashColorVar(phaseLabel) } as CSSProperties;
  const wsId = resourceId(ws);
  const lineageFlashIds = useMemo(
    () => (flash ? new Set([wsId]) : new Set<string>()),
    [flash, wsId]
  );
  const canReconcile = phase ? RECONCILABLE_PHASES.has(phase) : false;
  const reconcileDisabled = isSubmittingReconcile || !canReconcile || !namespace || !name;

  async function handleReconcile() {
    if (!namespace || !name || reconcileDisabled) return;

    setIsSubmittingReconcile(true);
    try {
      await apiClient.POST(
        "/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/reconcile",
        {
          params: { path: { namespace, name } },
        }
      );
    } finally {
      setIsSubmittingReconcile(false);
    }
  }

  return (
    <Stack gap="lg">
      <Breadcrumbs crumbs={[{ label: "Workspaces", to: "/workspaces" }, { label: name! }]} />

      <Group justify="space-between" align="flex-start">
        <Group gap="xs" align="center">
          <Title order={2}>{name}</Title>
          <KubeBadge label={namespace!} />
        </Group>
        <Button
          leftSection={<IconRefresh size={16} />}
          variant="default"
          size="sm"
          disabled={reconcileDisabled}
          loading={isSubmittingReconcile}
          onClick={handleReconcile}
        >
          Reconcile
        </Button>
      </Group>

      <Tabs defaultValue="overview">
        <Tabs.List>
          <Tabs.Tab value="overview">Overview</Tabs.Tab>
          <Tabs.Tab value="runs">Runs</Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="overview" pt="md">
          <Stack gap="lg">
            {namespace && (
              <WorkspaceOverview
                namespace={namespace}
                workspace={ws}
                phaseLabel={phaseLabel}
                projectName={projectName}
                flash={flash}
                flashStyle={flashStyle}
              />
            )}

            {ws.status?.message && (
              <Text size="sm" c="dimmed" fs="italic">
                {ws.status.message}
              </Text>
            )}

            {ws.status?.conditions && ws.status.conditions.length > 0 && (
              <ConditionsTable conditions={ws.status.conditions} />
            )}

            {ws.status?.policyViolations && ws.status.policyViolations.length > 0 && (
              <PolicyViolationsTable violations={ws.status.policyViolations} />
            )}

            {project && (
              <Stack gap="xs">
                <Title order={4}>Inheritance Lineage</Title>
                <Text size="sm" c="dimmed">
                  Variable sets flow into the project and are inherited by this workspace.
                </Text>
                <ProjectLineageGraph
                  project={project}
                  variableSetRefs={variableSetRefs}
                  workspaces={[ws]}
                  flashIds={lineageFlashIds}
                />
              </Stack>
            )}
          </Stack>
        </Tabs.Panel>

        <Tabs.Panel value="runs" pt="md">
          <Stack gap="lg">
            {namespace && name && (
              <WorkspaceLiveConsole
                namespace={namespace}
                workspaceName={name}
                phase={phase}
                currentRunID={ws.status?.currentRunID}
              />
            )}

            {namespace && name && (
              <WorkspaceRunHistory
                namespace={namespace}
                workspaceName={name}
                initialRuns={initial.initialRuns}
                phase={phase}
                currentRunID={ws.status?.currentRunID}
              />
            )}
          </Stack>
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}
