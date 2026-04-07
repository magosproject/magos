import {
  Handle,
  Position,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import { Text } from "@mantine/core";
import ResourceCard, { type ResourceCardProps } from "./ResourceCard";

export interface LineageNodeData extends ResourceCardProps {
  kindLabel: string;
  [key: string]: unknown;
}

export default function LineageNode({ data }: NodeProps<Node<LineageNodeData>>) {
  const { kindLabel, ...cardProps } = data;

  return (
    <>
      <Handle type="target" position={Position.Top} style={{ visibility: "hidden" }} />
      <Text size="10px" c="dimmed" tt="uppercase" fw={700} ta="center" mb={4}>
        {kindLabel}
      </Text>
      <ResourceCard {...cardProps} />
      <Handle type="source" position={Position.Bottom} style={{ visibility: "hidden" }} />
    </>
  );
}


