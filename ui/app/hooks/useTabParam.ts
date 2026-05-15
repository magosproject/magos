import { useSearchParams } from "react-router";

export function useTabParam(defaultTab: string) {
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = searchParams.get("tab") ?? defaultTab;

  function setTab(tab: string | null) {
    if (!tab) return;
    setSearchParams((prev) => { prev.set("tab", tab); return prev; }, { replace: true });
  }

  return [activeTab, setTab] as const;
}

