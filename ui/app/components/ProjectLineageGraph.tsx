import React, { useEffect, useMemo, useRef, useCallback } from "react";
import {
  ReactFlow,
  Controls,
  Background,
  MarkerType,
  Position,
  Handle,
  type Edge,
  type Node,
  type NodeProps,
  useNodesState,
  useEdgesState,
  ReactFlowProvider,
  useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Box, Text, Group, ThemeIcon, Stack, useMantineTheme } from "@mantine/core";
import { IconBox, IconFolder, IconBraces } from "@tabler/icons-react";
import { useNavigate } from "react-router";
import type { Project, Workspace } from "../api/types";
import { statusColor } from "../utils/colors";
import StatusBadge from "./StatusBadge";

interface LineageNodeData {
  kind: "project" | "workspace" | "variableset";
  name: string;
  status?: string;
  [key: string]: unknown;
}

const kindConfig = {
  project: { label: "Project", icon: IconBox, color: "blue" },
  workspace: { label: "Workspace", icon: IconFolder, color: "magos" },
  variableset: { label: "Variable Set", icon: IconBraces, color: "grape" },
} as const;

function LineageNode({ data }: NodeProps<Node<LineageNodeData>>) {
  const { kind, name, status } = data;
  const cfg = kindConfig[kind];
  const Icon = cfg.icon;

  return (
    <>
      <Handle type="target" position={Position.Top} style={{ visibility: "hidden" }} />
      <div className={`lineage-node lineage-node--${kind}`} data-status={status ?? ""}>
        <Stack gap={4} align="center">
          <Text size="10px" c="dimmed" tt="uppercase" fw={700} lh={1}>
            {cfg.label}
          </Text>
          <Group gap={6} wrap="nowrap" align="center">
            <ThemeIcon size={18} variant="light" color={cfg.color} radius="sm">
              <Icon size={11} />
            </ThemeIcon>
            <Text size="xs" fw={600} truncate="end" maw={120}>
              {name}
            </Text>
          </Group>
          {status && <StatusBadge status={status} size="xs" />}
        </Stack>
      </div>
      <Handle type="source" position={Position.Bottom} style={{ visibility: "hidden" }} />
    </>
  );
}

const nodeTypes = { lineageNode: LineageNode };

interface ProjectLineageGraphProps {
  project: Project;
  variableSetRefs: string[];
  workspaces: Workspace[];
}

function ProjectLineageGraphInner({
  project,
  variableSetRefs,
  workspaces,
}: ProjectLineageGraphProps) {
  const theme = useMantineTheme();
  const { fitView } = useReactFlow();
  const wrapperRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  const projectName = project.metadata?.name ?? "";
  const projectNamespace = project.metadata?.namespace ?? "";

  const onNodeClick = useCallback(
    (_event: React.MouseEvent, node: Node) => {
      if (node.id.startsWith("ws-")) {
        const [ns, n] = node.id.replace("ws-", "").split("/");
        navigate(`/workspaces/${ns}/${n}`);
      } else if (node.id.startsWith("vs-")) {
        const vsName = node.id.replace("vs-", "");
        navigate(`/variable-sets/${projectNamespace}/${vsName}`);
      }
    },
    [navigate, projectNamespace]
  );

  const nodeSpacing = 180;
  const getStartX = (targetX: number, count: number) =>
    targetX - ((count - 1) * nodeSpacing) / 2;

  const computedNodes = useMemo(() => {
    const nodes: Node<LineageNodeData>[] = [];
    const projX = 250;
    const projY = 140;

    nodes.push({
      id: `proj-${projectNamespace}/${projectName}`,
      type: "lineageNode",
      position: { x: projX, y: projY },
      draggable: false,
      data: { kind: "project", name: projectName, status: project.status?.phase ?? "" },
    });

    const vsStartX = getStartX(projX, variableSetRefs.length);
    variableSetRefs.forEach((vsName, i) => {
      nodes.push({
        id: `vs-${vsName}`,
        type: "lineageNode",
        position: { x: vsStartX + i * nodeSpacing, y: 0 },
        draggable: false,
        data: { kind: "variableset", name: vsName },
      });
    });

    const wsStartX = getStartX(projX, workspaces.length);
    workspaces.forEach((ws, i) => {
      nodes.push({
        id: `ws-${ws.metadata?.namespace ?? ""}/${ws.metadata?.name ?? ""}`,
        type: "lineageNode",
        position: { x: wsStartX + i * nodeSpacing, y: 280 },
        draggable: false,
        data: { kind: "workspace", name: ws.metadata?.name ?? "", status: ws.status?.phase ?? "" },
      });
    });

    return nodes;
  }, [project, variableSetRefs, workspaces, projectName, projectNamespace]);

  const computedEdges = useMemo(() => {
    const edges: Edge[] = [];

    variableSetRefs.forEach((vsName) => {
      edges.push({
        id: `e-vs-${vsName}-proj`,
        source: `vs-${vsName}`,
        target: `proj-${projectNamespace}/${projectName}`,
        type: "smoothstep",
        animated: true,
        markerEnd: { type: MarkerType.ArrowClosed, color: theme.colors.gray[6] },
        style: { stroke: theme.colors.gray[6], strokeWidth: 2 },
      });
    });

    workspaces.forEach((ws) => {
      const wsNs = ws.metadata?.namespace ?? "";
      const wsName = ws.metadata?.name ?? "";
      const phase = ws.status?.phase ?? "";
      const color = statusColor[phase] ?? "gray";
      const resolvedColor = theme.colors[color]?.[6] ?? theme.colors.gray[6];

      edges.push({
        id: `e-proj-ws-${wsNs}/${wsName}`,
        source: `proj-${projectNamespace}/${projectName}`,
        target: `ws-${wsNs}/${wsName}`,
        type: "smoothstep",
        animated: true,
        markerEnd: { type: MarkerType.ArrowClosed, color: resolvedColor },
        style: { stroke: resolvedColor, strokeWidth: 2 },
      });
    });

    return edges;
  }, [workspaces, variableSetRefs, theme, projectName, projectNamespace]);

  const [nodes, setNodes, onNodesChange] = useNodesState(computedNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(computedEdges);

  useEffect(() => {
    setNodes(computedNodes);
  }, [computedNodes, setNodes]);

  useEffect(() => {
    setEdges(computedEdges);
  }, [computedEdges, setEdges]);

  useEffect(() => {
    if (!wrapperRef.current) return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.width > 0 && entry.contentRect.height > 0) {
          window.requestAnimationFrame(() => {
            fitView({ padding: 0.3, minZoom: 0.5, maxZoom: 1.5, duration: 600 });
          });
        }
      }
    });
    observer.observe(wrapperRef.current);
    return () => observer.disconnect();
  }, [fitView]);

  return (
    <Box
      ref={wrapperRef}
      h={400}
      w="100%"
      style={{
        border: "1px solid var(--mantine-color-default-border)",
        borderRadius: "var(--mantine-radius-md)",
      }}
    >
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={onNodeClick}
        nodeTypes={nodeTypes}
        fitView
        panOnDrag
        zoomOnScroll={false}
        zoomOnPinch
        preventScrolling={false}
        proOptions={{ hideAttribution: true }}
      >
        <Controls showInteractive={false} />
        <Background />
      </ReactFlow>
    </Box>
  );
}

export default function ProjectLineageGraph(props: ProjectLineageGraphProps) {
  return (
    <ReactFlowProvider>
      <ProjectLineageGraphInner {...props} />
    </ReactFlowProvider>
  );
}
