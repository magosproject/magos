import { useMemo, useState } from "react";
import {
  ActionIcon,
  Chip,
  Group,
  SimpleGrid,
  Stack,
  Table,
  Text,
  TextInput,
  Tooltip,
} from "@mantine/core";
import {
  IconChevronDown,
  IconChevronUp,
  IconLayoutGrid,
  IconList,
  IconSearch,
  IconSelector,
} from "@tabler/icons-react";
import { useNavigate } from "react-router";
import { type ReactNode } from "react";
import ResourceCard, { type ResourceCardProps } from "./ResourceCard";

type StringKeys<T> = { [K in keyof T]: T[K] extends string ? K : never }[keyof T];

export interface ColumnDef<T> {
  key: string;
  label: string;
  sortField?: StringKeys<T>;
  render: (item: T) => ReactNode;
}

interface ResourceListProps<T extends { id: string }> {
  items: T[];
  searchKey: StringKeys<T>;
  filterKey?: StringKeys<T>;
  filterColors?: Record<string, string>;
  filterLabelMap?: Record<string, string>;
  columns: ColumnDef<T>[];
  toCard?: (item: T) => ResourceCardProps;
  toHref: (item: T) => string;
  defaultView?: ViewMode;
  hideViewToggle?: boolean;
}

type ViewMode = "card" | "row";

export default function ResourceList<T extends { id: string }>({
  items,
  searchKey,
  filterKey,
  filterColors = {},
  filterLabelMap = {},
  columns,
  toCard,
  toHref,
  defaultView = "card",
  hideViewToggle = false,
}: ResourceListProps<T>) {
  const [view, setView] = useState<ViewMode>(defaultView);
  const [search, setSearch] = useState("");
  const [activeFilters, setActiveFilters] = useState<string[]>([]);
  const [sortField, setSortField] = useState<StringKeys<T> | null>(null);
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");
  const navigate = useNavigate();

  const filterValues = filterKey
    ? [...new Set(items.map((item) => item[filterKey] as string))]
    : [];

  const filtered = useMemo(() => {
    const query = search.toLowerCase();
    return items
      .filter((item) => (item[searchKey] as string).toLowerCase().includes(query))
      .filter(
        (item) =>
          !filterKey ||
          activeFilters.length === 0 ||
          activeFilters.includes(item[filterKey] as string)
      )
      .sort((a, b) => {
        if (!sortField) return 0;
        const cmp = (a[sortField] as string).localeCompare(b[sortField] as string);
        return sortDir === "asc" ? cmp : -cmp;
      });
  }, [items, search, searchKey, filterKey, activeFilters, sortField, sortDir]);

  const toggleSort = (field: StringKeys<T>) => {
    if (sortField === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDir("asc");
    }
  };

  const SortIcon = ({ field }: { field: StringKeys<T> }) => {
    if (sortField !== field) return <IconSelector size={14} />;
    return sortDir === "asc" ? <IconChevronUp size={14} /> : <IconChevronDown size={14} />;
  };

  return (
    <Stack gap="md">
      <TextInput
        placeholder="Search..."
        leftSection={<IconSearch size={16} />}
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />

      <Group justify="space-between" wrap="nowrap">
        {filterKey && filterValues.length > 0 ? (
          <Chip.Group multiple value={activeFilters} onChange={setActiveFilters}>
            <Group gap="xs">
              {filterValues.map((val) => (
                <Chip key={val} value={val} size="sm" color={filterColors[val] ?? "gray"}>
                  {filterLabelMap[val] ?? val}
                </Chip>
              ))}
            </Group>
          </Chip.Group>
        ) : (
          <span />
        )}
        {!hideViewToggle && (
          <Group gap="xs" wrap="nowrap">
            <Tooltip label="Card view">
              <ActionIcon
                variant={view === "card" ? "filled" : "subtle"}
                color="magos"
                onClick={() => setView("card")}
              >
                <IconLayoutGrid size={18} />
              </ActionIcon>
            </Tooltip>
            <Tooltip label="Row view">
              <ActionIcon
                variant={view === "row" ? "filled" : "subtle"}
                color="magos"
                onClick={() => setView("row")}
              >
                <IconList size={18} />
              </ActionIcon>
            </Tooltip>
          </Group>
        )}
      </Group>

      {view === "card" && toCard ? (
        filtered.length === 0 ? (
          <Text c="dimmed" ta="center" py="xl">
            {items.length === 0 ? "Nothing here yet." : "No results match your search."}
          </Text>
        ) : (
          <SimpleGrid cols={{ base: 1, sm: 2, md: 3, lg: 4 }} spacing="md">
            {filtered.map((item) => (
              <ResourceCard key={item.id} {...toCard(item)} />
            ))}
          </SimpleGrid>
        )
      ) : (
        <Table highlightOnHover withTableBorder withColumnBorders={false}>
          <Table.Thead>
            <Table.Tr>
              {columns.map((col) => (
                <Table.Th
                  key={col.key}
                  onClick={col.sortField ? () => toggleSort(col.sortField!) : undefined}
                  style={col.sortField ? { cursor: "pointer", whiteSpace: "nowrap" } : undefined}
                >
                  {col.sortField ? (
                    <Group gap={4} wrap="nowrap">
                      <Text size="sm" fw={600}>
                        {col.label}
                      </Text>
                      <SortIcon field={col.sortField} />
                    </Group>
                  ) : (
                    <Text size="sm" fw={600}>
                      {col.label}
                    </Text>
                  )}
                </Table.Th>
              ))}
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {filtered.length === 0 ? (
              <Table.Tr>
                <Table.Td colSpan={columns.length}>
                  <Text c="dimmed" ta="center" py="md">
                    {items.length === 0 ? "Nothing here yet." : "No results match your search."}
                  </Text>
                </Table.Td>
              </Table.Tr>
            ) : (
              filtered.map((item) => (
                <Table.Tr
                  key={item.id}
                  onClick={() => navigate(toHref(item))}
                  style={{ cursor: "pointer" }}
                >
                  {columns.map((col) => (
                    <Table.Td key={col.key}>{col.render(item)}</Table.Td>
                  ))}
                </Table.Tr>
              ))
            )}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}
