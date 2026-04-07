import { useSyncExternalStore } from "react";

let visible = true;

const listeners = new Set<() => void>();

setInterval(() => {
  visible = !visible;
  listeners.forEach((fn) => fn());
}, 500);

function subscribe(cb: () => void) {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

function getSnapshot() {
  return visible;
}

export function useBlinkVisible(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

