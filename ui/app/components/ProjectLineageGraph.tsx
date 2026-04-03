import React, { useEffect, useMemo, useRef, useCallback } from "react";
import {
  ReactFlow,
  Controls,
  Background,
  MarkerType,
  Position,
  type Edge,
  type Node,
  useNodesState,
  useEdgesState,
  ReactFlowProvider,
  useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Box, Text, Group, ThemeIcon, Stack, useMantineTheme } from "@mantine/core";
import { IconBox, IconFolder, IconBraces } from "@tabler/icons-react";
import { useNavigate } from "react-router";
import { type Workspace } from "../mock-data/workspaces";
import { type Project } from "../mock-data/projects";
import { type VariableSet } from "../mock-data/variable-sets";

interface ProjectLineageGraphProps {
  project: Project;
  projectVariableSets: VariableSet[];
  projectWorkspaces: Workspace[];
}

function ProjectLineageGraphInner({
  project,
  projectVariableSets,
  projectWorkspaces,
}: ProjectLineageGraphProps) {
  const theme = useMantineTheme();
  const { fitView } = useReactFlow();
  const wrapperRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  const onNodeClick = useCallback(
    (_event: React.MouseEvent, node: Node) => {
      if (node.id.startsWith("ws-")) {
        navigate(`/workspaces/${node.id.replace("ws-", "")}`);
      } else if (node.id.startsWith("proj-")) {
        navigate(`/projects/${node.id.replace("proj-", "")}`);
      } else if (node.id.startsWith("vs-")) {
        navigate(`/variable-sets/${node.id.replace("vs-", "")}`);
      }
    },
    [navigate]
  );

  const initialNodes = useMemo(() => {
    const nodes: Node[] = [];

    const nodeSpacing = 200;
    const getStartX = (targetX: number, count: number) => targetX - ((count - 1) * nodeSpacing) / 2;

    const projX = 250;
    const projY = 150;

    // Project Node (middle)
    nodes.push({
      id: `proj-${project.id}`,
      type: "default",
      position: { x: projX, y: projY },
      sourcePosition: Position.Bottom,
      targetPosition: Position.Top,
      data: {
        label: (
          <Stack gap={4} align="center">
            <Text size="xs" c="dimmed" tt="uppercase" fw={700}>
              Project
            </Text>
            <Group gap="xs" wrap="nowrap">
              <ThemeIcon size="sm" variant="light" color="blue">
                <IconBox size={14} />
              </ThemeIcon>
              <Text size="sm" fw={600}>
                {project.name}
              </Text>
            </Group>
          </Stack>
        ),
      },
      style: {
        border: `1px solid ${theme.colors.blue[6]}`,
        borderRadius: 8,
        padding: 10,
        background: "var(--mantine-color-body)",
        cursor: "pointer",
      },
    });

    // VariableSet Nodes (top)
    const vsStartX = getStartX(projX, projectVariableSets.length);

    projectVariableSets.forEach((vs, index) => {
      nodes.push({
        id: `vs-${vs.id}`,
        type: "input",
        position: { x: vsStartX + index * nodeSpacing, y: 0 },
        sourcePosition: Position.Bottom,
        targetPosition: Position.Top,
        data: {
          label: (
            <Stack gap={4} align="center">
              <Text size="xs" c="dimmed" tt="uppercase" fw={700}>
                Variable Set
              </Text>
              <Group gap="xs" wrap="nowrap">
                <ThemeIcon size="sm" variant="light" color="grape">
                  <IconBraces size={14} />
                </ThemeIcon>
                <Text size="sm" fw={600}>
                  {vs.name}
                </Text>
              </Group>
            </Stack>
          ),
        },
        style: {
          border: `1px solid ${theme.colors.grape[6]}`,
          borderRadius: 8,
          padding: 10,
          background: "var(--mantine-color-body)",
          cursor: "pointer",
        },
      });
    });

    // Workspace Nodes (bottom)
    const wsStartX = getStartX(projX, projectWorkspaces.length);

    projectWorkspaces.forEach((ws, index) => {
      nodes.push({
        id: `ws-${ws.id}`,
        type: "output",
        position: { x: wsStartX + index * nodeSpacing, y: 300 },
        sourcePosition: Position.Bottom,
        targetPosition: Position.Top,
        data: {
          label: (
            <Stack gap={4} align="center">
              <Text size="xs" c="dimmed" tt="uppercase" fw={700}>
                Workspace
              </Text>
              <Group gap="xs" wrap="nowrap">
                <ThemeIcon size="sm" variant="light" color="magos">
                  <IconFolder size={14} />
                </ThemeIcon>
                <Text size="sm" fw={600}>
                  {ws.name}
                </Text>
              </Group>
            </Stack>
          ),
        },
        style: {
          border: `1px solid ${theme.colors.magos[6]}`,
          borderRadius: 8,
          padding: 10,
          background: "var(--mantine-color-body)",
          cursor: "pointer",
        },
      });
    });

    return nodes;
  }, [project, projectVariableSets, projectWorkspaces, theme]);

  const initialEdges = useMemo(() => {
    const edges: Edge[] = [];

    // Edges from VariableSets to Project
    projectVariableSets.forEach((vs) => {
      edges.push({
        id: `e-vs-${vs.id}-proj`,
        source: `vs-${vs.id}`,
        target: `proj-${project.id}`,
        type: "smoothstep",
        animated: true,
        markerEnd: { type: MarkerType.ArrowClosed },
        style: { stroke: theme.colors.gray[6] },
      });
    });

    // Edges from Project to Workspaces
    projectWorkspaces.forEach((ws) => {
      edges.push({
        id: `e-proj-ws-${ws.id}`,
        source: `proj-${project.id}`,
        target: `ws-${ws.id}`,
        type: "smoothstep",
        animated: true,
        markerEnd: { type: MarkerType.ArrowClosed },
        style: { stroke: theme.colors.gray[6] },
      });
    });

    return edges;
  }, [project, projectVariableSets, projectWorkspaces, theme]);

  const [nodes, , onNodesChange] = useNodesState(initialNodes);
  const [edges, , onEdgesChange] = useEdgesState(initialEdges);

  useEffect(() => {
    if (!wrapperRef.current) return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.width > 0 && entry.contentRect.height > 0) {
          window.requestAnimationFrame(() => {
            fitView({ padding: 0.2, minZoom: 0.1, maxZoom: 1.5, duration: 800 });
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
        fitView
      >
        <Controls />
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
