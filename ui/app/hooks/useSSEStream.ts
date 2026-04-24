import { useEffect, useRef } from "react";
import type { WatchEvent } from "../api/types";
import { type ObjectWatchEvent, toObjectWatchEvent } from "../utils/events";

interface UseSSEStreamOptions<T> {
  onEvent: (event: ObjectWatchEvent<T>) => void;
  onReconnect?: () => void;
}

export function useSSEStream<T>(url: string, { onEvent, onReconnect }: UseSSEStreamOptions<T>) {
  const onEventRef = useRef(onEvent);
  const onReconnectRef = useRef(onReconnect);

  useEffect(() => {
    onEventRef.current = onEvent;
    onReconnectRef.current = onReconnect;
  });

  useEffect(() => {
    const source = new EventSource(url);
    let opened = false;

    source.onopen = () => {
      if (opened) onReconnectRef.current?.();
      opened = true;
    };

    source.onmessage = (ev: MessageEvent<string>) => {
      const event = toObjectWatchEvent(JSON.parse(ev.data) as WatchEvent<T>);
      if (!event) return;
      onEventRef.current(event);
    };

    return () => source.close();
  }, [url]);
}
