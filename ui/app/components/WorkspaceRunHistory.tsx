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
  Tabs,
  Text,
  Tooltip,
} from "@mantine/core";
import { IconChevronDown, IconChevronUp } from "@tabler/icons-react";
import { type CSSProperties, useCallback, useEffect, useRef, useState } from "react";
import { apiUrl } from "../api/base";
import type { Phase, Run, RunListResponse, RunPhaseSummary } from "../api/types";
import { formatDateTime } from "../utils/formatDateTime";
import { flashColorVar } from "../utils/colors";
import SectionTable from "./SectionTable";

const pageSize = 20;

function formatDuration(startedAt?: string, finishedAt?: string) {
  if (!startedAt || !finishedAt) return "—";
  const ms = new Date(finishedAt).getTime() - new Date(startedAt).getTime();
  if (!Number.isFinite(ms) || ms < 0) return "—";
  return `${Math.round(ms / 1000)}s`;
}

function displayRevision(run: Run) {
  return run.targetRevision?.trim() || run.observedRevision?.trim() || "—";
}

function PhaseBadge({ summary }: { summary?: RunPhaseSummary }) {
  if (!summary) return <Text size="sm">—</Text>;
  const color = summary.result === "Succeeded" ? "green" : "red";
  return (
    <Badge size="sm" color={color} variant="light" tt="none">
      {summary.result ?? "Unknown"}
    </Badge>
  );
}

const triggerLabels: Record<string, string> = {
  configuration: "config change",
};

function TriggerBadge({ trigger }: { trigger?: string }) {
  if (!trigger || trigger === "unknown") return <Text size="sm" c="dimmed">—</Text>;
  const label = triggerLabels[trigger] ?? trigger;
  return (
    <Badge size="sm" variant="outline" tt="none" color="gray">
      {label}
    </Badge>
  );
}

interface LogPaneProps {
  namespace: string;
  workspaceName: string;
  runID: string;
  phase: "plan" | "apply";
  summary?: RunPhaseSummary;
}

// LogPane fetches and renders the archived log for one phase. It is always
// rendered with a key derived from the run and phase so React remounts it when
// the selection changes, which avoids the need to reset state inside an effect.
function LogPane({ namespace, workspaceName, runID, phase, summary }: LogPaneProps) {
  const hasLog = Boolean(summary);
  const [content, setContent] = useState<string | null>(null);
  const [loading, setLoading] = useState(hasLog);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!hasLog) return;

    const controller = new AbortController();

    fetch(
      apiUrl(
        `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs/${runID}/log?phase=${phase}`
      ),
      { signal: controller.signal, cache: "no-store" }
    )
      .then(async (res) => {
        if (!res.ok) throw new Error(`Request failed with status ${res.status}`);
        return res.text();
      })
      .then((text) => {
        setContent(text);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        setError(err instanceof Error ? err.message : "Failed to load log");
        setLoading(false);
      });

    return () => controller.abort();
  }, [namespace, workspaceName, runID, phase, hasLog]);

  if (!summary) {
    return (
      <Text size="sm" c="dimmed" py="md">
        {phase === "apply" ? "Apply phase did not run for this cycle." : "No plan log available."}
      </Text>
    );
  }

  return (
    <Stack gap="xs" style={{ flex: 1, minHeight: 0 }}>
      <Group gap="xs">
        <PhaseBadge summary={summary} />
        {summary.startedAt && (
          <Text size="xs" c="dimmed">
            {formatDateTime(summary.startedAt)} · {formatDuration(summary.startedAt, summary.finishedAt)}
          </Text>
        )}
      </Group>

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
      {!loading && !error && content !== null && (
        <ScrollArea style={{ flex: 1, minHeight: 0 }}>
          <Code block>
            {content || "Log is empty."}
          </Code>
        </ScrollArea>
      )}
    </Stack>
  );
}

interface Props {
  namespace: string;
  workspaceName: string;
  initialRuns?: RunListResponse;
  phase?: Phase;
  currentRunID?: string;
}

interface PageState {
  items: Run[];
  nextCursor: string;
  cursor: string;
}

