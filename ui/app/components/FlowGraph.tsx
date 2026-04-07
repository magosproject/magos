import { useCallback, useEffect, useRef } from "react";
import {
  ReactFlow,
  Controls,
  type Edge,
  type Node,
  type NodeTypes,
  useNodesState,
  useEdgesState,
  useReactFlow,
  ReactFlowProvider,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Box } from "@mantine/core";

interface FlowGraphProps {
  nodes: Node[];
  edges: Edge[];
  nodeTypes: NodeTypes;
  height?: number | string;
}

function FlowGraphInner({ nodes, edges, nodeTypes, height = 520 }: FlowGraphProps) {
  const { fitView } = useReactFlow();
  const wrapperRef = useRef<HTMLDivElement>(null);

  const [rfNodes, setNodes, onNodesChange] = useNodesState(nodes);
  const [rfEdges, setEdges, onEdgesChange] = useEdgesState(edges);

  useEffect(() => { setNodes(nodes); }, [nodes, setNodes]);
  useEffect(() => { setEdges(edges); }, [edges, setEdges]);

  const handleFit = useCallback(() => {
    fitView({ padding: 0.3, minZoom: 0.5, maxZoom: 1.5, duration: 600 });
  }, [fitView]);

  useEffect(() => {
    if (!wrapperRef.current) return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.width > 0 && entry.contentRect.height > 0) {
          window.requestAnimationFrame(handleFit);
        }
      }
    });
    observer.observe(wrapperRef.current);
    return () => observer.disconnect();
  }, [handleFit]);

  return (
    <Box
      ref={wrapperRef}
      h={height}
      w="100%"
      style={{
        border: "1px solid var(--mantine-color-default-border)",
        borderRadius: "var(--mantine-radius-md)",
      }}
    >
      <ReactFlow
        nodes={rfNodes}
        edges={rfEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        fitView
        panOnDrag
        preventScrolling={false}
        proOptions={{ hideAttribution: true }}
      >
        <Controls showInteractive={false} />
      </ReactFlow>
    </Box>
  );
}

export default function FlowGraph(props: FlowGraphProps) {
  return (
    <ReactFlowProvider>
      <FlowGraphInner {...props} />
    </ReactFlowProvider>
  );
}


