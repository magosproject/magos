import { useCallback, useEffect, useRef, useState } from "react";
import type { WatchEvent } from "../api/types";

export function useSSEList<TApi, TRow extends { id: string }>(
  url: string,
  initial: TRow[],
  toRow: (item: TApi) => TRow,
  fetchItems?: () => Promise<TRow[]>
): [TRow[], Set<string>] {
  const [items, setItems] = useState<TRow[]>(initial);
  const [changedIds, setChangedIds] = useState<Set<string>>(new Set());
  const timersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const toRowRef = useRef(toRow);
  const fetchItemsRef = useRef(fetchItems);

  useEffect(() => {
    toRowRef.current = toRow;
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
      const event: WatchEvent<TApi> = JSON.parse(ev.data);
      if (!event.type || !event.object) return;
      if (event.type === "BOOKMARK") return;

      const row = toRowRef.current(event.object);

      setItems((prev) => {
        switch (event.type) {
          case "ADDED":
            if (prev.some((r) => r.id === row.id)) return prev;
            markChanged(row.id);
            return [...prev, row];
          case "MODIFIED":
          case "ERROR":
            markChanged(row.id);
            return prev.map((r) => (r.id === row.id ? row : r));
          case "DELETED":
            return prev.filter((r) => r.id !== row.id);
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
