import { useEffect, useRef, useState } from "react";
import type { WatchEvent } from "../api/types";

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
    matchRef.current = match;
    fetchItemRef.current = fetchItem;
  });

  useEffect(() => {
    const source = new EventSource(url);
    let opened = false;

    source.onopen = () => {
      if (opened && fetchItemRef.current) {
        fetchItemRef.current().then(setItem).catch(() => {});
      }
      opened = true;
    };

    source.onmessage = (ev: MessageEvent<string>) => {
      const event: WatchEvent<T> = JSON.parse(ev.data);
      if (!event.type || !event.object) return;
      if (event.type === "BOOKMARK") return;

      if (matchRef.current(event.object)) {
        if (event.type === "ADDED" || event.type === "MODIFIED" || event.type === "ERROR") {
          setItem(event.object);
        }
      }
    };

    return () => source.close();
  }, [url]);

  return item;
}

