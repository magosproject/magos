import type { EventType, WatchEvent } from "../api/types";

export const EVENT_TYPE = {
  Added: "ADDED",
  Modified: "MODIFIED",
  Deleted: "DELETED",
  Error: "ERROR",
  Bookmark: "BOOKMARK",
} as const satisfies Record<string, EventType>;

export type ObjectWatchEvent<T> = {
  type: EventType;
  object: T;
};

export function toObjectWatchEvent<T>(event: WatchEvent<T>): ObjectWatchEvent<T> | null {
  if (!event.type || !event.object || event.type === EVENT_TYPE.Bookmark) return null;
  return { type: event.type, object: event.object };
}
