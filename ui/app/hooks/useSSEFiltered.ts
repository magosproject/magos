import { useCallback, useEffect, useRef, useState } from "react";
import { resourceId } from "../api/resource";
import type { ResourceObject, WatchEvent } from "../api/types";

export function useSSEFiltered<T extends ResourceObject>(
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

      const uid = resourceId(event.object);

      setItems((prev) => {
        switch (event.type) {
          case "ADDED":
            if (prev.some((r) => resourceId(r) === uid)) return prev;
            markChanged(uid);
            return [...prev, event.object!];
          case "MODIFIED":
            markChanged(uid);
            return prev.map((r) => (resourceId(r) === uid ? event.object! : r));
          case "DELETED":
            return prev.filter((r) => resourceId(r) !== uid);
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
