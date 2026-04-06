import type { components } from "./types.gen";

export type Project = components["schemas"]["handlers.Project"];
export type Workspace = components["schemas"]["handlers.Workspace"];
export type Rollout = components["schemas"]["handlers.Rollout"];
export type VariableSet = components["schemas"]["handlers.VariableSet"];

export type WatchEvent<T> = {
  type?: components["schemas"]["watch.EventType"];
  object?: T;
};
