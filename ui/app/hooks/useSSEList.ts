import { useEffect, useRef, useState } from "react";
import type { WatchEvent } from "../api/types";

export function useSSEList<TApi, TRow extends { id: string }>(
  url: string,
  initial: TRow[],
  toRow: (item: TApi) => TRow,
  fetchItems?: () => Promise<TRow[]>
): TRow[] {
  const [items, setItems] = useState<TRow[]>(initial);
  const toRowRef = useRef(toRow);
  const fetchItemsRef = useRef(fetchItems);

  useEffect(() => {
    toRowRef.current = toRow;
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
      const event: WatchEvent<TApi> = JSON.parse(ev.data);
      if (!event.type || !event.object) return;
      if (event.type === "BOOKMARK" || event.type === "ERROR") return;

      const row = toRowRef.current(event.object);

      setItems((prev) => {
        switch (event.type) {
          case "ADDED":
            return prev.some((r) => r.id === row.id) ? prev : [...prev, row];
          case "MODIFIED":
            return prev.map((r) => (r.id === row.id ? row : r));
          case "DELETED":
            return prev.filter((r) => r.id !== row.id);
          default:
            return prev;
        }
      });
    };

    return () => source.close();
  }, [url]);

  return items;
}
