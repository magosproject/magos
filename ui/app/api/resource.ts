import type { ObjectMeta, ResourceObject } from "./types";

export function resourceId(resource: ResourceObject): string {
  return resource.metadata?.uid ?? resourceRef(resource.metadata);
}

export function resourceName(resource: ResourceObject): string {
  return resource.metadata?.name ?? "";
}

export function resourceNamespace(resource: ResourceObject): string {
  return resource.metadata?.namespace ?? "";
}

export function resourceRef(metadata?: ObjectMeta): string {
  return `${metadata?.namespace ?? ""}/${metadata?.name ?? ""}`;
}
