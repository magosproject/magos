import { Stack, Text } from "@mantine/core";
import { type ReactNode } from "react";

interface InfoCardProps {
  label: string;
  children: ReactNode;
}

export default function InfoCard({ label, children }: InfoCardProps) {
  return (
    <Stack
      gap={4}
      p="sm"
      style={{
        border: "1px solid var(--mantine-color-default-border)",
        borderRadius: "var(--mantine-radius-md)",
      }}
    >
      <Text size="xs" c="dimmed" tt="uppercase" fw={600}>
        {label}
      </Text>
      {children}
    </Stack>
  );
}
