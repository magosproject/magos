import {
  ActionIcon,
  Badge,
  Button,
  Code,
  Drawer,
  Group,
  Loader,
  ScrollArea,
  Stack,
  Text,
  Tooltip,
} from "@mantine/core";
import { IconChevronDown, IconChevronUp } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import { apiUrl } from "../api/base";
import type { RunLogListResponse, RunLogSummary } from "../api/types";
import { formatDateTime } from "../utils/formatDateTime";
import SectionTable from "./SectionTable";

const pageSize = 5;

function formatDuration(startedAt?: string, finishedAt?: string) {
  if (!startedAt || !finishedAt) return "—";
  const ms = new Date(finishedAt).getTime() - new Date(startedAt).getTime();
  if (!Number.isFinite(ms) || ms < 0) return "—";
  const seconds = Math.round(ms / 1000);
  return `${seconds}s`;
}

function formatBytes(bytes?: number) {
  if (!bytes || bytes <= 0) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

function displayRevision(item: RunLogSummary) {
  return item.targetRevision?.trim() || item.observedRevision?.trim() || "—";
}

interface Props {
  namespace: string;
  workspaceName: string;
  initialLogs: RunLogListResponse;
}

interface PageState {
  items: RunLogSummary[];
  nextCursor: string;
  cursor: string;
}

export default function WorkspaceApplyLogs({ namespace, workspaceName, initialLogs }: Props) {
  const [pages, setPages] = useState<PageState[]>([
    {
      items: initialLogs.items ?? [],
      nextCursor: initialLogs.nextCursor ?? "",
      cursor: "",
    },
  ]);
  const [activePage, setActivePage] = useState(1);
  const [selected, setSelected] = useState<RunLogSummary | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [loadingPage, setLoadingPage] = useState(false);

  const currentPage = pages[activePage - 1] ?? { items: [], nextCursor: "", cursor: "" };
  const hasNewer = activePage > 1;
  const hasOlder = activePage < pages.length || Boolean(currentPage.nextCursor);

  useEffect(() => {
    if (!selected?.runID) {
      setContent("");
      setError("");
      return;
    }

    const controller = new AbortController();
    setLoading(true);
    setError("");

    fetch(
      apiUrl(
        `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs/${selected.runID}/log?phase=apply`
      ),
      { signal: controller.signal }
    )
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`Request failed with status ${response.status}`);
        }
        return response.text();
      })
      .then((text) => {
        setContent(text);
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        setError(err instanceof Error ? err.message : "Failed to load log");
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      });

    return () => controller.abort();
  }, [namespace, selected?.runID, workspaceName]);

  async function showOlderPage() {
    if (loadingPage) return;
    if (activePage < pages.length) {
      setActivePage((page) => page + 1);
      return;
    }

    const cursor = currentPage.nextCursor;
    if (!cursor) return;

    setLoadingPage(true);
    try {
      const response = await fetch(
        apiUrl(
          `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs?phase=apply&limit=${pageSize}&cursor=${encodeURIComponent(cursor)}`
        )
      );
      if (!response.ok) {
        throw new Error(`Request failed with status ${response.status}`);
      }
      const page = (await response.json()) as RunLogListResponse;
      setPages((current) => [
        ...current,
        {
          items: page.items ?? [],
          nextCursor: page.nextCursor ?? "",
          cursor,
        },
      ]);
      setActivePage((current) => current + 1);
    } catch (err) {
      console.error(err);
    } finally {
      setLoadingPage(false);
    }
  }

  function showNewerPage() {
    if (!hasNewer || loadingPage) return;
    setActivePage((page) => page - 1);
  }

  function refreshToLatest() {
    setPages([
      {
        items: initialLogs.items ?? [],
        nextCursor: initialLogs.nextCursor ?? "",
        cursor: "",
      },
    ]);
    setActivePage(1);
  }

  return (
    <>
      <SectionTable
        title="Apply Logs"
        columns={[
          { key: "time", label: "Time" },
          { key: "revision", label: "Revision" },
          { key: "result", label: "Result" },
          { key: "duration", label: "Duration" },
          { key: "size", label: "Size" },
        ]}
        rows={currentPage.items.map((item) => {
          return {
            id: item.runID ?? "",
            onClick: () => setSelected(item),
            cells: [
              <Text size="sm" key="time">
                {formatDateTime(item.finishedAt ?? item.startedAt)}
              </Text>,
              <Code key="revision" fz="xs">
                {displayRevision(item)}
              </Code>,
              <Badge
                key="result"
                size="sm"
                color={item.result === "Succeeded" ? "green" : "red"}
                variant="light"
                tt="none"
              >
                {item.result ?? "Unknown"}
              </Badge>,
              <Text size="sm" key="duration">
                {formatDuration(item.startedAt, item.finishedAt)}
              </Text>,
              <Text size="sm" key="size">
                {formatBytes(item.logSizeBytes)}
              </Text>,
            ],
          };
        })}
        emptyMessage="No archived apply logs yet."
        action={{ label: "Refresh", onClick: refreshToLatest }}
      />
      <Group justify="flex-end">
        <Button
          size="xs"
          variant="default"
          onClick={showNewerPage}
          disabled={!hasNewer || loadingPage}
        >
          Newer
        </Button>
        <Button
          size="xs"
          variant="default"
          onClick={showOlderPage}
          disabled={!hasOlder || loadingPage}
        >
          Older
        </Button>
      </Group>

      <Drawer
        opened={selected !== null}
        onClose={() => {
          setSelected(null);
          setExpanded(false);
        }}
        title={
          <Group justify="space-between" w="100%" pr="md">
            <Group gap="xs">
              <Text fw={600} size="sm">
                Apply Log
              </Text>
              {selected?.result && (
                <Badge
                  color={selected.result === "Succeeded" ? "green" : "red"}
                  variant="light"
                  size="sm"
                  tt="none"
                >
                  {selected.result}
                </Badge>
              )}
              {selected?.runID && <Code fz="xs">{selected.runID}</Code>}
            </Group>
            <Tooltip label={expanded ? "Collapse" : "Expand"}>
              <ActionIcon
                variant="subtle"
                color="gray"
                size="sm"
                onClick={() => setExpanded((value) => !value)}
              >
                {expanded ? <IconChevronDown size={16} /> : <IconChevronUp size={16} />}
              </ActionIcon>
            </Tooltip>
          </Group>
        }
        position="bottom"
        size={expanded ? "100%" : "55%"}
      >
        <Stack gap="sm">
          {loading && (
            <Group justify="center" py="md">
              <Loader size="sm" />
            </Group>
          )}
          {!loading && error && (
            <Text size="sm" c="red">
              {error}
            </Text>
          )}
          {!loading && !error && (
            <ScrollArea h={expanded ? "calc(100vh - 110px)" : 420}>
              <Code block>{content || "Log is empty."}</Code>
            </ScrollArea>
          )}
        </Stack>
      </Drawer>
    </>
  );
}
