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

interface VariableLineageGraphProps {
  workspace: Workspace;
  project: Project | undefined;
  projectVariableSets: VariableSet[];
  directVariableSets: VariableSet[];
}

function LineageGraphInner({
  workspace,
  project,
  projectVariableSets,
  directVariableSets,
}: VariableLineageGraphProps) {
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

    if (project) {
      const wsX = 250;
      const wsY = 250;
      const projX = 100;
      const projY = 100;

      // Workspace Node (bottom)
      nodes.push({
        id: `ws-${workspace.id}`,
        type: "output",
        position: { x: wsX, y: wsY },
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
                  {workspace.name}
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

      // Project Node (middle left)
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
          border: `1px solid ${theme.colors.grape[6]}`,
          borderRadius: 8,
          padding: 10,
          background: "var(--mantine-color-body)",
          cursor: "pointer",
        },
      });

      // Inherited VariableSet Nodes (top left)
      const pvsStartX = getStartX(projX, projectVariableSets.length);
      projectVariableSets.forEach((vs, index) => {
        nodes.push({
          id: `vs-${vs.id}`,
          type: "input",
          position: { x: pvsStartX + index * nodeSpacing, y: 0 },
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
            border: `1px solid ${theme.colors.blue[6]}`,
            borderRadius: 8,
            padding: 10,
            background: "var(--mantine-color-body)",
            cursor: "pointer",
          },
        });
      });

      // Direct VariableSet Nodes (middle right)
      const dvsStartX = getStartX(400, directVariableSets.length);
      directVariableSets.forEach((vs, index) => {
        nodes.push({
          id: `vs-${vs.id}`,
          type: "input",
          position: { x: dvsStartX + index * nodeSpacing, y: projY },
          sourcePosition: Position.Bottom,
          targetPosition: Position.Top,
          data: {
            label: (
              <Stack gap={4} align="center">
                <Text size="xs" c="dimmed" tt="uppercase" fw={700}>
                  Variable Set
                </Text>
                <Group gap="xs" wrap="nowrap">
                  <ThemeIcon size="sm" variant="light" color="blue">
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
    } else {
      const wsX = 250;
      const wsY = 150;

      nodes.push({
        id: `ws-${workspace.id}`,
        type: "output",
        position: { x: wsX, y: wsY },
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
                  {workspace.name}
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

      const dvsStartX = getStartX(wsX, directVariableSets.length);
      directVariableSets.forEach((vs, index) => {
        nodes.push({
          id: `vs-${vs.id}`,
          type: "input",
          position: { x: dvsStartX + index * nodeSpacing, y: 0 },
          sourcePosition: Position.Bottom,
          targetPosition: Position.Top,
          data: {
            label: (
              <Stack gap={4} align="center">
                <Text size="xs" c="dimmed" tt="uppercase" fw={700}>
                  Variable Set
                </Text>
                <Group gap="xs" wrap="nowrap">
                  <ThemeIcon size="sm" variant="light" color="blue">
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
    }

    return nodes;
  }, [workspace, project, projectVariableSets, directVariableSets, theme]);

  const initialEdges = useMemo(() => {
    const edges: Edge[] = [];

    if (project) {
      // Edge from Project to Workspace
      edges.push({
        id: `e-proj-ws`,
        source: `proj-${project.id}`,
        target: `ws-${workspace.id}`,
        type: "smoothstep",
        animated: true,
        markerEnd: { type: MarkerType.ArrowClosed },
        style: { stroke: theme.colors.gray[6] },
      });

      // Edges from Inherited VariableSets to Project
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
    }

    // Edges from Direct VariableSets to Workspace
    directVariableSets.forEach((vs) => {
      edges.push({
        id: `e-vs-${vs.id}-ws`,
        source: `vs-${vs.id}`,
        target: `ws-${workspace.id}`,
        type: "smoothstep",
        animated: true,
        markerEnd: { type: MarkerType.ArrowClosed },
        style: { stroke: theme.colors.gray[6] },
      });
    });

    return edges;
  }, [workspace, project, projectVariableSets, directVariableSets, theme]);

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
      h={300}
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

export default function VariableLineageGraph(props: VariableLineageGraphProps) {
  return (
    <ReactFlowProvider>
      <LineageGraphInner {...props} />
    </ReactFlowProvider>
  );
}
