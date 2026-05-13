import { useEffect, useRef, useState } from "react";
import { resourceId } from "../api/resource";
import type { ResourceObject } from "../api/types";
import { EVENT_TYPE } from "../utils/events";
import { useSSEStream } from "./useSSEStream";
import { useTransientIds } from "./useTransientIds";

export function useSSEFiltered<T extends ResourceObject>(
  url: string,
  initial: T[],
  fetchItems?: () => Promise<T[]>
): [T[], Set<string>] {
  const [state, setState] = useState({ initial, items: initial });
  const [changedIds, markChanged] = useTransientIds();
  const fetchItemsRef = useRef(fetchItems);

  let items = state.items;
  if (state.initial !== initial) {
    items = initial;
    setState({ initial, items: initial });
  }

  const setItems = (next: T[] | ((current: T[]) => T[])) => {
    setState((current) => ({
      initial: current.initial,
      items: typeof next === "function" ? next(current.items) : next,
    }));
  };

  useEffect(() => {
    fetchItemsRef.current = fetchItems;
  });

  useSSEStream<T>(url, {
    onReconnect: () => {
      if (fetchItemsRef.current) {
        fetchItemsRef.current().then(setItems).catch(() => {});
      }
    },
    onEvent: (event) => {
      const uid = resourceId(event.object);

      setItems((prev) => {
        switch (event.type) {
          case EVENT_TYPE.Added:
            if (prev.some((r) => resourceId(r) === uid)) return prev;
            markChanged(uid);
            return [...prev, event.object];
          case EVENT_TYPE.Modified:
          case EVENT_TYPE.Error:
            markChanged(uid);
            return prev.map((r) => (resourceId(r) === uid ? event.object : r));
          case EVENT_TYPE.Deleted:
            return prev.filter((r) => resourceId(r) !== uid);
          default:
            return prev;
        }
      });
    },
  });

  return [items, changedIds];
}
