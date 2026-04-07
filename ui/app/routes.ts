import { type RouteConfig, index, route, layout } from "@react-router/dev/routes";

export default [
  layout("components/AppShell.tsx", [
    index("routes/home.tsx"),
    route("workspaces", "routes/workspaces.tsx"),
    route("workspaces/:namespace/:name", "routes/workspace.tsx"),
    route("projects", "routes/projects.tsx"),
    route("projects/:namespace/:name", "routes/project.tsx"),
    route("rollouts", "routes/rollouts.tsx"),
    route("rollouts/:namespace/:name", "routes/rollout.tsx"),
    route("variable-sets", "routes/variable-sets.tsx"),
    route("variable-sets/:namespace/:name", "routes/variable-set.tsx"),
    route("settings", "routes/settings.tsx"),
  ]),
] satisfies RouteConfig;
