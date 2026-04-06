import { useEffect, useRef, useState } from "react";

type WatchEvent = {
  type?: string;
  object?: unknown;
};

export function useSSEList<TRow extends { id: string }>(
  url: string,
  initial: TRow[],
  toRow: (item: unknown) => TRow,
  fetchItems?: () => Promise<TRow[]>
): TRow[] {
  const [items, setItems] = useState<TRow[]>(initial);
  const toRowRef = useRef(toRow);
  toRowRef.current = toRow;
  const fetchItemsRef = useRef(fetchItems);
  fetchItemsRef.current = fetchItems;

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
      const event: WatchEvent = JSON.parse(ev.data);
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

