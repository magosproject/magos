import { useEffect, useMemo, useRef } from "react";
import {
  ReactFlow,
  Controls,
  Background,
  MarkerType,
  type Edge,
  type Node,
  useNodesState,
  useEdgesState,
  ReactFlowProvider,
  useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Box, useMantineTheme } from "@mantine/core";
import { IconBox, IconFolder } from "@tabler/icons-react";
import type { CSSProperties } from "react";
import type { Project, Workspace } from "../api/types";
import { statusColor, flashColorVar } from "../utils/colors";
import { repoIcon } from "../utils/repoIcon";
import { spinningStatuses } from "./StatusBadge";
import LineageNode, { type LineageNodeData } from "./LineageNode";

const nodeTypes = { lineageNode: LineageNode };

interface ProjectLineageGraphProps {
  project: Project;
  variableSetRefs: string[];
  workspaces: Workspace[];
  flashIds?: Set<string>;
}

function ProjectLineageGraphInner({
  project,
  variableSetRefs,
  workspaces,
  flashIds,
}: ProjectLineageGraphProps) {
  const theme = useMantineTheme();
  const { fitView } = useReactFlow();
  const wrapperRef = useRef<HTMLDivElement>(null);

  const projectName = project.metadata?.name ?? "";
  const projectNamespace = project.metadata?.namespace ?? "";
  const projectPhase = project.status?.phase ?? "";

  const nodeSpacing = 280;
  const nodeWidth = 250;
  const getStartX = (targetX: number, count: number) =>
    targetX - ((count - 1) * nodeSpacing) / 2;

  const computedNodes = useMemo(() => {
    const nodes: Node<LineageNodeData>[] = [];
    const projX = 250;
    const projY = 160;

    nodes.push({
      id: `proj-${projectNamespace}/${projectName}`,
      type: "lineageNode",
      position: { x: projX, y: projY },
      draggable: false,
      width: nodeWidth,
      data: {
        kindLabel: "Project",
        to: `/projects/${projectNamespace}/${projectName}`,
        title: projectName,
        badges: projectPhase
          ? [{ label: projectPhase, color: statusColor[projectPhase] ?? "gray", spinning: spinningStatuses.has(projectPhase) }]
          : [],
        meta: [],
        statusColor: statusColor[projectPhase] ?? "gray",
        borderAll: true,
      },
    });

    const vsStartX = getStartX(projX, variableSetRefs.length);
    variableSetRefs.forEach((vsName, i) => {
      nodes.push({
        id: `vs-${vsName}`,
        type: "lineageNode",
        position: { x: vsStartX + i * nodeSpacing, y: 0 },
        draggable: false,
        width: nodeWidth,
        data: {
          kindLabel: "Variable Set",
          to: `/variable-sets/${projectNamespace}/${vsName}`,
          title: vsName,
          badges: [],
          meta: [],
          borderAll: true,
        },
      });
    });

    const wsStartX = getStartX(projX, workspaces.length);
    workspaces.forEach((ws, i) => {
      const wsNs = ws.metadata?.namespace ?? "";
      const wsName = ws.metadata?.name ?? "";
      const wsPhase = ws.status?.phase ?? "";
      const wsProjectRef = ws.spec?.projectRef?.name ?? "";
      const wsRepoURL = ws.spec?.source?.repoURL ?? "";
      const wsPath = ws.spec?.source?.path ?? "";
      const wsId = ws.metadata?.uid ?? `${wsNs}/${wsName}`;
      const isFlashing = flashIds?.has(wsId);

      nodes.push({
        id: `ws-${wsNs}/${wsName}`,
        type: "lineageNode",
        position: { x: wsStartX + i * nodeSpacing, y: 340 },
        draggable: false,
        width: nodeWidth,
        data: {
          kindLabel: "Workspace",
          to: `/workspaces/${wsNs}/${wsName}`,
          title: wsName,
          badges: wsPhase
            ? [{ label: wsPhase, color: statusColor[wsPhase] ?? "gray", spinning: spinningStatuses.has(wsPhase) }]
            : [],
          meta: [
            {
              icon: <IconBox size={16} color="gray" />,
              label: wsProjectRef || "No Project",
              to: wsProjectRef ? `/projects/${wsNs}/${wsProjectRef}` : undefined,
            },
            {
              icon: repoIcon(wsRepoURL),
              label: wsRepoURL.replace(/^https?:\/\//, ""),
              href: wsRepoURL,
            },
            { icon: <IconFolder size={16} color="gray" />, label: wsPath },
          ],
          statusColor: statusColor[wsPhase] ?? "gray",
          borderAll: true,
          flashStyle: isFlashing
            ? ({ "--flash-color": flashColorVar(wsPhase) } as CSSProperties)
            : undefined,
        },
      });
    });

    return nodes;
  }, [variableSetRefs, workspaces, projectName, projectNamespace, projectPhase, flashIds]);

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
      h={520}
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
