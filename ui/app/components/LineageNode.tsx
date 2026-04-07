import {
  Handle,
  Position,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import { Text } from "@mantine/core";
import type { ReactNode } from "react";

export interface LineageNodeData {
  kindLabel: string;
  content: ReactNode;
  [key: string]: unknown;
}

export default function LineageNode({
  data,
  sourcePosition,
  targetPosition,
}: NodeProps<Node<LineageNodeData>>) {
  return (
    <>
      <Handle
        type="target"
        position={targetPosition ?? Position.Top}
        style={{ visibility: "hidden" }}
      />
      <Text size="10px" c="dimmed" tt="uppercase" fw={700} ta="center" mb={4}>
        {data.kindLabel}
      </Text>
      {data.content}
      <Handle
        type="source"
        position={sourcePosition ?? Position.Bottom}
        style={{ visibility: "hidden" }}
      />
    </>
  );
}
