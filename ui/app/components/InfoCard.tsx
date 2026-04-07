import { Stack, Text } from "@mantine/core";
import { type CSSProperties, type ReactNode } from "react";

interface InfoCardProps {
  label: string;
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
}

export default function InfoCard({ label, children, className, style }: InfoCardProps) {
  return (
    <Stack
      gap={4}
      p="sm"
      className={`info-card${className ? ` ${className}` : ""}`}
      style={{
        border: "1px solid var(--mantine-color-default-border)",
        borderRadius: "var(--mantine-radius-md)",
        background: "var(--mantine-color-default)",
        ...style,
      }}
    >
      <Text size="xs" c="dimmed" tt="uppercase" fw={600}>
        {label}
      </Text>
      {children}
    </Stack>
  );
}
