import { type RouteConfig, index, route, layout, prefix } from "@react-router/dev/routes";

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
    ...prefix("admin", [
      route("users", "routes/admin.users.tsx"),
      route("users/:id", "routes/admin.users.$id.tsx"),
      route("groups", "routes/admin.groups.tsx"),
      route("groups/:id", "routes/admin.groups.$id.tsx"),
      route("permissions", "routes/admin.permissions.tsx"),
    ]),
  ]),
] satisfies RouteConfig;
