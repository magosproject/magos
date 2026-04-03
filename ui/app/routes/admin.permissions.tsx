import { Badge, Group, Stack, Text, Title } from "@mantine/core";
import { useNavigate } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import SectionTable from "../components/SectionTable";
import { groups } from "../mock-data/rbac";
import { projects } from "../mock-data/projects";
import { workspaces } from "../mock-data/workspaces";

export function meta() {
  return [{ title: "Permissions – magos" }];
}

export default function AdminPermissions() {
  const navigate = useNavigate();

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Admin" }, { label: "Permissions" }]} />
      <Title order={2}>Permissions</Title>

      <SectionTable
        title=""
        columns={[
          { key: "group", label: "Group" },
          { key: "projects", label: "Project access" },
          { key: "workspaces", label: "Direct workspace access" },
        ]}
        rows={groups.map((g) => {
          const groupProjects = projects.filter((p) => g.projectIds.includes(p.id));
          const groupWorkspaces = workspaces.filter((ws) => g.workspaceIds.includes(ws.id));
          return {
            id: g.id,
            onClick: () => navigate(`/admin/groups/${g.id}`),
            cells: [
              <Stack key="group" gap={0}>
                <Text size="sm" fw={500}>
                  {g.name}
                </Text>
                <Text size="xs" c="dimmed">
                  {g.description}
                </Text>
              </Stack>,
              groupProjects.length === 0 ? (
                <Text key="projects" size="sm" c="dimmed">
                  —
                </Text>
              ) : (
                <Group key="projects" gap={4} wrap="wrap">
                  {groupProjects.map((p) => (
                    <Badge key={p.id} variant="light" color="magos" size="sm">
                      {p.name}
                    </Badge>
                  ))}
                </Group>
              ),
              groupWorkspaces.length === 0 ? (
                <Text key="workspaces" size="sm" c="dimmed">
                  —
                </Text>
              ) : (
                <Group key="workspaces" gap={4} wrap="wrap">
                  {groupWorkspaces.map((ws) => (
                    <Badge key={ws.id} variant="light" color="gray" size="sm">
                      {ws.name} / {ws.namespace}
                    </Badge>
                  ))}
                </Group>
              ),
            ],
          };
        })}
        emptyMessage="No permissions configured."
      />
    </Stack>
  );
}
