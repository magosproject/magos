import { Stack, Group } from "@mantine/core";
import { useLoaderData } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import PageTagline from "../components/PageTagline";
import ResourceList from "../components/ResourceList";
import { apiUrl } from "../api/base";
import WorkspaceCard, {
  type WorkspaceItem,
  toWorkspaceItem,
  workspaceColumns,
} from "../components/WorkspaceCard";
import { flashColorVar } from "../utils/colors";
import apiClient from "../api/client";
import type { Workspace } from "../api/types";
import { useSSEList } from "../hooks/useSSEList";
import type { CSSProperties } from "react";

export function meta() {
  return [{ title: "Workspaces – magos" }];
}

export async function clientLoader() {
  const { data } = await apiClient.GET("/apis/magosproject.io/v1alpha1/workspaces");
  return (data ?? []).map(toWorkspaceItem);
}

export default function Workspaces() {
  const initial = useLoaderData<typeof clientLoader>();
  const [workspaces, changedIds] = useSSEList<Workspace, WorkspaceItem>(
    apiUrl("/apis/magosproject.io/v1alpha1/workspaces/events"),
    initial,
    toWorkspaceItem,
    clientLoader
  );

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Workspaces" }]} />
      <Group justify="space-between" align="center">
        <PageTagline text="// where states mutate" />
      </Group>
      <ResourceList
        items={workspaces}
        getSearchText={(ws) => ws.metadata?.name ?? ""}
        columns={workspaceColumns}
        renderCard={(ws) => <WorkspaceCard workspace={ws} flash={changedIds.has(ws.id)} />}
        toHref={(ws) => `/workspaces/${ws.metadata?.namespace}/${ws.metadata?.name}`}
        flashIds={changedIds}
        getFlashStyle={(ws) => ({ "--flash-color": flashColorVar(ws.status?.phase ?? "") }) as CSSProperties}
      />
    </Stack>
  );
}
