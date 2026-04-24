import {
  Anchor,
  Badge,
  Button,
  Group,
  SimpleGrid,
  Stack,
  Tabs,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import { Link, useLoaderData, useParams } from "react-router";
import type { CSSProperties } from "react";
import { IconRefresh } from "@tabler/icons-react";
import Breadcrumbs from "~/components/Breadcrumbs";
import InfoCard from "~/components/InfoCard";
import StatusBadge from "~/components/StatusBadge";
import KubeBadge from "~/components/KubeBadge";
import ProjectLineageGraph from "~/components/ProjectLineageGraph";
import ResourceList from "~/components/ResourceList";
import WorkspaceCard, { toWorkspaceItem, workspaceColumns } from "~/components/WorkspaceCard";
import { apiUrl } from "~/api/base";
import apiClient from "~/api/client";
import type { Project as ProjectType, Workspace } from "~/api/types";
import { useSSEItem } from "~/hooks/useSSEItem";
import { useSSEFiltered } from "~/hooks/useSSEFiltered";
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
  const [projectRes, workspacesRes] = await Promise.all([
    apiClient.GET("/apis/magosproject.io/v1alpha1/projects/{namespace}/{name}", {
      params: { path: { namespace: params.namespace, name: params.name } },
    }),
    apiClient.GET("/apis/magosproject.io/v1alpha1/workspaces"),
  ]);

  if (!projectRes.data) throw new Response("Not found", { status: 404 });

  const projectWorkspaces = (workspacesRes.data ?? []).filter(
    (ws) => ws.spec?.projectRef?.name === params.name
  );

  return { project: projectRes.data, workspaces: projectWorkspaces };
}

export default function Project() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const initial = useLoaderData<typeof clientLoader>();

  const project = useSSEItem<ProjectType>(
    apiUrl("/apis/magosproject.io/v1alpha1/projects/events"),
    initial.project,
    (obj) => obj.metadata?.namespace === namespace && obj.metadata?.name === name
  );

  const [workspaces, wsChangedIds] = useSSEFiltered<Workspace>(
    apiUrl(`/apis/magosproject.io/v1alpha1/workspaces/events?projectRef=${name}`),
    initial.workspaces
  );

  const variableSetRefs = (project.spec?.variableSetRef ?? []).map((ref) => ref.name ?? "");
  const workspaceItems = workspaces.map(toWorkspaceItem);

  const phase = project.status?.phase ?? "";
  const flash = useFlashOnChange(phase);
  const flashStyle = { "--flash-color": flashColorVar(phase) } as CSSProperties;

  return (
    <Stack gap="lg">
      <Breadcrumbs crumbs={[{ label: "Projects", to: "/projects" }, { label: name! }]} />

      <Group justify="space-between" align="flex-start">
        <Stack gap={4}>
          <Group gap="xs" align="center">
            <Title order={2}>{name}</Title>
            <KubeBadge label={namespace!} />
          </Group>
          {project.spec?.description && (
            <Text size="sm" c="dimmed">
              {project.spec.description}
            </Text>
          )}
        </Stack>
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
          <Tabs.Tab value="workspaces">Workspaces ({workspaces.length})</Tabs.Tab>
          <Tabs.Tab value="variable-sets">Variable Sets ({variableSetRefs.length})</Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="overview" pt="md">
          <Stack gap="md">
            <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="md">
              <InfoCard label="Status" className={flash ? "flash-highlight" : undefined} style={flashStyle}>
                <StatusBadge status={phase} size="md" />
              </InfoCard>
              <InfoCard label="Workspaces">
                <Text size="sm">{workspaces.length}</Text>
              </InfoCard>
              <InfoCard label="Variable Sets">
                <Text size="sm">{variableSetRefs.length}</Text>
              </InfoCard>
            </SimpleGrid>

            {(workspaces.length > 0 || variableSetRefs.length > 0) && (
              <Stack gap="xs">
                <Title order={4}>Inheritance Lineage</Title>
                <Text size="sm" c="dimmed">
                  Variable sets flow into the project and are inherited by its workspaces.
                </Text>
                <ProjectLineageGraph
                  project={project}
                  variableSetRefs={variableSetRefs}
                  workspaces={workspaces}
                  flashIds={wsChangedIds}
                />
              </Stack>
            )}
          </Stack>
        </Tabs.Panel>

        <Tabs.Panel value="workspaces" pt="md">
          {workspaceItems.length === 0 ? (
            <Text size="sm" c="dimmed">
              No workspaces linked to this project.
            </Text>
          ) : (
            <ResourceList
              items={workspaceItems}
              getSearchText={(ws) => ws.metadata?.name ?? ""}
              columns={workspaceColumns}
              renderCard={(ws) => <WorkspaceCard workspace={ws} flash={wsChangedIds.has(ws.id)} />}
              toHref={(ws) => `/workspaces/${ws.metadata?.namespace}/${ws.metadata?.name}`}
              flashIds={wsChangedIds}
              getFlashStyle={(ws) => ({ "--flash-color": flashColorVar(ws.status?.phase ?? "") }) as CSSProperties}
              defaultView="row"
            />
          )}
        </Tabs.Panel>

        <Tabs.Panel value="variable-sets" pt="md">
          {variableSetRefs.length === 0 ? (
            <Text size="sm" c="dimmed">
              No variable sets linked to this project.
            </Text>
          ) : (
            <Group gap="xs">
              {variableSetRefs.map((vsName) => (
                <Anchor
                  key={vsName}
                  component={Link}
                  to={`/variable-sets/${namespace}/${vsName}`}
                  underline="never"
                >
                  <Badge variant="light" color="magos" size="sm" style={{ cursor: "pointer" }}>
                    {vsName}
                  </Badge>
                </Anchor>
              ))}
            </Group>
          )}
        </Tabs.Panel>
      </Tabs>
    </Stack>
  );
}
