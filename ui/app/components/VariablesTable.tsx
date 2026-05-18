import { Badge, Code, Stack, Table, Text, Title } from "@mantine/core";
import type { components } from "../api/types.gen";

type Variable = components["schemas"]["github_com_magosproject_magos_types_magosproject_v1alpha1.Variable"];
type VariableSource = components["schemas"]["v1alpha1.VariableSource"];

interface Props {
  variables: Variable[];
}

// VariablesTable renders the contents of a VariableSet.Spec.Variables in a
// way that keeps inline values and reference-backed values visually
// distinct. Inline values render as monospace Code so they read like the
// terraform input they map to. Reference-backed values render only the Kind
// and the referenced resource name: we never resolve or display the
// underlying secret bytes here, and we deliberately leave key paths and
// selector flags like `optional` out of the operator-facing summary because
// they belong in the YAML, not in the dashboard.
export default function VariablesTable({ variables }: Props) {
  if (variables.length === 0) return null;

  return (
    <Stack gap="xs">
      <Title order={4}>Variables</Title>
      <Text size="sm" c="dimmed">
        Inline values are stored verbatim on the CR. Secret and ConfigMap
        references are resolved at pod startup, so their values never appear
        in the UI.
      </Text>
      <Table withTableBorder withColumnBorders={false}>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Name</Table.Th>
            <Table.Th>Source</Table.Th>
            <Table.Th>Value or reference</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {variables.map((v, i) => (
            <Table.Tr key={v.name ?? i}>
              <Table.Td>
                <Code>{v.name ?? ""}</Code>
              </Table.Td>
              <Table.Td>
                <SourceBadge variable={v} />
              </Table.Td>
              <Table.Td>
                <ValueCell variable={v} />
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Stack>
  );
}

// SourceBadge condenses the question "where does this variable come from"
// into a small coloured tag. Keeping it as its own component means the
// caller does not have to know about valueFrom internals to render the
// table.
function SourceBadge({ variable }: { variable: Variable }) {
  if (variable.value !== undefined) {
    return <Badge variant="light" color="gray" size="sm">Inline</Badge>;
  }
  if (variable.valueFrom?.secretKeyRef) {
    return <Badge variant="light" color="grape" size="sm">Secret</Badge>;
  }
  if (variable.valueFrom?.configMapKeyRef) {
    return <Badge variant="light" color="blue" size="sm">ConfigMap</Badge>;
  }
  return <Badge variant="light" color="red" size="sm">Invalid</Badge>;
}

// ValueCell renders the right-hand column. For inline values we show the
// value verbatim in a Code element so it is immediately distinguishable
// from a reference. For references we show only the referenced resource
// name; the Source column already tells the reader which kind of resource
// we are pointing at.
function ValueCell({ variable }: { variable: Variable }) {
  if (variable.value !== undefined) {
    return <Code>{variable.value}</Code>;
  }

  const name = referenceNameFor(variable.valueFrom);
  if (!name) {
    return (
      <Text size="sm" c="red">
        no value or valueFrom set
      </Text>
    );
  }

  return <Code>{name}</Code>;
}

function referenceNameFor(src: VariableSource | undefined): string | null {
  if (!src) return null;
  if (src.secretKeyRef) return src.secretKeyRef.name ?? "";
  if (src.configMapKeyRef) return src.configMapKeyRef.name ?? "";
  return null;
}
