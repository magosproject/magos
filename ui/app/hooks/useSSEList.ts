import { useEffect, useRef, useState } from "react";
import { EVENT_TYPE } from "../utils/events";
import { useSSEStream } from "./useSSEStream";
import { useTransientIds } from "./useTransientIds";

export function useSSEList<TApi, TRow extends { id: string }>(
  url: string,
  initial: TRow[],
  toRow: (item: TApi) => TRow,
  fetchItems?: () => Promise<TRow[]>
): [TRow[], Set<string>] {
  const [items, setItems] = useState<TRow[]>(initial);
  const [changedIds, markChanged] = useTransientIds();
  const toRowRef = useRef(toRow);
  const fetchItemsRef = useRef(fetchItems);

  useEffect(() => {
    setItems(initial);
  }, [initial]);

  useEffect(() => {
    toRowRef.current = toRow;
    fetchItemsRef.current = fetchItems;
  });

  useSSEStream<TApi>(url, {
    onReconnect: () => {
      if (fetchItemsRef.current) {
        fetchItemsRef.current().then(setItems).catch(() => {});
      }
    },
    onEvent: (event) => {
      const row = toRowRef.current(event.object);

      setItems((prev) => {
        switch (event.type) {
          case EVENT_TYPE.Added:
            if (prev.some((r) => r.id === row.id)) return prev;
            markChanged(row.id);
            return [...prev, row];
          case EVENT_TYPE.Modified:
          case EVENT_TYPE.Error:
            markChanged(row.id);
            return prev.map((r) => (r.id === row.id ? row : r));
          case EVENT_TYPE.Deleted:
            return prev.filter((r) => r.id !== row.id);
          default:
            return prev;
        }
      });
    },
  });

  return [items, changedIds];
}
