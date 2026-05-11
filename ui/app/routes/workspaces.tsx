import { useEffect, useMemo } from "react";
import { Stack, Group } from "@mantine/core";
import { useLoaderData } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import PageTagline from "../components/PageTagline";
import ResourceList, {
  type FilterGroup,
} from "../components/ResourceList";
import { apiUrl } from "../api/base";
import WorkspaceCard, {
  type WorkspaceItem,
  toWorkspaceItem,
  workspaceColumns,
} from "../components/WorkspaceCard";
import { flashColorVar, statusColorFor } from "../utils/colors";
import apiClient from "../api/client";
import type { Workspace } from "../api/types";
import { useSSEList } from "../hooks/useSSEList";
import { setWorkspaceCache } from "../api/workspaceCache";
import type { CSSProperties } from "react";

export function meta() {
  return [{ title: "Workspaces – magos" }];
}

export async function clientLoader() {
  const { data } = await apiClient.GET("/apis/magosproject.io/v1alpha1/workspaces");
  return (data ?? []).map(toWorkspaceItem);
}

const PAGE_SIZE = 24;

export default function Workspaces() {
  const initial = useLoaderData<typeof clientLoader>();
  const [workspaces, changedIds] = useSSEList<Workspace, WorkspaceItem>(
    apiUrl("/apis/magosproject.io/v1alpha1/workspaces/events"),
    initial,
    toWorkspaceItem,
    clientLoader
  );

  // Keep the module-level cache warm so navigating to a workspace detail
  // page can skip the workspace GET entirely.
  useEffect(() => {
    workspaces.forEach((ws) => setWorkspaceCache(ws));
  }, [workspaces]);

  const phaseOptions = useMemo(() => {
    const counts = new Map<string, number>();
    for (const ws of workspaces) {
      const p = ws.status?.phase;
      if (p) counts.set(p, (counts.get(p) ?? 0) + 1);
    }
    return [...counts.entries()].map(([phase, count]) => ({
      value: phase,
      color: statusColorFor(phase),
      count,
    }));
  }, [workspaces]);

  const projectOptions = useMemo(() => {
    const seen = new Set<string>();
    for (const ws of workspaces) {
      const p = ws.spec?.projectRef?.name;
      if (p) seen.add(p);
    }
    return [...seen].sort().map((p) => ({ value: p, color: "violet" as const }));
  }, [workspaces]);

  const filterGroups = useMemo<FilterGroup<WorkspaceItem>[]>(
    () => [
      {
        key: "status",
        label: "Status",
        options: phaseOptions,
        match: (ws, selected) => selected.includes(ws.status?.phase ?? ""),
      },
      {
        key: "project",
        label: "Project",
        options: projectOptions,
        match: (ws, selected) => selected.includes(ws.spec?.projectRef?.name ?? ""),
        variant: "multiselect",
      },
    ],
    [phaseOptions, projectOptions]
  );

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Workspaces" }]} />
      <Group justify="space-between" align="center">
        <PageTagline text="// where states mutate" />
      </Group>
      <ResourceList
        items={workspaces}
        getSearchText={(ws) =>
          [
            ws.metadata?.name,
            ws.spec?.projectRef?.name,
            ws.spec?.source?.repoURL,
            ws.spec?.source?.path,
          ]
            .filter(Boolean)
            .join(" ")
        }
        filterGroups={filterGroups}
        columns={workspaceColumns}
        renderCard={(ws) => <WorkspaceCard workspace={ws} flash={changedIds.has(ws.id)} />}
        toHref={(ws) => `/workspaces/${ws.metadata?.namespace}/${ws.metadata?.name}`}
        flashIds={changedIds}
        getFlashStyle={(ws) => ({ "--flash-color": flashColorVar(ws.status?.phase ?? "") }) as CSSProperties}
        pageSize={PAGE_SIZE}
        noun="workspace"
      />
    </Stack>
  );
}
