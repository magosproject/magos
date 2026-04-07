import { Stack, Group } from "@mantine/core";
import { useLoaderData } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import PageTagline from "../components/PageTagline";
import ResourceList from "../components/ResourceList";
import apiClient from "../api/client";
import type { Workspace } from "../api/types";
import { useSSEList } from "../hooks/useSSEList";
import {
  type WorkspaceRow,
  toWorkspaceRow,
  workspaceColumns,
  workspaceToCard,
  workspaceToHref,
  workspaceFlashStyle,
} from "../utils/workspace";

export function meta() {
  return [{ title: "Workspaces – magos" }];
}

export async function clientLoader() {
  const { data } = await apiClient.GET("/apis/magosproject.io/v1alpha1/workspaces");
  return (data ?? []).map(toWorkspaceRow);
}

export default function Workspaces() {
  const initial = useLoaderData<typeof clientLoader>();
  const [workspaces, changedIds] = useSSEList<Workspace, WorkspaceRow>(
    "/apis/magosproject.io/v1alpha1/workspaces/events",
    initial,
    toWorkspaceRow,
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
        searchKey="name"
        columns={workspaceColumns}
        toCard={workspaceToCard}
        toHref={workspaceToHref}
        flashIds={changedIds}
        getFlashStyle={workspaceFlashStyle}
      />
    </Stack>
  );
}
