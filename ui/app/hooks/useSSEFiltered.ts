import { useEffect, useRef, useState } from "react";
import type { WatchEvent } from "../api/types";

type WithMetadata = { metadata?: { uid?: string; namespace?: string; name?: string } };

function objectId(obj: WithMetadata): string {
  return obj.metadata?.uid ?? `${obj.metadata?.namespace}/${obj.metadata?.name}`;
}

// useSSEFiltered manages a list of raw API objects via SSE.
// The server is expected to handle filtering via query params in the URL.
// fetchItems is called on reconnect to re-sync.
export function useSSEFiltered<T extends WithMetadata>(
  url: string,
  initial: T[],
  fetchItems?: () => Promise<T[]>
): T[] {
  const [items, setItems] = useState<T[]>(initial);
  const fetchItemsRef = useRef(fetchItems);

  useEffect(() => {
    fetchItemsRef.current = fetchItems;
  });

  useEffect(() => {
    const source = new EventSource(url);
    let opened = false;

    source.onopen = () => {
      if (opened && fetchItemsRef.current) {
        fetchItemsRef.current().then(setItems).catch(() => {});
      }
      opened = true;
    };

    source.onmessage = (ev: MessageEvent<string>) => {
      const event: WatchEvent<T> = JSON.parse(ev.data);
      if (!event.type || !event.object) return;
      if (event.type === "BOOKMARK") return;

      const uid = objectId(event.object);

      setItems((prev) => {
        switch (event.type) {
          case "ADDED":
            return prev.some((r) => objectId(r) === uid) ? prev : [...prev, event.object!];
          case "MODIFIED":
            return prev.map((r) => (objectId(r) === uid ? event.object! : r));
          case "DELETED":
            return prev.filter((r) => objectId(r) !== uid);
          default:
            return prev;
        }
      });
    };

    return () => source.close();
  }, [url]);

  return items;
}

