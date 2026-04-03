import {
  Anchor,
  Badge,
  Card,
  Group,
  Stack,
  Text,
  Divider,
  Box,
  useMantineTheme,
} from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { useNavigate } from "react-router";
import { type ReactNode } from "react";

export interface ResourceCardBadge {
  label: string;
  color: string;
  spinning?: boolean;
}

export interface ResourceCardMeta {
  icon?: ReactNode;
  label: ReactNode;
  href?: string;
}

export interface ResourceCardProps {
  to: string;
  title: string;
  description?: string;
  badges: ResourceCardBadge[];
  meta: ResourceCardMeta[];
  statusColor?: string;
}

export default function ResourceCard({
  to,
  title,
  description,
  badges,
  meta,
  statusColor,
}: ResourceCardProps) {
  const navigate = useNavigate();
  const theme = useMantineTheme();

  // Resolve status color to actual theme color if possible, fallback to the string
  const resolvedColor =
    statusColor && theme.colors[statusColor]
      ? theme.colors[statusColor][theme.primaryShade as number | 6]
      : statusColor;

  return (
    <Card
      withBorder
      padding="md"
      radius="md"
      className="resource-card"
      style={{
        cursor: "pointer",
        textDecoration: "none",
        borderLeft: resolvedColor ? `4px solid ${resolvedColor}` : undefined,
        transition: "box-shadow 0.2s ease, transform 0.2s ease",
      }}
      onClick={() => navigate(to)}
    >
      <Stack gap="sm">
        <Group justify="space-between" align="flex-start" wrap="nowrap">
          <Box style={{ flex: 1, minWidth: 0 }}>
            <Text fw={600} size="md" truncate>
              {title}
            </Text>
            {description && (
              <Text size="xs" c="dimmed" truncate mt={2}>
                {description}
              </Text>
            )}
          </Box>
          {badges.length > 0 && (
            <Group gap={6} wrap="nowrap">
              {badges.map((b) => (
                <Badge key={b.label} color={b.color} variant="light" size="sm" radius="sm">
                  <Group gap={4} wrap="nowrap" align="center">
                    {b.spinning && (
                      <span className="spin" style={{ display: "inline-block", lineHeight: 0 }}>
                        <IconRefresh size={10} />
                      </span>
                    )}
                    {b.label}
                  </Group>
                </Badge>
              ))}
            </Group>
          )}
        </Group>

        <Divider />

        <Stack gap={6}>
          {meta.map((m, i) =>
            m.href ? (
              <Group key={i} gap={8} wrap="nowrap">
                {m.icon && (
                  <Box
                    style={{
                      display: "flex",
                      alignItems: "center",
                      color: "var(--mantine-color-dimmed)",
                    }}
                  >
                    {m.icon}
                  </Box>
                )}
                <Anchor
                  href={m.href}
                  target="_blank"
                  size="xs"
                  truncate
                  onClick={(e) => e.stopPropagation()}
                >
                  {m.label}
                </Anchor>
              </Group>
            ) : (
              <Group key={i} gap={8} wrap="nowrap">
                {m.icon && (
                  <Box
                    style={{
                      display: "flex",
                      alignItems: "center",
                      color: "var(--mantine-color-dimmed)",
                    }}
                  >
                    {m.icon}
                  </Box>
                )}
                <Text size="xs" c="dimmed" truncate component="div">
                  {m.label}
                </Text>
              </Group>
            )
          )}
        </Stack>
      </Stack>
    </Card>
  );
}
