import { Badge, Group, Stack, Text, Title } from "@mantine/core";
import { useNavigate } from "react-router";
import Breadcrumbs from "../components/Breadcrumbs";
import SectionTable from "../components/SectionTable";
import UserAvatar from "../components/UserAvatar";
import { rbacUsers, groups } from "../mock-data/rbac";

export function meta() {
  return [{ title: "Users – magos" }];
}

const roleColor: Record<string, string> = {
  admin: "magos",
  user: "gray",
};

export default function AdminUsers() {
  const navigate = useNavigate();

  return (
    <Stack gap="md">
      <Breadcrumbs crumbs={[{ label: "Admin" }, { label: "Users" }]} />
      <Title order={2}>Users</Title>

      <SectionTable
        title=""
        columns={[
          { key: "user", label: "User" },
          { key: "role", label: "Role" },
          { key: "groups", label: "Groups" },
        ]}
        rows={rbacUsers.map((u) => ({
          id: u.id,
          onClick: () => navigate(`/admin/users/${u.id}`),
          cells: [
            <UserAvatar key="avatar" name={u.name} email={u.email} />,
            <Badge key="role" color={roleColor[u.role]} variant="light" size="sm">
              {u.role}
            </Badge>,
            u.groupIds.length === 0 ? (
              <Text key="groups" size="sm" c="dimmed">
                —
              </Text>
            ) : (
              <Group key="groups" gap={6}>
                {u.groupIds.map((id) => {
                  const g = groups.find((g) => g.id === id);
                  return g ? (
                    <Badge key={id} variant="light" color="magos" size="sm">
                      {g.name}
                    </Badge>
                  ) : null;
                })}
              </Group>
            ),
          ],
        }))}
        emptyMessage="No users found."
      />
    </Stack>
  );
}
