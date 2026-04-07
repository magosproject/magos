import {
  Anchor,
  Badge,
  Button,
  Group,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import { IconFolder, IconRefresh } from "@tabler/icons-react";
import { useLoaderData, useParams } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import InfoCard from "../components/InfoCard";
import StatusBadge from "../components/StatusBadge";
import KubeBadge from "../components/KubeBadge";
import { repoIcon } from "../utils/repoIcon";
import apiClient from "../api/client";
import type { Workspace as WorkspaceType } from "../api/types";
import { useSSEItem } from "../hooks/useSSEItem";

export function meta({ params }: { params: { namespace: string; name: string } }) {
  return [{ title: `${params.name} – magos` }];
}

export async function clientLoader({
  params,
}: {
  params: { namespace: string; name: string };
}) {
  const { data } = await apiClient.GET(
    "/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}",
    { params: { path: { namespace: params.namespace, name: params.name } } }
  );
  if (!data) throw new Response("Not found", { status: 404 });
  return data;
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

export default function Workspace() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const initial = useLoaderData<typeof clientLoader>();
  const ws = useSSEItem<WorkspaceType>(
    "/apis/magosproject.io/v1alpha1/workspaces/events",
    initial,
    (obj) => obj.metadata?.namespace === namespace && obj.metadata?.name === name
  );

  const repoURL = ws.spec?.source?.repoURL ?? "";

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
        <InfoCard label="Status">
          <StatusBadge status={ws.status?.phase ?? ""} size="md" />
        </InfoCard>

        <InfoCard label="Project">
          {ws.spec?.projectRef?.name ? (
            <Text size="sm" c="dimmed">
              {ws.spec.projectRef.name}
            </Text>
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
          <Text size="sm" c="dimmed" truncate>
            {ws.spec?.source?.targetRevision ?? "—"}
          </Text>
        </InfoCard>

        <InfoCard label="Terraform Version">
          <Text size="sm" c="dimmed">
            {ws.spec?.terraform?.version ?? "—"}
          </Text>
        </InfoCard>

        <InfoCard label="Auto Apply">
          <Badge color={ws.spec?.autoApply ? "magos" : "gray"} variant="light" size="sm">
            {ws.spec?.autoApply ? "enabled" : "disabled"}
          </Badge>
        </InfoCard>

        {ws.status?.lastReconcileTime && (
          <InfoCard label="Last reconcile">
            <Text size="sm" c="dimmed">
              {formatDate(ws.status.lastReconcileTime)}
            </Text>
          </InfoCard>
        )}

        {ws.status?.observedRevision && (
          <InfoCard label="Observed revision">
            <Text size="sm" c="dimmed" truncate>
              {ws.status.observedRevision}
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
        <Stack gap="xs">
          <Title order={4}>Conditions</Title>
          <Table withTableBorder withColumnBorders={false}>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Type</Table.Th>
                <Table.Th>Status</Table.Th>
                <Table.Th>Reason</Table.Th>
                <Table.Th>Message</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {ws.status.conditions.map((c) => (
                <Table.Tr key={c.type}>
                  <Table.Td>
                    <Text size="sm" fw={500}>
                      {c.type}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Badge
                      variant="light"
                      color={c.status === "True" ? "green" : c.status === "False" ? "red" : "gray"}
                      size="sm"
                    >
                      {c.status}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm" c="dimmed">
                      {c.reason}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm" c="dimmed">
                      {c.message}
                    </Text>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Stack>
      )}
    </Stack>
  );
}
