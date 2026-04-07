import { useCallback, useEffect, useRef, useState } from "react";
import type { WatchEvent } from "../api/types";

type WithMetadata = { metadata?: { uid?: string; namespace?: string; name?: string } };

function objectId(obj: WithMetadata): string {
  return obj.metadata?.uid ?? `${obj.metadata?.namespace}/${obj.metadata?.name}`;
}

export function useSSEFiltered<T extends WithMetadata>(
  url: string,
  initial: T[],
  fetchItems?: () => Promise<T[]>
): [T[], Set<string>] {
  const [items, setItems] = useState<T[]>(initial);
  const [changedIds, setChangedIds] = useState<Set<string>>(new Set());
  const timersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const fetchItemsRef = useRef(fetchItems);

  useEffect(() => {
    fetchItemsRef.current = fetchItems;
  });

  const markChanged = useCallback((id: string) => {
    const existing = timersRef.current.get(id);
    if (existing) clearTimeout(existing);

    setChangedIds((prev) => {
      const next = new Set(prev);
      next.add(id);
      return next;
    });

    const timer = setTimeout(() => {
      timersRef.current.delete(id);
      setChangedIds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }, 500);

    timersRef.current.set(id, timer);
  }, []);

  useEffect(() => {
    const source = new EventSource(url);
    const timers = timersRef.current;
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
            if (prev.some((r) => objectId(r) === uid)) return prev;
            markChanged(uid);
            return [...prev, event.object!];
          case "MODIFIED":
            markChanged(uid);
            return prev.map((r) => (objectId(r) === uid ? event.object! : r));
          case "DELETED":
            return prev.filter((r) => objectId(r) !== uid);
          default:
            return prev;
        }
      });
    };

    return () => {
      source.close();
      timers.forEach(clearTimeout);
      timers.clear();
    };
  }, [url, markChanged]);

  return [items, changedIds];
}

