import { Code, Group, Loader, Text, Title } from "@mantine/core";
import AnsiToHtml from "ansi-to-html";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { apiUrl } from "../api/base";
import type { Phase, Run, RunLogStreamEvent, RunLogStreamEventType } from "../api/types";
import { PHASE } from "../utils/phases";

const EVENT = {
  PhaseStart: "phase_start",
  Status: "status",
  Line: "line",
  Error: "error",
  EOF: "eof",
} as const satisfies Record<string, RunLogStreamEventType>;

const ACTIVE_CONSOLE_PHASES = new Set<Phase>([PHASE.Planning, PHASE.Planned, PHASE.Applying]);

interface Props {
  namespace: string;
  workspaceName: string;
  phase?: Phase;
  currentRunID?: string;
}

export default function WorkspaceLiveConsole({
  namespace,
  workspaceName,
  phase,
  currentRunID,
}: Props) {
  const [status, setStatus] = useState("Loading latest completed run log");
  const [podName, setPodName] = useState<string | null>(null);
  const [streamDone, setStreamDone] = useState(false);
  const [currentStreamPhase, setCurrentStreamPhase] = useState<"plan" | "apply" | null>(null);
  const viewportRef = useRef<HTMLDivElement>(null);
  const pendingRef = useRef<string[]>([]);
  const revealTimerRef = useRef<number | null>(null);
  const isActive = phase !== undefined && ACTIVE_CONSOLE_PHASES.has(phase);
  const streamKey = currentRunID ?? "";

  const [contentState, setContentState] = useState<{ key: string; value: string | null }>({
    key: streamKey,
    value: null,
  });
  const content = contentState.value;

  const stopRevealTimer = useCallback(() => {
    if (revealTimerRef.current != null) {
      window.clearInterval(revealTimerRef.current);
      revealTimerRef.current = null;
    }
  }, []);

  const flushPendingLines = useCallback(() => {
    const pending = pendingRef.current;
    if (pending.length === 0) return;
    const chunk = pending.splice(0, 2).join("\n");
    setContentState((current) => {
      const base = current.key === streamKey ? (current.value ?? "") : "";
      return { key: streamKey, value: `${base}${base && chunk ? "\n" : ""}${chunk}` };
    });
    if (pending.length === 0) stopRevealTimer();
  }, [streamKey, stopRevealTimer]);

  useEffect(() => {
    if (isActive) return;

    const controller = new AbortController();

    // A new run ID can appear while the workspace is still Pending or
    // Reconciling. Keep showing the previous archived log until planning/apply
    // actually starts, then let the live stream take over for the new run.
    fetch(
      apiUrl(`/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs?limit=1`),
      { signal: controller.signal, cache: "no-store" }
    )
      .then(async (response) => {
        if (!response.ok) throw new Error(`Request failed with status ${response.status}`);
        return response.json() as Promise<{ items?: Run[] }>;
      })
      .then(async (payload) => {
        const latest = payload.items?.[0];
        if (!latest?.runID) {
          setContentState((current) => ({ key: current.key, value: "" }));
          setStatus("No logs recorded yet");
          return;
        }
        const logPhase = latest.apply ? "apply" : latest.plan ? "plan" : null;
        if (!logPhase) {
          setContentState((current) => ({ key: current.key, value: "" }));
          setStatus("No logs recorded yet");
          return;
        }
        const response = await fetch(
          apiUrl(
            `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs/${latest.runID}/log?phase=${logPhase}`
          ),
          { signal: controller.signal, cache: "no-store" }
        );
        if (!response.ok) throw new Error(`Request failed with status ${response.status}`);
        const text = await response.text();
        setContentState((current) => ({ key: current.key, value: text }));
        setStatus(`Showing latest completed ${logPhase} log`);
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        setContentState((current) => ({ key: current.key, value: "" }));
        setStatus(err instanceof Error ? err.message : "Failed to load latest logs");
      });

    return () => controller.abort();
  }, [isActive, namespace, workspaceName]);

  useEffect(() => {
    if (!isActive) return;

    const pending = pendingRef.current;
    const source = new EventSource(
      apiUrl(
        `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs/current/log/stream`
      )
    );

    source.onmessage = (event) => {
      const payload = JSON.parse(event.data) as RunLogStreamEvent;
      switch (payload.type) {
        case EVENT.PhaseStart:
          // Clear the console when the apply phase begins. For the plan phase
          // (the first event on a fresh connection) there is nothing to clear.
          if (payload.phase === "apply") {
            pending.splice(0);
            stopRevealTimer();
            setContentState({ key: streamKey, value: "" });
          }
          setCurrentStreamPhase(payload.phase ?? null);
          setPodName(null);
          break;
        case EVENT.Status:
          if (payload.podName) setPodName(payload.podName);
          break;
        case EVENT.Line:
          pending.push(payload.line ?? "");
          if (revealTimerRef.current == null) {
            revealTimerRef.current = window.setInterval(flushPendingLines, 60);
          }
          break;
        case EVENT.Error:
          setStatus(payload.message || "Error streaming logs");
          break;
        case EVENT.EOF:
          setStreamDone(true);
          setStatus("Run completed");
          source.close();
          break;
      }
    };

    source.onerror = () => {
      setStatus("Stream connection lost");
    };

    return () => {
      source.close();
      stopRevealTimer();
      setPodName(null);
      setStreamDone(false);
      setCurrentStreamPhase(null);
      while (pending.length > 0) {
        flushPendingLines();
      }
    };
  }, [isActive, namespace, streamKey, workspaceName, flushPendingLines, stopRevealTimer]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    viewport.scrollTop = viewport.scrollHeight;
  }, [content]);

  const loading = content === null || (isActive && contentState.key !== streamKey);
  const ansiConverter = useMemo(() => new AnsiToHtml({ escapeXML: true, stream: false }), []);

  return (
    <div style={{ height: 430, display: "flex", flexDirection: "column", gap: 8 }}>
      <Title order={4}>Live Console</Title>
      <Group gap="xs">
        {loading && <Loader size="xs" />}
        <Text size="sm" c="dimmed">
          {isActive
            ? streamDone
              ? currentStreamPhase && podName
                ? `Run completed – showing ${currentStreamPhase} logs for pod ${podName}`
                : "Run completed"
              : currentStreamPhase
                ? podName
                  ? `Streaming ${currentStreamPhase} logs for pod ${podName}`
                  : `Waiting for ${currentStreamPhase} logs`
                : "Connecting..."
            : status}
        </Text>
      </Group>
      <div ref={viewportRef} style={{ flex: 1, minHeight: 0, overflow: "auto" }}>
        <Code
          block
          style={{
            minHeight: "100%",
            boxSizing: "border-box",
            whiteSpace: "pre-wrap",
            overflowWrap: "anywhere",
          }}
        >
          <span
            dangerouslySetInnerHTML={{
              __html: ansiConverter.toHtml(content || "Waiting for the latest completed run log."),
            }}
          />
        </Code>
      </div>
    </div>
  );
}
