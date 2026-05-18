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
  // VariableSets attached at the Project level. Edges draw from each of
  // these to the Project node and flow down to every Workspace.
  variableSetRefs: string[];
  workspaces: Workspace[];
  flashIds?: Set<string>;
}

// workspaceVariableSetRefs reads ws.spec.variableSetRef and returns the names
// in order, dropping anything malformed. Kept outside the component so the
// memo dependency stays a stable list of workspaces rather than a derived
// array that would invalidate every render.
function workspaceVariableSetRefs(ws: Workspace): string[] {
  return (ws.spec?.variableSetRef ?? [])
    .map((ref) => ref.name ?? "")
    .filter((name) => name !== "");
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

  // The full set of VariableSet nodes to render is the union of refs
  // attached to the project and refs attached to each visible workspace.
  // We preserve declaration order so the layout stays stable across
  // reconciles: project refs first, then any workspace-only refs in the
  // order they were declared. Sorting alphabetically would shuffle nodes
  // every time someone adds a ref to a single workspace.
  const orderedVariableSets = useMemo(() => {
    const seen = new Set<string>();
    const ordered: string[] = [];
    const add = (name: string) => {
      if (!name || seen.has(name)) return;
      seen.add(name);
      ordered.push(name);
    };
    variableSetRefs.forEach(add);
    workspaces.forEach((ws) => workspaceVariableSetRefs(ws).forEach(add));
    return ordered;
  }, [variableSetRefs, workspaces]);

  // Set of project-attached refs as a Set so edge generation can answer
  // "is this VS attached to the project" in O(1). Built once per change.
  const projectRefSet = useMemo(() => new Set(variableSetRefs), [variableSetRefs]);

  // Per-workspace direct refs, indexed by workspace ID. Recomputed when
  // either the workspace list or any workspace's spec changes.
  const workspaceRefIndex = useMemo(() => {
    const idx = new Map<string, string[]>();
    workspaces.forEach((ws) => idx.set(resourceId(ws), workspaceVariableSetRefs(ws)));
    return idx;
  }, [workspaces]);

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

    const vsStartX = getStartX(projX, orderedVariableSets.length);
    orderedVariableSets.forEach((vsName, i) => {
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
  }, [orderedVariableSets, workspaces, projectName, projectNamespace, projectPhase, flashIds]);

  const edges = useMemo(() => {
    const result: Edge[] = [];

    // VariableSet → Project edges. Only drawn when the VS is referenced at
    // the project level. A VS that only appears via a workspace-level
    // attachment skips this edge so the diagram does not imply a
    // project-wide inheritance that does not exist.
    orderedVariableSets
      .filter((vsName) => projectRefSet.has(vsName))
      .forEach((vsName) => {
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
      const wsId = resourceId(ws);
      const phase = ws.status?.phase ?? "";
      const color = statusColorFor(phase);
      const resolvedColor = theme.colors[color]?.[6] ?? theme.colors.gray[6];

      // Project → Workspace edge. This is the orchestration edge: the
      // project (or its Rollout) decides when this Workspace executes,
      // so we color it by phase to mirror status at a glance.
      result.push({
        id: `e-proj-ws-${wsNs}/${wsName}`,
        source: `proj-${projectNamespace}/${projectName}`,
        target: `ws-${wsNs}/${wsName}`,
        type: "smoothstep",
        animated: true,
        markerEnd: { type: MarkerType.ArrowClosed, color: resolvedColor },
        style: { stroke: resolvedColor, strokeWidth: 2 },
      });

      // VariableSet → Workspace direct edges. A workspace-level
      // variableSetRef layers on top of the inherited project refs, so
      // we draw a separate edge that bypasses the Project node entirely.
      // The dashed style and gray color signal that this is a static
      // input wiring rather than an orchestration relationship, which
      // keeps the project edge above the visually dominant one.
      const directRefs = workspaceRefIndex.get(wsId) ?? [];
      directRefs.forEach((vsName) => {
        result.push({
          id: `e-vs-${vsName}-ws-${wsNs}/${wsName}`,
          source: `vs-${vsName}`,
          target: `ws-${wsNs}/${wsName}`,
          type: "smoothstep",
          animated: false,
          markerEnd: { type: MarkerType.ArrowClosed, color: theme.colors.gray[6] },
          style: {
            stroke: theme.colors.gray[6],
            strokeWidth: 2,
            strokeDasharray: "6 4",
          },
        });
      });
    });

    return result;
  }, [
    workspaces,
    orderedVariableSets,
    projectRefSet,
    workspaceRefIndex,
    theme,
    projectName,
    projectNamespace,
  ]);

  return <FlowGraph nodes={nodes} edges={edges} nodeTypes={nodeTypes} />;
}
