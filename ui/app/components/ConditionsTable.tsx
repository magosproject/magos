import { Badge, Stack, Table, Text, Title } from "@mantine/core";

interface Condition {
  type?: string;
  status?: string;
  reason?: string;
  message?: string;
  [key: string]: unknown;
}

interface Props {
  conditions: Condition[];
}

function conditionStatusColor(status?: string): string {
  if (status === "True") return "green";
  if (status === "False") return "red";
  return "gray";
}

export default function ConditionsTable({ conditions }: Props) {
  if (conditions.length === 0) return null;

  return (
    <Stack gap="xs">
      <Title order={4}>Conditions</Title>
      <Table withTableBorder withColumnBorders={false}>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Type</Table.Th>
            <Table.Th>Status</Table.Th>
            <Table.Th>Reason</Table.Th>
            <Table.Th>Message</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {conditions.map((c, i) => (
            <Table.Tr key={c.type ?? i}>
              <Table.Td>
                <Text size="sm" fw={500}>
                  {c.type}
                </Text>
              </Table.Td>
              <Table.Td>
                <Badge variant="light" color={conditionStatusColor(c.status)} size="sm">
                  {c.status}
                </Badge>
              </Table.Td>
              <Table.Td>
                <Text size="sm" c="dimmed">
                  {c.reason}
                </Text>
              </Table.Td>
              <Table.Td>
                <Text size="sm" c="dimmed">
                  {c.message}
                </Text>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Stack>
  );
}




