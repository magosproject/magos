import { Stack, Table, Text, Title } from "@mantine/core";
import type { components } from "../api/types.gen";

type PolicyViolation = components["schemas"]["v1alpha1.PolicyViolation"];

interface Props {
  violations: PolicyViolation[];
}

export default function PolicyViolationsTable({ violations }: Props) {
  if (violations.length === 0) return null;

  return (
    <Stack gap="xs">
      <Title order={4}>Policy Violations</Title>
      <Table withTableBorder withColumnBorders={false}>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Policy</Table.Th>
            <Table.Th>Message</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {violations.map((v, i) => (
            <Table.Tr key={v.policy ?? i}>
              <Table.Td>
                <Text size="sm" fw={500} ff="monospace">
                  {v.policy ?? "—"}
                </Text>
              </Table.Td>
              <Table.Td>
                <Text size="sm" c="dimmed">
                  {v.message ?? "—"}
                </Text>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Stack>
  );
}
