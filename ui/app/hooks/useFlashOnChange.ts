import { useEffect, useState } from "react";

export function useFlashOnChange(value: unknown): boolean {
  const [flash, setFlash] = useState(false);
  const [prev, setPrev] = useState(value);

  if (prev !== value) {
    setPrev(value);
    setFlash(true);
  }

  useEffect(() => {
    if (!flash) return;
    const timer = setTimeout(() => setFlash(false), 500);
    return () => clearTimeout(timer);
  }, [flash]);

  return flash;
}