export default function WorkspaceRunHistory({
  namespace,
  workspaceName,
  initialRuns,
  phase,
  currentRunID,
}: Props) {
  const [pages, setPages] = useState<PageState[]>([
    {
      items: initialRuns?.items ?? [],
      nextCursor: initialRuns?.nextCursor ?? "",
      cursor: "",
    },
  ]);
  const [activePage, setActivePage] = useState(1);
  const [selected, setSelected] = useState<Run | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [loadingPage, setLoadingPage] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [flashingIDs, setFlashingIDs] = useState<Set<string>>(new Set());
  const previousPhaseRef = useRef<Phase | undefined>(phase);
  const previousRunIDRef = useRef<string | undefined>(currentRunID);
  const flashTimeoutRef = useRef<number | null>(null);

  const currentPage = pages[activePage - 1] ?? { items: [], nextCursor: "", cursor: "" };
  const hasNewer = activePage > 1;
  const hasOlder = activePage < pages.length || Boolean(currentPage.nextCursor);

  async function showOlderPage() {
    if (loadingPage) return;
    if (activePage < pages.length) {
      setActivePage((p) => p + 1);
      return;
    }

    const cursor = currentPage.nextCursor;
    if (!cursor) return;

    setLoadingPage(true);
    try {
      const res = await fetch(
        apiUrl(
          `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs?limit=${pageSize}&cursor=${encodeURIComponent(cursor)}`
        ),
        { cache: "no-store" }
      );
      if (!res.ok) throw new Error(`Request failed with status ${res.status}`);
      const page = (await res.json()) as RunListResponse;
      setPages((current) => [
        ...current,
        { items: page.items ?? [], nextCursor: page.nextCursor ?? "", cursor },
      ]);
      setActivePage((p) => p + 1);
    } catch (err) {
      console.error(err);
    } finally {
      setLoadingPage(false);
    }
  }

  function showNewerPage() {
    if (!hasNewer || loadingPage) return;
    setActivePage((p) => p - 1);
  }

  const refreshToLatest = useCallback(async () => {
    setRefreshing(true);
    try {
      const res = await fetch(
        apiUrl(
          `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs?limit=${pageSize}`
        ),
        { cache: "no-store" }
      );
      if (!res.ok) throw new Error(`Request failed with status ${res.status}`);

      const runs = (await res.json()) as RunListResponse;
      const previousIDs = new Set(pages[0]?.items.map((r) => r.runID ?? "").filter(Boolean));
      const newIDs = (runs.items ?? [])
        .map((r) => r.runID ?? "")
        .filter((id) => id && !previousIDs.has(id));

      if (flashTimeoutRef.current != null) {
        window.clearTimeout(flashTimeoutRef.current);
        flashTimeoutRef.current = null;
      }

      setPages([{ items: runs.items ?? [], nextCursor: runs.nextCursor ?? "", cursor: "" }]);
      setActivePage(1);

      if (newIDs.length > 0) {
        setFlashingIDs(new Set(newIDs));
        flashTimeoutRef.current = window.setTimeout(() => {
          setFlashingIDs(new Set());
          flashTimeoutRef.current = null;
        }, 1400);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setRefreshing(false);
    }
  }, [namespace, pages, workspaceName]);

  // When no initial data was provided (loader skipped the S3 fetch to keep
  // navigation fast), fetch the first page on mount.
  useEffect(() => {
    if (!initialRuns) {
      void refreshToLatest();
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Refresh the list when a run cycle completes so new entries appear without
  // requiring a manual reload.
  useEffect(() => {
    const previousPhase = previousPhaseRef.current;
    const previousRunID = previousRunIDRef.current;

    const cycleJustCompleted =
      (previousPhase === "Applying" && phase !== "Applying" && (phase === "Applied" || phase === "Failed")) ||
      (previousPhase === "Planning" && phase !== "Planning" && phase === "Failed") ||
      (previousRunID !== undefined && previousRunID !== currentRunID && previousPhase === "Applying");

    if (cycleJustCompleted) {
      const timer = window.setTimeout(() => void refreshToLatest(), 1250);
      previousPhaseRef.current = phase;
      previousRunIDRef.current = currentRunID;
      return () => window.clearTimeout(timer);
    }

    previousPhaseRef.current = phase;
    previousRunIDRef.current = currentRunID;
  }, [currentRunID, phase, refreshToLatest]);

  return (
    <>
      <SectionTable
        title="Run History"
        columns={[
          {
            key: "scheduledAt",
            label: "Scheduled",
            tooltip:
              "The time this run was originally due according to its schedule. \n\n Runs triggered by a configuration change or manually don't have a scheduled time. \n \n For scheduled runs, the actual start time may be later - this happens when an earlier rollout level (e.g. dev or staging) was still in progress when this workspace's turn came.",
          },
          { key: "startedAt", label: "Start time" },
          { key: "finishedAt", label: "End time" },
          { key: "duration", label: "Duration" },
          { key: "trigger", label: "Trigger" },
          { key: "revision", label: "Revision" },
          { key: "plan", label: "Plan" },
          { key: "apply", label: "Apply" },
        ]}
        rows={currentPage.items.map((run) => {
          const isFlashing = flashingIDs.has(run.runID ?? "");
          return {
            id: run.runID ?? "",
            onClick: () => setSelected(run),
            className: isFlashing ? "flash-highlight" : undefined,
            style: isFlashing
              ? ({ "--flash-color": flashColorVar("Applied") } as CSSProperties)
              : undefined,
            cells: [
              <Text size="sm" key="scheduledAt" c={run.scheduledAt ? undefined : "dimmed"}>
                {run.scheduledAt ? formatDateTime(run.scheduledAt) : "—"}
              </Text>,
              <Text size="sm" key="startedAt">
                {formatDateTime(run.startedAt)}
              </Text>,
              <Text size="sm" key="finishedAt" c={run.finishedAt ? undefined : "dimmed"}>
                {formatDateTime(run.finishedAt)}
              </Text>,
              <Text size="sm" key="duration">
                {formatDuration(run.startedAt, run.finishedAt)}
              </Text>,
              <TriggerBadge key="trigger" trigger={run.trigger} />,
              <Code key="revision" fz="xs">
                {displayRevision(run)}
              </Code>,
              <PhaseBadge key="plan" summary={run.plan} />,
              <PhaseBadge key="apply" summary={run.apply} />,
            ],
          };
        })}
        emptyMessage="No runs archived yet."
      />

      <Group justify="flex-end">
        <Button
          size="xs"
          variant="default"
          onClick={showNewerPage}
          disabled={!hasNewer || loadingPage || refreshing}
        >
          Newer
        </Button>
        <Button
          size="xs"
          variant="default"
          onClick={showOlderPage}
          disabled={!hasOlder || loadingPage || refreshing}
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
                Run
              </Text>
              {selected?.runID && <Code fz="xs">{selected.runID}</Code>}
              {selected?.trigger && selected.trigger !== "unknown" && (
                <TriggerBadge trigger={selected.trigger} />
              )}
            </Group>
            <Tooltip label={expanded ? "Collapse" : "Expand"}>
              <ActionIcon
                variant="subtle"
                color="gray"
                size="sm"
                onClick={() => setExpanded((v) => !v)}
              >
                {expanded ? <IconChevronDown size={16} /> : <IconChevronUp size={16} />}
              </ActionIcon>
            </Tooltip>
          </Group>
        }
        position="bottom"
        size={expanded ? "100%" : "60%"}
        styles={{
          content: { display: "flex", flexDirection: "column" },
          body: { flex: 1, minHeight: 0, display: "flex", flexDirection: "column", overflow: "hidden" },
        }}
      >
        {selected && (
          <Tabs
            defaultValue="plan"
            styles={{
              root: { display: "flex", flexDirection: "column", flex: 1, minHeight: 0 },
              panel: { flex: 1, minHeight: 0, display: "flex", flexDirection: "column" },
            }}
          >
            <Tabs.List>
              <Tabs.Tab value="plan">Plan</Tabs.Tab>
              <Tabs.Tab value="apply">Apply</Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="plan" pt="md">
              <LogPane
                key={`${selected.runID}:plan`}
                namespace={namespace}
                workspaceName={workspaceName}
                runID={selected.runID ?? ""}
                phase="plan"
                summary={selected.plan}
              />
            </Tabs.Panel>

            <Tabs.Panel value="apply" pt="md">
              <LogPane
                key={`${selected.runID}:apply`}
                namespace={namespace}
                workspaceName={workspaceName}
                runID={selected.runID ?? ""}
                phase="apply"
                summary={selected.apply}
              />
            </Tabs.Panel>
          </Tabs>
        )}
      </Drawer>
    </>
  );
}
