import { Avatar, Badge, Group, Stack, Text, Title } from "@mantine/core";
import { useNavigate, useParams } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import SectionTable from "../components/SectionTable";
import NotFound from "../components/NotFound";
import { rbacUsers, groups } from "../mock-data/rbac";

export function meta({ params }: { params: { id: string } }) {
  const user = rbacUsers.find((u) => u.id === params.id);
  return [{ title: `${user?.name ?? params.id} – magos` }];
}

const roleColor: Record<string, string> = {
  admin: "magos",
  user: "gray",
};

export default function AdminUser() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const user = rbacUsers.find((u) => u.id === id);

  if (!user) {
    return <NotFound message="User not found." />;
  }

  const userGroups = groups.filter((g) => user.groupIds.includes(g.id));

  return (
    <Stack gap="lg">
      <Breadcrumbs
        crumbs={[{ label: "Admin" }, { label: "Users", to: "/admin/users" }, { label: user.name }]}
      />

      <Group gap="md" align="center">
        <Avatar size={48} color="magos" radius="xl">
          {user.name[0]}
        </Avatar>
        <Stack gap={2}>
          <Group gap="xs" align="center">
            <Title order={2}>{user.name}</Title>
            <Badge color={roleColor[user.role]} variant="light">
              {user.role}
            </Badge>
          </Group>
          <Text size="sm" c="dimmed">
            {user.email}
          </Text>
        </Stack>
      </Group>

      <SectionTable
        title="Groups"
        columns={[
          { key: "name", label: "Name" },
          { key: "description", label: "Description" },
          { key: "projects", label: "Projects" },
        ]}
        rows={userGroups.map((g) => ({
          id: g.id,
          onClick: () => navigate(`/admin/groups/${g.id}`),
          cells: [
            <Text key="name" size="sm" fw={500}>
              {g.name}
            </Text>,
            <Text key="desc" size="sm" c="dimmed">
              {g.description}
            </Text>,
            g.projectIds.length === 0 ? (
              <Text key="projects" size="sm" c="dimmed">
                —
              </Text>
            ) : (
              <Badge key="projects" variant="light" color="magos" size="sm">
                {g.projectIds.length} projects
              </Badge>
            ),
          ],
        }))}
        emptyMessage="Not a member of any group."
      />
    </Stack>
  );
}
