import { Code, Group, Loader, Text, Title } from "@mantine/core";
import AnsiToHtml from "ansi-to-html";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { apiUrl } from "../api/base";
import type { Phase, Run } from "../api/types";

type StreamEvent = {
  type?: "status" | "line" | "error" | "eof";
  runID?: string;
  phase?: "plan" | "apply";
  podName?: string;
  line?: string;
  message?: string;
};

interface Props {
  namespace: string;
  workspaceName: string;
  phase?: Phase;
  currentRunID?: string;
}

// activeStreamPhase returns the log phase to stream for the current workspace
// phase, or null when no active run is in progress.
function activeStreamPhase(phase?: Phase): "plan" | "apply" | null {
  switch (phase) {
    case "Planning":
      return "plan";
    case "Applying":
      return "apply";
    default:
      return null;
  }
}

export default function WorkspaceLiveConsole({
  namespace,
  workspaceName,
  phase,
  currentRunID,
}: Props) {
  const [status, setStatus] = useState("Loading latest completed run log");
  const viewportRef = useRef<HTMLDivElement>(null);
  const pendingRef = useRef<string[]>([]);
  const revealTimerRef = useRef<number | null>(null);
  const streamPhase = activeStreamPhase(phase);
  const isActive = Boolean(streamPhase && currentRunID);
  // streamKey changes whenever the run or active phase changes, causing all
  // effects that depend on it to re-run with a clean slate.
  const streamKey = useMemo(
    () => `${currentRunID ?? ""}:${streamPhase ?? ""}`,
    [streamPhase, currentRunID]
  );

  // Content is stored together with the streamKey it was produced for.
  // When streamKey changes the derived `content` value is automatically null
  // (loading), so no explicit reset logic is needed anywhere.
  const [contentState, setContentState] = useState<{
    key: string;
    value: string | null;
  }>({
    key: streamKey,
    value: null,
  });
  const content = contentState.value;

  const setContent = useCallback(
    (value: string | null) => setContentState({ key: streamKey, value }),
    [streamKey]
  );

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

  // When there is no active run, load the most recent completed run's apply
  // log (falling back to the plan log) so the console is never blank.
  useEffect(() => {
    if (isActive) return;

    const controller = new AbortController();

    fetch(
      apiUrl(
        `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs?limit=1`
      ),
      { signal: controller.signal, cache: "no-store" }
    )
      .then(async (response) => {
        if (!response.ok) throw new Error(`Request failed with status ${response.status}`);
        return response.json() as Promise<{ items?: Run[] }>;
      })
      .then(async (payload) => {
        const latest = payload.items?.[0];
        if (!latest?.runID) {
          setContent("");
          setStatus("No logs recorded yet");
          return;
        }

        // Prefer the apply log; fall back to plan when apply did not run.
        const logPhase = latest.apply ? "apply" : latest.plan ? "plan" : null;
        if (!logPhase) {
          setContent("");
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
        setContent(text);
        setStatus(`Showing latest completed ${logPhase} log`);
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        setContent("");
        setStatus(err instanceof Error ? err.message : "Failed to load latest logs");
      });

    return () => controller.abort();
  }, [isActive, namespace, workspaceName, streamKey, setContent]);

  // Stream live logs from the active run's pod when a plan or apply is in
  // progress. The EventSource is torn down and rebuilt whenever streamKey
  // changes so stale streams from a previous run do not bleed into the next.
  useEffect(() => {
    if (!isActive || !streamPhase) return;

    const pending = pendingRef.current;
    const source = new EventSource(
      apiUrl(
        `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs/current/log/stream?phase=${streamPhase}`
      )
    );

    source.onmessage = (event) => {
      const payload = JSON.parse(event.data) as StreamEvent;
      switch (payload.type) {
        case "status":
          break;
        case "line":
          pending.push(payload.line ?? "");
          if (revealTimerRef.current == null) {
            revealTimerRef.current = window.setInterval(flushPendingLines, 60);
          }
          break;
        case "error":
          setStatus(payload.message || `Error streaming ${streamPhase} logs`);
          break;
        case "eof":
          setStatus("Run phase completed");
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
      while (pending.length > 0) {
        flushPendingLines();
      }
    };
  }, [streamPhase, isActive, namespace, streamKey, workspaceName, currentRunID, flushPendingLines, stopRevealTimer]);

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
        {loading && <Loader size="sm" />}
        <Text size="sm" c="dimmed">
          {isActive && currentRunID
            ? contentState.key === streamKey && content
              ? `Streaming ${streamPhase} logs for job ${currentRunID}`
              : `Waiting for ${streamPhase} logs from job ${currentRunID}`
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
