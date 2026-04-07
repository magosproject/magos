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
import { type CSSProperties, Fragment, type ReactNode } from "react";

type StringKeys<T> = { [K in keyof T]: T[K] extends string ? K : never }[keyof T];

export interface ColumnDef<T> {
  key: string;
  label: string;
  sortField?: StringKeys<T>;
  sortValue?: (item: T) => string;
  render: (item: T) => ReactNode;
}

interface ResourceListProps<T extends { id: string }> {
  items: T[];
  searchKey?: StringKeys<T>;
  getSearchText?: (item: T) => string;
  filterKey?: StringKeys<T>;
  filterColors?: Record<string, string>;
  filterLabelMap?: Record<string, string>;
  columns: ColumnDef<T>[];
  renderCard?: (item: T) => ReactNode;
  toHref: (item: T) => string;
  defaultView?: ViewMode;
  hideViewToggle?: boolean;
  flashIds?: Set<string>;
  getFlashStyle?: (item: T) => CSSProperties | undefined;
}

type ViewMode = "card" | "row";

export default function ResourceList<T extends { id: string }>({
  items,
  searchKey,
  getSearchText,
  filterKey,
  filterColors = {},
  filterLabelMap = {},
  columns,
  renderCard,
  toHref,
  defaultView = "card",
  hideViewToggle = false,
  flashIds,
  getFlashStyle,
}: ResourceListProps<T>) {
  const [view, setView] = useState<ViewMode>(defaultView);
  const [search, setSearch] = useState("");
  const [activeFilters, setActiveFilters] = useState<string[]>([]);
  const [sortKey, setSortKey] = useState<string | null>(null);
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");
  const navigate = useNavigate();

  const sortColumn = sortKey ? columns.find((c) => c.key === sortKey) : null;

  const filterValues = filterKey
    ? [...new Set(items.map((item) => item[filterKey] as string))]
    : [];

  const filtered = useMemo(() => {
    const query = search.toLowerCase();
    return items
      .filter((item) => {
        if (!query) return true;
        const text = getSearchText
          ? getSearchText(item)
          : searchKey
            ? (item[searchKey] as string)
            : "";
        return text.toLowerCase().includes(query);
      })
      .filter(
        (item) =>
          !filterKey ||
          activeFilters.length === 0 ||
          activeFilters.includes(item[filterKey] as string)
      )
      .sort((a, b) => {
        if (!sortColumn) return 0;
        const aVal = sortColumn.sortValue
          ? sortColumn.sortValue(a)
          : sortColumn.sortField
            ? (a[sortColumn.sortField] as string)
            : "";
        const bVal = sortColumn.sortValue
          ? sortColumn.sortValue(b)
          : sortColumn.sortField
            ? (b[sortColumn.sortField] as string)
            : "";
        const cmp = aVal.localeCompare(bVal);
        return sortDir === "asc" ? cmp : -cmp;
      });
  }, [items, search, searchKey, getSearchText, filterKey, activeFilters, sortColumn, sortDir]);

  const isSortable = (col: ColumnDef<T>) => !!(col.sortField || col.sortValue);

  const toggleSort = (key: string) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const SortIcon = ({ colKey }: { colKey: string }) => {
    if (sortKey !== colKey) return <IconSelector size={14} />;
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

      {view === "card" && renderCard ? (
        filtered.length === 0 ? (
          <Text c="dimmed" ta="center" py="xl">
            {items.length === 0 ? "Nothing here yet." : "No results match your search."}
          </Text>
        ) : (
          <SimpleGrid cols={{ base: 1, sm: 2, md: 3, lg: 4 }} spacing="md">
            {filtered.map((item) => (
              <Fragment key={item.id}>{renderCard(item)}</Fragment>
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
                  onClick={isSortable(col) ? () => toggleSort(col.key) : undefined}
                  style={isSortable(col) ? { cursor: "pointer", whiteSpace: "nowrap" } : undefined}
                >
                  {isSortable(col) ? (
                    <Group gap={4} wrap="nowrap">
                      <Text size="sm" fw={600}>
                        {col.label}
                      </Text>
                      <SortIcon colKey={col.key} />
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
              filtered.map((item) => {
                const isFlashing = flashIds?.has(item.id);
                const flashStyle = isFlashing && getFlashStyle ? getFlashStyle(item) : undefined;
                return (
                  <Table.Tr
                    key={item.id}
                    onClick={() => navigate(toHref(item))}
                    className={isFlashing ? "flash-highlight" : undefined}
                    style={{ cursor: "pointer", ...flashStyle }}
                  >
                    {columns.map((col) => (
                      <Table.Td key={col.key}>{col.render(item)}</Table.Td>
                    ))}
                  </Table.Tr>
                );
              })
            )}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
}
