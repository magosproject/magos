import { startTransition, useEffect, useRef, useState } from "react";
import type { ObjectWatchEvent } from "../utils/events";
import { EVENT_TYPE } from "../utils/events";
import { useSSEStream } from "./useSSEStream";
import { useTransientIds } from "./useTransientIds";

const FLUSH_INTERVAL_MS = 150;

export function useSSEList<TApi, TRow extends { id: string }>(
  url: string,
  initial: TRow[],
  toRow: (item: TApi) => TRow,
  fetchItems?: () => Promise<TRow[]>
): [TRow[], Set<string>] {
  const [state, setState] = useState({ initial, items: initial });
  const [changedIds, markChanged] = useTransientIds();
  const toRowRef = useRef(toRow);
  const fetchItemsRef = useRef(fetchItems);
  const markChangedRef = useRef(markChanged);
  const pendingRef = useRef<ObjectWatchEvent<TApi>[]>([]);
  // Mirrors items state so flush can compute against the current list without
  // a functional setItems updater (which would run side effects twice in strict mode).
  const itemsRef = useRef<TRow[]>(initial);

  let items = state.items;
  if (state.initial !== initial) {
    items = initial;
    setState({ initial, items: initial });
  }

  useEffect(() => {
    itemsRef.current = items;
  }, [items]);

  useEffect(() => {
    toRowRef.current = toRow;
    fetchItemsRef.current = fetchItems;
    markChangedRef.current = markChanged;
  });

  useSSEStream<TApi>(url, {
    onReconnect: () => {
      if (fetchItemsRef.current) {
        fetchItemsRef.current()
          .then((newItems) => {
            itemsRef.current = newItems;
            setState((current) => ({ initial: current.initial, items: newItems }));
          })
          .catch(() => {});
      }
    },
    onEvent: (event) => {
      pendingRef.current.push(event);
    },
  });

  useEffect(() => {
    const flush = () => {
      const pending = pendingRef.current;
      if (pending.length === 0) return;
      pendingRef.current = [];

      let next = itemsRef.current;
      const changedRowIds: string[] = [];

      for (const event of pending) {
        const row = toRowRef.current(event.object);
        switch (event.type) {
          case EVENT_TYPE.Added:
            if (!next.some((r) => r.id === row.id)) {
              changedRowIds.push(row.id);
              next = [...next, row];
            }
            break;
          case EVENT_TYPE.Modified:
          case EVENT_TYPE.Error:
            changedRowIds.push(row.id);
            next = next.map((r) => (r.id === row.id ? row : r));
            break;
          case EVENT_TYPE.Deleted:
            next = next.filter((r) => r.id !== row.id);
            break;
        }
      }

      if (next === itemsRef.current) return;

      itemsRef.current = next;
      // startTransition marks this as a background update so that user
      // interactions (navigation clicks) are not blocked waiting for renders
      // triggered by SSE events to complete.
      startTransition(() => {
        setState((current) => ({ initial: current.initial, items: next }));
        changedRowIds.forEach((id) => markChangedRef.current(id));
      });
    };

    const id = setInterval(flush, FLUSH_INTERVAL_MS);
    return () => clearInterval(id);
  }, []);

  return [items, changedIds];
}
