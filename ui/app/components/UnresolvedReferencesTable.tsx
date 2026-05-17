import { Alert, Code, Stack, Table, Text } from "@mantine/core";
import { IconAlertTriangle } from "@tabler/icons-react";
import type { components } from "../api/types.gen";

type UnresolvedReference = components["schemas"]["v1alpha1.UnresolvedReference"];

interface Props {
  references: UnresolvedReference[];
}

// UnresolvedReferencesTable surfaces the entries reported on
// VariableSet.status.unresolvedReferences. The Alert wrapper is intentional:
// when this table is rendered, the VariableSet is in Failed and any
// downstream Workspace using it is blocked. Operators should not have to
// read the table to know something is wrong.
export default function UnresolvedReferencesTable({ references }: Props) {
  if (references.length === 0) return null;

  return (
    <Alert variant="light" color="red" icon={<IconAlertTriangle size={18} />} title="Unresolved references">
      <Stack gap="xs">
        <Text size="sm">
          One or more references could not be read on the last reconcile.
          Workspaces consuming this VariableSet are parked in
          PhaseFailed.UnresolvedVariables until every required reference
          resolves.
        </Text>
        <Table withTableBorder withColumnBorders={false}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Variable</Table.Th>
              <Table.Th>Kind</Table.Th>
              <Table.Th>Name</Table.Th>
              <Table.Th>Key</Table.Th>
              <Table.Th>Reason</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {references.map((r, i) => (
              <Table.Tr key={(r.variable ?? "") + "-" + i}>
                <Table.Td>
                  <Code>{r.variable ?? "—"}</Code>
                </Table.Td>
                <Table.Td>
                  <Text size="sm">{r.kind ?? "—"}</Text>
                </Table.Td>
                <Table.Td>
                  <Code>{r.name ?? "—"}</Code>
                </Table.Td>
                <Table.Td>
                  <Code>{r.key ?? "—"}</Code>
                </Table.Td>
                <Table.Td>
                  <Text size="sm" c="red">
                    {r.reason ?? "—"}
                  </Text>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Stack>
    </Alert>
  );
}
