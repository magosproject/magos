import type { Workspace } from "./types";

const cache = new Map<string, Workspace>();

export function setWorkspaceCache(ws: Workspace): void {
  const ns = ws.metadata?.namespace ?? "";
  const name = ws.metadata?.name ?? "";
  if (ns && name) cache.set(`${ns}/${name}`, ws);
}

export function getWorkspaceCache(namespace: string, name: string): Workspace | undefined {
  return cache.get(`${namespace}/${name}`);
}
