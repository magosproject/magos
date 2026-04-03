import { Stack, Text, Title } from "@mantine/core";
import { IconMoodSad } from "@tabler/icons-react";

interface Props {
  message?: string;
}

export default function NotFound({
  message = "The page you're looking for doesn't exist.",
}: Props) {
  return (
    <Stack align="center" gap="xs" py="xl">
      <IconMoodSad size={40} stroke={1.5} color="gray" />
      <Title order={4} c="dimmed">
        Not found
      </Title>
      <Text size="sm" c="dimmed">
        {message}
      </Text>
    </Stack>
  );
}
