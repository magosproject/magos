import { ActionIcon, Button, Group, Stack, Table, Text, Title, Tooltip } from "@mantine/core";
import { IconX } from "@tabler/icons-react";
import { type CSSProperties, type ReactNode } from "react";

interface Column {
  key: string;
  label: string;
}

interface Row {
  id: string;
  cells: ReactNode[];
  onClick?: () => void;
  onRemove?: () => void;
  className?: string;
  style?: CSSProperties;
}

interface Props {
  title: string;
  columns: Column[];
  rows: Row[];
  emptyMessage?: string;
  action?: { label: string; onClick: () => void };
}

export default function SectionTable({
  title,
  columns,
  rows,
  emptyMessage = "Nothing here yet.",
  action,
}: Props) {
  const hasRemove = rows.some((r) => r.onRemove);

  return (
    <Stack gap="xs">
      <Group justify="space-between" align="center">
        {title ? <Title order={4}>{title}</Title> : <span />}
        {action && (
          <Button size="xs" variant="light" color="magos" onClick={action.onClick}>
            {action.label}
          </Button>
        )}
      </Group>
      {rows.length === 0 ? (
        <Text size="sm" c="dimmed">
          {emptyMessage}
        </Text>
      ) : (
        <Table highlightOnHover withTableBorder withColumnBorders={false}>
          <Table.Thead>
            <Table.Tr>
              {columns.map((col) => (
                <Table.Th key={col.key}>
                  <Text size="sm" fw={600}>
                    {col.label}
                  </Text>
                </Table.Th>
              ))}
              {hasRemove && <Table.Th style={{ width: 36 }} />}
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {rows.map((row) => (
              <Table.Tr
                key={row.id}
                onClick={row.onClick}
                className={row.className}
                style={row.onClick ? { cursor: "pointer", ...row.style } : row.style}
              >
                {row.cells.map((cell, i) => (
                  <Table.Td key={i}>{cell}</Table.Td>
                ))}
                {hasRemove && (
                  <Table.Td>
                    {row.onRemove && (
                      <Tooltip label="Remove">
                        <ActionIcon
                          size="sm"
                          variant="subtle"
                          color="red"
                          onClick={(e) => {
                            e.stopPropagation();
                            row.onRemove!();
                          }}
                        >
                          <IconX size={14} />
                        </ActionIcon>
                      </Tooltip>
                    )}
                  </Table.Td>
                )}
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}
