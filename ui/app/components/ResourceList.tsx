import { useMemo, useState } from "react";
import {
  ActionIcon,
  Anchor,
  Chip,
  Group,
  MultiSelect,
  Pagination,
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

export interface FilterOption {
  value: string;
  label?: string;
  color?: string;
  count?: number;
}

export interface FilterGroup<T> {
  key: string;
  label?: string;
  options: FilterOption[];
  match: (item: T, selected: string[]) => boolean;
  // "chips"      – colored pill buttons, rendered below the search row
  // "multiselect" – searchable dropdown, rendered inline with the search bar
  variant?: "chips" | "multiselect";
}

type ViewMode = "card" | "row";

interface ResourceListProps<T extends { id: string }> {
  items: T[];
  searchKey?: StringKeys<T>;
  getSearchText?: (item: T) => string;
  filterGroups?: FilterGroup<T>[];
  columns: ColumnDef<T>[];
  renderCard?: (item: T) => ReactNode;
  toHref: (item: T) => string;
  defaultView?: ViewMode;
  hideViewToggle?: boolean;
  flashIds?: Set<string>;
  getFlashStyle?: (item: T) => CSSProperties | undefined;
  pageSize?: number;
  noun?: string;
}

export default function ResourceList<T extends { id: string }>({
  items,
  searchKey,
  getSearchText,
  filterGroups,
  columns,
  renderCard,
  toHref,
  defaultView = "card",
  hideViewToggle = false,
  flashIds,
  getFlashStyle,
  pageSize,
  noun = "item",
}: ResourceListProps<T>) {
  const [view, setView] = useState<ViewMode>(defaultView);
  const [search, setSearch] = useState("");
  const [activeFilters, setActiveFilters] = useState<Record<string, string[]>>({});
  const [sortKey, setSortKey] = useState<string | null>(null);
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");
  const [page, setPage] = useState(1);
  const navigate = useNavigate();

  const sortColumn = sortKey ? columns.find((c) => c.key === sortKey) : null;

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
      .filter((item) => {
        if (!filterGroups) return true;
        return filterGroups.every((group) => {
          const selected = activeFilters[group.key] ?? [];
          if (selected.length === 0) return true;
          return group.match(item, selected);
        });
      })
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
  }, [items, search, searchKey, getSearchText, filterGroups, activeFilters, sortColumn, sortDir]);

  const totalPages = pageSize ? Math.max(1, Math.ceil(filtered.length / pageSize)) : 1;
  const safePage = Math.min(page, totalPages);
  const paged = pageSize ? filtered.slice((safePage - 1) * pageSize, safePage * pageSize) : filtered;

  const hasActiveFilters = Object.values(activeFilters).some((v) => v.length > 0);

  function handleSearch(val: string) {
    setSearch(val);
    setPage(1);
  }

  function handleFilterChange(groupKey: string, values: string[]) {
    setActiveFilters((prev) => ({ ...prev, [groupKey]: values }));
    setPage(1);
  }

  function clearFilters() {
    setActiveFilters({});
    setSearch("");
    setPage(1);
  }

  const isSortable = (col: ColumnDef<T>) => !!(col.sortField || col.sortValue);

  function toggleSort(key: string) {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  }

  function SortIcon({ colKey }: { colKey: string }) {
    if (sortKey !== colKey) return <IconSelector size={14} />;
    return sortDir === "asc" ? <IconChevronUp size={14} /> : <IconChevronDown size={14} />;
  }

  const visibleGroups = filterGroups?.filter((g) => g.options.length > 0) ?? [];
  const multiselectGroups = visibleGroups.filter((g) => g.variant === "multiselect");
  const chipGroups = visibleGroups.filter((g) => g.variant !== "multiselect");

  const countLabel =
    filtered.length === items.length
      ? `${items.length} ${items.length === 1 ? noun : `${noun}s`}`
      : `${filtered.length} of ${items.length} ${noun}s`;

  return (
    <Stack gap="md">
      {/* Search bar + multiselect filters share one visual row */}
      <Group gap="sm" align="flex-start" wrap="nowrap">
        <TextInput
          style={{ flex: 1 }}
          placeholder="Search..."
          leftSection={<IconSearch size={16} />}
          value={search}
          onChange={(e) => handleSearch(e.target.value)}
        />
        {multiselectGroups.map((group) => (
          <MultiSelect
            key={group.key}
            placeholder={group.label ?? "Filter"}
            data={group.options.map((opt) => ({
              value: opt.value,
              label: opt.label ?? opt.value,
            }))}
            value={activeFilters[group.key] ?? []}
            onChange={(v) => handleFilterChange(group.key, v)}
            searchable
            clearable
            style={{ width: 240, flexShrink: 0 }}
            maxDropdownHeight={280}
          />
        ))}
      </Group>

      {/* Chip filters below the search row */}
      {chipGroups.length > 0 && (
        <Stack gap="xs">
          {chipGroups.map((group) => (
            <Group key={group.key} gap="xs" align="center" wrap="wrap">
              {group.label && (
                <Text size="xs" c="dimmed" fw={500} style={{ minWidth: 52 }}>
                  {group.label}
                </Text>
              )}
              <Chip.Group
                multiple
                value={activeFilters[group.key] ?? []}
                onChange={(v) => handleFilterChange(group.key, v)}
              >
                <Group gap={6}>
                  {group.options.map((opt) => (
                    <Chip
                      key={opt.value}
                      value={opt.value}
                      size="xs"
                      color={opt.color ?? "gray"}
                      variant="outline"
                    >
                      {opt.count !== undefined
                        ? `${opt.label ?? opt.value} · ${opt.count}`
                        : (opt.label ?? opt.value)}
                    </Chip>
                  ))}
                </Group>
              </Chip.Group>
            </Group>
          ))}
        </Stack>
      )}

      <Group justify="space-between" wrap="nowrap">
        <Group gap={4} align="center">
          <Text size="sm" c="dimmed">
            {countLabel}
          </Text>
          {(hasActiveFilters || search) && (
            <Anchor size="sm" onClick={clearFilters} style={{ lineHeight: 1 }}>
              · clear
            </Anchor>
          )}
        </Group>
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
        paged.length === 0 ? (
          <Text c="dimmed" ta="center" py="xl">
            {items.length === 0 ? "Nothing here yet." : "No results match your search."}
          </Text>
        ) : (
          <SimpleGrid cols={{ base: 1, sm: 2, md: 3, lg: 4 }} spacing="md">
            {paged.map((item) => (
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
            {paged.length === 0 ? (
              <Table.Tr>
                <Table.Td colSpan={columns.length}>
                  <Text c="dimmed" ta="center" py="md">
                    {items.length === 0 ? "Nothing here yet." : "No results match your search."}
                  </Text>
                </Table.Td>
              </Table.Tr>
            ) : (
              paged.map((item) => {
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

      {pageSize && filtered.length > pageSize && (
        <Group justify="space-between" align="center">
          <Text size="sm" c="dimmed">
            {`${(safePage - 1) * pageSize + 1}–${Math.min(safePage * pageSize, filtered.length)} of ${filtered.length}`}
          </Text>
          <Pagination
            total={totalPages}
            value={safePage}
            onChange={setPage}
            size="sm"
            color="magos"
          />
        </Group>
      )}
    </Stack>
  );
}
