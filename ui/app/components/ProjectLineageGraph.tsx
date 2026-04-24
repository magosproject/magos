import { useMemo } from "react";
import { type Edge, type Node, MarkerType } from "@xyflow/react";
import { useMantineTheme } from "@mantine/core";
import { resourceId, resourceName, resourceNamespace } from "../api/resource";
import type { Project, Workspace } from "../api/types";
import { statusColorFor } from "../utils/colors";
import { spinningStatuses } from "./StatusBadge";
import ResourceCard from "./ResourceCard";
import WorkspaceCard from "./WorkspaceCard";
import FlowGraph from "./FlowGraph";
import LineageNode, { type LineageNodeData } from "./LineageNode";

const nodeTypes = { lineageNode: LineageNode };

interface ProjectLineageGraphProps {
  project: Project;
  variableSetRefs: string[];
  workspaces: Workspace[];
  flashIds?: Set<string>;
}

export default function ProjectLineageGraph({
  project,
  variableSetRefs,
  workspaces,
  flashIds,
}: ProjectLineageGraphProps) {
  const theme = useMantineTheme();

  const projectName = resourceName(project);
  const projectNamespace = resourceNamespace(project);
  const projectPhase = project.status?.phase ?? "";

  const nodeSpacing = 280;
  const nodeWidth = 250;
  const getStartX = (targetX: number, count: number) =>
    targetX - ((count - 1) * nodeSpacing) / 2;

  const nodes = useMemo(() => {
    const result: Node<LineageNodeData>[] = [];
    const projX = 250;
    const projY = 160;

    result.push({
      id: `proj-${projectNamespace}/${projectName}`,
      type: "lineageNode",
      position: { x: projX, y: projY },
      draggable: false,
      width: nodeWidth,
      data: {
        kindLabel: "Project",
        content: (
          <ResourceCard
            to={`/projects/${projectNamespace}/${projectName}`}
            title={projectName}
            badges={
              projectPhase
                ? [{ label: projectPhase, color: statusColorFor(projectPhase), spinning: spinningStatuses.has(projectPhase) }]
                : []
            }
            meta={[]}
            statusColor={statusColorFor(projectPhase)}
            borderAll
          />
        ),
      },
    });

    const vsStartX = getStartX(projX, variableSetRefs.length);
    variableSetRefs.forEach((vsName, i) => {
      result.push({
        id: `vs-${vsName}`,
        type: "lineageNode",
        position: { x: vsStartX + i * nodeSpacing, y: 0 },
        draggable: false,
        width: nodeWidth,
        data: {
          kindLabel: "Variable Set",
          content: (
            <ResourceCard
              to={`/variable-sets/${projectNamespace}/${vsName}`}
              title={vsName}
              badges={[]}
              meta={[]}
              borderAll
            />
          ),
        },
      });
    });

    const wsStartX = getStartX(projX, workspaces.length);
    workspaces.forEach((ws, i) => {
      const wsNs = resourceNamespace(ws);
      const wsName = resourceName(ws);
      const wsId = resourceId(ws);

      result.push({
        id: `ws-${wsNs}/${wsName}`,
        type: "lineageNode",
        position: { x: wsStartX + i * nodeSpacing, y: 340 },
        draggable: false,
        width: nodeWidth,
        data: {
          kindLabel: "Workspace",
          content: <WorkspaceCard workspace={ws} borderAll flash={flashIds?.has(wsId)} />,
        },
      });
    });

    return result;
  }, [variableSetRefs, workspaces, projectName, projectNamespace, projectPhase, flashIds]);

  const edges = useMemo(() => {
    const result: Edge[] = [];

    variableSetRefs.forEach((vsName) => {
      result.push({
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
      const wsNs = resourceNamespace(ws);
      const wsName = resourceName(ws);
      const phase = ws.status?.phase ?? "";
      const color = statusColorFor(phase);
      const resolvedColor = theme.colors[color]?.[6] ?? theme.colors.gray[6];

      result.push({
        id: `e-proj-ws-${wsNs}/${wsName}`,
        source: `proj-${projectNamespace}/${projectName}`,
        target: `ws-${wsNs}/${wsName}`,
        type: "smoothstep",
        animated: true,
        markerEnd: { type: MarkerType.ArrowClosed, color: resolvedColor },
        style: { stroke: resolvedColor, strokeWidth: 2 },
      });
    });

    return result;
  }, [workspaces, variableSetRefs, theme, projectName, projectNamespace]);

  return <FlowGraph nodes={nodes} edges={edges} nodeTypes={nodeTypes} />;
}
