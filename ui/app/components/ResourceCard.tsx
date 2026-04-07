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
import { Link, useNavigate } from "react-router";
import { type CSSProperties, type ReactNode } from "react";

export interface ResourceCardBadge {
  label: string;
  color: string;
  spinning?: boolean;
}

export interface ResourceCardMeta {
  icon?: ReactNode;
  label: ReactNode;
  href?: string;
  to?: string;
}

export interface ResourceCardProps {
  to: string;
  title: string;
  description?: string;
  badges: ResourceCardBadge[];
  meta: ResourceCardMeta[];
  statusColor?: string;
  borderAll?: boolean;
  flashStyle?: CSSProperties;
}

function MetaRow({ icon, label, href, to }: ResourceCardMeta) {
  const iconBox = icon && (
    <Box style={{ display: "flex", alignItems: "center", color: "var(--mantine-color-dimmed)" }}>
      {icon}
    </Box>
  );

  let content: ReactNode;
  if (href) {
    content = (
      <Anchor href={href} target="_blank" size="xs" truncate onClick={(e) => e.stopPropagation()}>
        {label}
      </Anchor>
    );
  } else if (to) {
    content = (
      <Anchor component={Link} to={to} size="xs" truncate onClick={(e) => e.stopPropagation()}>
        {label}
      </Anchor>
    );
  } else {
    content = (
      <Text size="xs" c="dimmed" truncate component="div">
        {label}
      </Text>
    );
  }

  return (
    <Group gap={8} wrap="nowrap">
      {iconBox}
      {content}
    </Group>
  );
}

export default function ResourceCard({
  to,
  title,
  description,
  badges,
  meta,
  statusColor,
  borderAll,
  flashStyle,
}: ResourceCardProps) {
  const navigate = useNavigate();
  const theme = useMantineTheme();

  const resolvedColor =
    statusColor && theme.colors[statusColor]
      ? theme.colors[statusColor][theme.primaryShade as number | 6]
      : statusColor;

  const borderStyle = resolvedColor
    ? borderAll
      ? { border: `2px solid ${resolvedColor}` }
      : { borderLeft: `4px solid ${resolvedColor}` }
    : undefined;

  return (
    <Card
      withBorder
      padding="md"
      radius="md"
      className={`resource-card${flashStyle ? " flash-highlight" : ""}`}
      style={{
        cursor: "pointer",
        textDecoration: "none",
        ...borderStyle,
        ...flashStyle,
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

        {meta.length > 0 && (
          <>
            <Divider />
            <Stack gap={6}>
              {meta.map((m, i) => (
                <MetaRow key={i} {...m} />
              ))}
            </Stack>
          </>
        )}
      </Stack>
    </Card>
  );
}
