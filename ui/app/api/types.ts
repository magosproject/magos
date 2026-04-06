import type { components } from "./types.gen";

export type Project = components["schemas"]["internal_api_handlers.Project"];
export type Workspace = components["schemas"]["internal_api_handlers.Workspace"];
export type Rollout = components["schemas"]["internal_api_handlers.Rollout"];
export type VariableSet = components["schemas"]["internal_api_handlers.VariableSet"];

export type WatchEvent<T> = {
  type?: components["schemas"]["watch.EventType"];
  object?: T;
};

