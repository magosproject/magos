import { Code, Loader, ScrollArea, Stack, Text, Title } from "@mantine/core";
import { useEffect, useMemo, useRef, useState } from "react";
import { apiUrl } from "../api/base";
import type { Phase } from "../api/types";

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

function streamPhase(phase?: Phase): "apply" | "" {
  if (phase === "Applying") {
    return "apply";
  }
  return "";
}

export default function WorkspaceLiveConsole({
  namespace,
  workspaceName,
  phase,
  currentRunID,
}: Props) {
  const [content, setContent] = useState("");
  const [status, setStatus] = useState("Loading latest completed apply log");
  const [loading, setLoading] = useState(false);
  const viewportRef = useRef<HTMLDivElement>(null);
  const pendingRef = useRef<string[]>([]);
  const revealTimerRef = useRef<number | null>(null);
  const currentPhase = streamPhase(phase);
  const isActive = Boolean(currentPhase && currentRunID);
  const streamKey = useMemo(
    () => `${currentRunID ?? ""}:${currentPhase}`,
    [currentPhase, currentRunID]
  );

  function stopRevealTimer() {
    if (revealTimerRef.current != null) {
      window.clearInterval(revealTimerRef.current);
      revealTimerRef.current = null;
    }
  }

  function flushPendingLines() {
    if (pendingRef.current.length === 0) {
      return;
    }
    const chunk = pendingRef.current.splice(0, 2).join("\n");
    setContent((current) => `${current}${current && chunk ? "\n" : ""}${chunk}`);
    if (pendingRef.current.length === 0) {
      stopRevealTimer();
    }
  }

  useEffect(() => {
    setStatus(
      isActive && currentRunID
        ? `Waiting for live logs from apply job ${currentRunID}`
        : "Loading latest completed apply log"
    );
  }, [currentRunID, isActive, streamKey]);

  useEffect(() => {
    if (isActive) {
      return;
    }

    const controller = new AbortController();
    setLoading(true);

    fetch(
      apiUrl(
        `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs?phase=apply&limit=1`
      ),
      { signal: controller.signal, cache: "no-store" }
    )
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`Request failed with status ${response.status}`);
        }
        return response.json() as Promise<{ items?: Array<{ runID?: string }> }>;
      })
      .then(async (payload) => {
        const latestRunID = payload.items?.[0]?.runID;
        if (!latestRunID) {
          setContent("");
          setStatus("No logs recorded yet");
          return;
        }

        const response = await fetch(
          apiUrl(
            `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs/${latestRunID}/log?phase=apply`
          ),
          { signal: controller.signal, cache: "no-store" }
        );
        if (!response.ok) {
          throw new Error(`Request failed with status ${response.status}`);
        }

        const text = await response.text();
        setContent(text);
        setStatus("Showing latest completed apply log");
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        setStatus(err instanceof Error ? err.message : "Failed to load latest logs");
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      });

    return () => controller.abort();
  }, [isActive, namespace, workspaceName, streamKey]);

  useEffect(() => {
    if (!isActive || !currentPhase) {
      setLoading(false);
      return;
    }

    const source = new EventSource(
      apiUrl(
        `/apis/magosproject.io/v1alpha1/workspaces/${namespace}/${workspaceName}/runs/current/log/stream?phase=${currentPhase}`
      )
    );

    setLoading(true);

    source.onmessage = (event) => {
      const payload = JSON.parse(event.data) as StreamEvent;
      switch (payload.type) {
        case "status":
          setStatus(payload.message || `Waiting for live logs from apply job ${currentRunID}`);
          setLoading(false);
          break;
        case "line":
          pendingRef.current.push(payload.line ?? "");
          if (revealTimerRef.current == null) {
            revealTimerRef.current = window.setInterval(() => {
              flushPendingLines();
            }, 60);
          }
          setLoading(false);
          break;
        case "error":
          setStatus(payload.message || `Waiting for live logs from apply job ${currentRunID}`);
          setLoading(false);
          break;
        case "eof":
          setStatus("Run completed");
          setLoading(false);
          source.close();
          break;
      }
    };

    source.onerror = () => {
      setLoading(false);
    };

    return () => {
      source.close();
      stopRevealTimer();
      while (pendingRef.current.length > 0) {
        flushPendingLines();
      }
    };
  }, [currentPhase, isActive, namespace, streamKey, workspaceName]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    viewport.scrollTop = viewport.scrollHeight;
  }, [content]);

  return (
    <Stack gap="xs" h={430}>
      <Title order={4}>Live Console</Title>
      <Text size="sm" c="dimmed">
        {status}
      </Text>
      {loading && !content && <Loader size="sm" />}
      <ScrollArea
        viewportRef={viewportRef}
        style={{ flex: 1 }}
        type="always"
        offsetScrollbars="y"
        scrollbarSize={10}
      >
        <Code
          block
          style={{
            boxSizing: "border-box",
            height: "100%",
            whiteSpace: "pre-wrap",
            overflowWrap: "anywhere",
          }}
        >
          {content || "Waiting for the latest completed apply log."}
        </Code>
      </ScrollArea>
    </Stack>
  );
}
