import { useEffect, useRef, useState } from "react";
import { EVENT_TYPE } from "../utils/events";
import { useSSEStream } from "./useSSEStream";

export function useSSEItem<T>(
  url: string,
  initial: T,
  match: (obj: T) => boolean,
  fetchItem?: () => Promise<T>
): T {
  const [item, setItem] = useState<T>(initial);
  const matchRef = useRef(match);
  const fetchItemRef = useRef(fetchItem);

  useEffect(() => {
    setItem(initial);
  }, [initial]);

  useEffect(() => {
    matchRef.current = match;
    fetchItemRef.current = fetchItem;
  });

  useSSEStream<T>(url, {
    onReconnect: () => {
      if (fetchItemRef.current) {
        fetchItemRef.current().then(setItem).catch(() => {});
      }
    },
    onEvent: (event) => {
      if (!matchRef.current(event.object)) return;
      if (
        event.type === EVENT_TYPE.Added ||
        event.type === EVENT_TYPE.Modified ||
        event.type === EVENT_TYPE.Error
      ) {
        setItem(event.object);
      }
    },
  });

  return item;
}
