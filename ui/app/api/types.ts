import type { components } from "./types.gen";

export type Project = components["schemas"]["handlers.Project"];
export type Workspace = components["schemas"]["handlers.Workspace"];
export type Rollout = components["schemas"]["handlers.Rollout"];
export type VariableSet = components["schemas"]["handlers.VariableSet"];

export type Phase = components["schemas"]["v1alpha1.Phase"];
export type EventType = components["schemas"]["watch.EventType"];
export type Condition = components["schemas"]["v1.Condition"];
export type ObjectMeta = components["schemas"]["v1.ObjectMeta"];
export type RolloutStep = components["schemas"]["v1alpha1.RolloutStep"];
export type LabelSelector = components["schemas"]["v1.LabelSelector"];

export type ResourceObject = {
  metadata?: ObjectMeta;
};

export type WatchEvent<T> = {
  type?: EventType;
  object?: T;
};
