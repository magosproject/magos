import { Badge, Group, Stack, Text, Title } from "@mantine/core";
import { useNavigate } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import SectionTable from "../components/SectionTable";
import KubeBadge from "../components/KubeBadge";
import { groups, rbacUsers } from "../mock-data/rbac";
import { projects } from "../mock-data/projects";
import { workspaces } from "../mock-data/workspaces";

export function meta() {
  return [{ title: "Groups – magos" }];
}

export default function AdminGroups() {
  const navigate = useNavigate();

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Admin" }, { label: "Groups" }]} />
      <Title order={2}>Groups</Title>

      <SectionTable
        title=""
        columns={[
          { key: "name", label: "Name" },
          { key: "members", label: "Members" },
          { key: "projects", label: "Projects" },
          { key: "workspaces", label: "Direct workspaces" },
        ]}
        rows={groups.map((g) => {
          const memberCount = rbacUsers.filter((u) => u.groupIds.includes(g.id)).length;
          return {
            id: g.id,
            onClick: () => navigate(`/admin/groups/${g.id}`),
            cells: [
              <Stack key="name" gap={0}>
                <Text size="sm" fw={500}>
                  {g.name}
                </Text>
                <Text size="xs" c="dimmed">
                  {g.description}
                </Text>
              </Stack>,
              <Badge key="members" variant="light" color="magos" size="sm">
                {memberCount}
              </Badge>,
              g.projectIds.length === 0 ? (
                <Text key="projects" size="sm" c="dimmed">
                  —
                </Text>
              ) : (
                <Group key="projects" gap={4}>
                  {g.projectIds.map((id) => {
                    const p = projects.find((p) => p.id === id);
                    return p ? (
                      <Badge key={id} variant="light" color="blue" size="xs">
                        {p.name}
                      </Badge>
                    ) : null;
                  })}
                </Group>
              ),
              g.workspaceIds.length === 0 ? (
                <Text key="workspaces" size="sm" c="dimmed">
                  —
                </Text>
              ) : (
                <Group key="workspaces" gap={4}>
                  {g.workspaceIds.map((id) => {
                    const ws = workspaces.find((w) => w.id === id);
                    return ws ? <KubeBadge key={id} label={ws.namespace} /> : null;
                  })}
                </Group>
              ),
            ],
          };
        })}
        emptyMessage="No groups found."
      />
    </Stack>
  );
}
