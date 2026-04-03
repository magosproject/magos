import { Avatar, Group, Stack, Text } from "@mantine/core";

interface Props {
  name: string;
  email: string;
  size?: number;
}

export default function UserAvatar({ name, email, size = 28 }: Props) {
  return (
    <Group gap="xs" wrap="nowrap">
      <Avatar size={size} color="magos" radius="xl" style={{ flexShrink: 0 }}>
        {name[0]}
      </Avatar>
      <Stack gap={0} style={{ minWidth: 0 }}>
        <Text size="sm" fw={500} truncate>
          {name}
        </Text>
        <Text size="xs" c="dimmed" truncate>
          {email}
        </Text>
      </Stack>
    </Group>
  );
}
