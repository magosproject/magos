import { useCallback, useEffect, useRef, useState } from "react";

export function useTransientIds(durationMs = 500): [Set<string>, (id: string) => void] {
  const [ids, setIds] = useState<Set<string>>(new Set());
  const timersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  const mark = useCallback((id: string) => {
    const existing = timersRef.current.get(id);
    if (existing) clearTimeout(existing);

    setIds((prev) => {
      const next = new Set(prev);
      next.add(id);
      return next;
    });

    const timer = setTimeout(() => {
      timersRef.current.delete(id);
      setIds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }, durationMs);

    timersRef.current.set(id, timer);
  }, [durationMs]);

  useEffect(() => {
    const timers = timersRef.current;
    return () => {
      timers.forEach(clearTimeout);
      timers.clear();
    };
  }, []);

  return [ids, mark];
}
