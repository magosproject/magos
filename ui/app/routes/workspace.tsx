import {
  Anchor,
  Badge,
  Button,
  Group,
  SimpleGrid,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { IconFolder, IconRefresh } from "@tabler/icons-react";
import type { CSSProperties } from "react";
import { Link, useLoaderData, useParams } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import InfoCard from "../components/InfoCard";
import StatusBadge from "../components/StatusBadge";
import KubeBadge from "../components/KubeBadge";
import ConditionsTable from "../components/ConditionsTable";
import ProjectLineageGraph from "../components/ProjectLineageGraph";
import { repoIcon } from "../utils/repoIcon";
import apiClient from "../api/client";
import type { Project, Workspace as WorkspaceType } from "../api/types";
import { useSSEItem } from "../hooks/useSSEItem";
import { useFlashOnChange } from "../hooks/useFlashOnChange";
import { flashColorVar } from "../utils/colors";

export function meta({ params }: { params: { namespace: string; name: string } }) {
  return [{ title: `${params.name} – magos` }];
}

export async function clientLoader({
  params,
}: {
  params: { namespace: string; name: string };
}) {
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

  return { workspace: ws, project };
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

function revisionUrl(repoURL: string, revision: string): string | null {
  if (!repoURL || !revision) return null;
  const base = repoURL.replace(/\.git$/, "");
  if (base.includes("github.com") || base.includes("gitlab.com") || base.includes("gitlab."))
    return `${base}/tree/${revision}`;
  if (base.includes("bitbucket.org")) return `${base}/src/${revision}`;
  return null;
}

function terraformReleaseUrl(version: string): string | null {
  if (!version) return null;
  return `https://releases.hashicorp.com/terraform/${version}`;
}

export default function Workspace() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const initial = useLoaderData<typeof clientLoader>();
  const ws = useSSEItem<WorkspaceType>(
    "/apis/magosproject.io/v1alpha1/workspaces/events",
    initial.workspace,
    (obj) => obj.metadata?.namespace === namespace && obj.metadata?.name === name
  );

  const project = initial.project;
  const variableSetRefs = (project?.spec?.variableSetRef ?? []).map((ref) => ref.name ?? "");

  const repoURL = ws.spec?.source?.repoURL ?? "";
  const revision = ws.spec?.source?.targetRevision ?? "";
  const tfVersion = ws.spec?.terraform?.version ?? "";
  const projectName = ws.spec?.projectRef?.name ?? "";
  const phase = ws.status?.phase ?? "";
  const flash = useFlashOnChange(phase);
  const flashStyle = { "--flash-color": flashColorVar(phase) } as CSSProperties;

  return (
    <Stack gap="lg">
      <Breadcrumbs crumbs={[{ label: "Workspaces", to: "/workspaces" }, { label: name! }]} />

      <Group justify="space-between" align="flex-start">
        <Group gap="xs" align="center">
          <Title order={2}>{name}</Title>
          <KubeBadge label={namespace!} />
        </Group>
        <Button leftSection={<IconRefresh size={16} />} variant="default" size="sm">
          Reconcile
        </Button>
      </Group>

      <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="md">
        <InfoCard
          label="Status"
          className={flash ? "flash-highlight" : undefined}
          style={flashStyle}
        >
          <StatusBadge status={phase} size="md" />
        </InfoCard>

        <InfoCard label="Project">
          {projectName ? (
            <Anchor component={Link} to={`/projects/${namespace}/${projectName}`} size="sm">
              {projectName}
            </Anchor>
          ) : (
            <Text size="sm" c="dimmed" fs="italic">
              None
            </Text>
          )}
        </InfoCard>

        <InfoCard label="Repository">
          <Group gap={6} wrap="nowrap">
            {repoIcon(repoURL, 14)}
            <Anchor href={repoURL} target="_blank" size="sm" truncate>
              {repoURL.replace(/^https?:\/\//, "")}
            </Anchor>
          </Group>
        </InfoCard>

        <InfoCard label="Path">
          <Group gap={6} wrap="nowrap">
            <IconFolder size={14} />
            <Text size="sm" c="dimmed" truncate>
              {ws.spec?.source?.path ?? "—"}
            </Text>
          </Group>
        </InfoCard>

        <InfoCard label="Revision">
          {revision ? (
            (() => {
              const href = revisionUrl(repoURL, revision);
              return href ? (
                <Anchor href={href} target="_blank" size="sm" truncate>
                  {revision}
                </Anchor>
              ) : (
                <Text size="sm" c="dimmed" truncate>{revision}</Text>
              );
            })()
          ) : (
            <Text size="sm" c="dimmed">—</Text>
          )}
        </InfoCard>

        <InfoCard label="Terraform Version">
          {tfVersion ? (
            (() => {
              const href = terraformReleaseUrl(tfVersion);
              return href ? (
                <Anchor href={href} target="_blank" size="sm">
                  {tfVersion}
                </Anchor>
              ) : (
                <Text size="sm" c="dimmed">{tfVersion}</Text>
              );
            })()
          ) : (
            <Text size="sm" c="dimmed">—</Text>
          )}
        </InfoCard>

        <InfoCard label="Auto Apply">
          <Badge color={ws.spec?.autoApply ? "magos" : "gray"} variant="light" size="sm">
            {ws.spec?.autoApply ? "enabled" : "disabled"}
          </Badge>
        </InfoCard>

        {ws.status?.lastReconcileTime && (
          <InfoCard label="Last reconcile">
            <Text size="sm">
              {formatDate(ws.status.lastReconcileTime)}
            </Text>
          </InfoCard>
        )}
      </SimpleGrid>

      {ws.status?.message && (
        <Text size="sm" c="dimmed" fs="italic">
          {ws.status.message}
        </Text>
      )}

      {ws.status?.conditions && ws.status.conditions.length > 0 && (
        <ConditionsTable conditions={ws.status.conditions} />
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
          />
        </Stack>
      )}
    </Stack>
  );
}
