import { Badge, Stack, Text, Title } from "@mantine/core";
import { useNavigate, useParams } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import SectionTable from "../components/SectionTable";
import UserAvatar from "../components/UserAvatar";
import NotFound from "../components/NotFound";
import KubeBadge from "../components/KubeBadge";
import { groups, rbacUsers } from "../mock-data/rbac";
import { projects } from "../mock-data/projects";
import { workspaces } from "../mock-data/workspaces";

export function meta({ params }: { params: { id: string } }) {
  const group = groups.find((g) => g.id === params.id);
  return [{ title: `${group?.name ?? params.id} – magos` }];
}

const roleColor: Record<string, string> = {
  admin: "magos",
  user: "gray",
};

export default function AdminGroup() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const group = groups.find((g) => g.id === id);

  if (!group) {
    return <NotFound message="Group not found." />;
  }

  const members = rbacUsers.filter((u) => u.groupIds.includes(group.id));
  const groupProjects = projects.filter((p) => group.projectIds.includes(p.id));
  const groupWorkspaces = workspaces.filter((ws) => group.workspaceIds.includes(ws.id));

  return (
    <Stack gap="lg">
      <Breadcrumbs
        crumbs={[
          { label: "Admin" },
          { label: "Groups", to: "/admin/groups" },
          { label: group.name },
        ]}
      />

      <Stack gap={4}>
        <Title order={2}>{group.name}</Title>
        <Text size="sm" c="dimmed">
          {group.description}
        </Text>
      </Stack>

      <SectionTable
        title="Members"
        columns={[
          { key: "user", label: "User" },
          { key: "role", label: "Role" },
        ]}
        rows={members.map((u) => ({
          id: u.id,
          onClick: () => navigate(`/admin/users/${u.id}`),
          cells: [
            <UserAvatar key="avatar" name={u.name} email={u.email} />,
            <Badge key="role" color={roleColor[u.role]} variant="light" size="sm">
              {u.role}
            </Badge>,
          ],
        }))}
        emptyMessage="No members in this group."
      />

      <SectionTable
        title="Project access"
        columns={[
          { key: "name", label: "Name" },
          { key: "description", label: "Description" },
        ]}
        rows={groupProjects.map((p) => ({
          id: p.id,
          onClick: () => navigate(`/projects/${p.id}`),
          cells: [
            <Text key="name" size="sm" fw={500}>
              {p.name}
            </Text>,
            <Text key="desc" size="sm" c="dimmed">
              {p.description}
            </Text>,
          ],
        }))}
        emptyMessage="No project access assigned."
      />

      <SectionTable
        title="Direct workspace access"
        columns={[
          { key: "name", label: "Name" },
          { key: "namespace", label: "Namespace" },
        ]}
        rows={groupWorkspaces.map((ws) => ({
          id: ws.id,
          onClick: () => navigate(`/workspaces/${ws.id}`),
          cells: [
            <Text key="name" size="sm" fw={500}>
              {ws.name}
            </Text>,
            <KubeBadge key="namespace" label={ws.namespace} />,
          ],
        }))}
        emptyMessage="No direct workspace access assigned."
      />
    </Stack>
  );
}
