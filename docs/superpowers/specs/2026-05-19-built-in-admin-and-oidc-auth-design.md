# Built-in admin account and OIDC authentication

Date: 2026-05-19
Status: Draft
Scope: API server, controllers, UI, Helm chart, Project CR

## Goal

Magos currently has no authentication. The API server accepts every request, the UI never asks who you are, and the workspace controller writes run records to an unauthenticated `/internal/*` endpoint. This spec adds authentication and authorization to all three surfaces, copying Kargo's model verbatim where the underlying stack permits and porting it where it does not.

The end state: every API request carries a verifiable principal, the UI presents a login page that supports both a built-in admin account and OIDC single sign on, and operators can optionally turn on a bundled Dex deployment to bridge identity providers that do not support PKCE.

## Non-goals

- A CLI is not part of this spec. If `magos-cli` lands later it reuses the same login endpoints.
- A separate test framework for the UI is not added. End to end coverage is chainsaw, manual smoke runs `make run`.
- Per resource RBAC fine tuning beyond what Kubernetes `SubjectAccessReview` already provides through the resolved ServiceAccounts.

## Trust model

Three principal types are recognized by the API server:

1. **admin**: a single built-in account whose password is stored as a bcrypt hash and whose sessions are HS256 JWTs signed by the API server. Enabled by default in fresh installs, can be disabled per Helm value. Bypasses authorization checks.
2. **OIDC user**: identity verified by `coreos/go-oidc` against the configured issuer, either an external IdP directly or the optional bundled Dex via the API server's `/dex` reverse proxy. Authorization derived from the user's claims through the ServiceAccount claim mapping.
3. **Kubernetes ServiceAccount**: identity verified by Kubernetes `TokenReview`. Used by the workspace controller's calls into `/internal/*` and by anyone calling the API with a kubeconfig identity. Authorization derived from the SA's own RBAC.

The middleware decides which verification path runs based on the token's `iss` claim. Admin issuer dispatches to local HS256 verification. OIDC issuer dispatches to the multi client verifier. Anything else falls through to `TokenReview`.

Exempt routes, no authentication required: `GET /healthz`, `GET /readyz`, `GET /openapi.json`, `GET /docs`, `POST /api/v1alpha1/auth/login`, `GET /api/v1alpha1/system/public-config`, and any path under `/dex/` when the proxy is enabled.

## Project becomes cluster scoped

Kargo's authorization model relies on per Project namespaces. To match it, Magos's `Project` CR is converted to cluster scope and the Project controller acquires a new responsibility: ensure a namespace exists named after the Project, labeled `magosproject.io/project=true`, owned by the Project via `OwnerReference`. Workspaces, Rollouts, and VariableSets live in their Project's namespace. The OIDC ServiceAccount lookup walks all namespaces carrying that label plus any configured in `api.oidc.globalServiceAccounts.namespaces`.

This is a structural change to the data model, not an auth detail. It breaks every existing installation that has Projects in arbitrary namespaces. Mitigation: a one shot migration tool ships in this PR (`cmd/migrate-projects/main.go`) that creates per Project namespaces, recreates Projects at cluster scope, and moves member resources into the Project namespace. The migration is documented as a required step in the upgrade notes.

## Backend changes

### Package layout

New auth package under `api/internal/api/auth/`, mirroring `kargo/pkg/server`:

```
api/internal/api/auth/
  config.go              # ServerConfig, AdminConfig, envconfig loaders
  middleware.go          # authenticate() and the http.Handler wrapper
  admin_login.go         # POST /api/v1alpha1/auth/login
  public_config.go       # GET /api/v1alpha1/system/public-config
  authz.go               # SubjectAccessReview helpers used by handlers
  dex/
    proxy.go             # copied verbatim from kargo/pkg/server/dex
  oidc/
    config.go            # copied verbatim from kargo/pkg/server/oidc
    verifier.go          # multi client verifier, getKeySet with Dex rewrite
  user/
    info.go              # copied verbatim from kargo/pkg/server/user
  indexer/
    serviceaccount.go    # copied from kargo/pkg/indexer, SA by claim field index
```

Files copied substantially from Kargo retain their Apache 2.0 copyright header. A Magos `NOTICE` entry credits the Kargo project.

### Middleware

Kargo's authentication is a `connect.Interceptor`. Magos uses `http.ServeMux`, so the verification logic ports verbatim into a function with the same signature shape:

```go
func (m *Middleware) authenticate(ctx context.Context, path string, header http.Header) (context.Context, error)
```

A small adapter wraps it as `func(http.Handler) http.Handler`, slotted into `server.go:Router()` after `loggingMiddleware` and before `corsMiddleware`. On verification failure it writes 401 with a JSON error body. On success it stores the resolved `user.Info` in the request context.

The `authenticate` body is the direct port of `kargo/pkg/server/option/auth.go:authenticate`: skip exempt routes, extract bearer token, parse unverified JWT to inspect `iss`, dispatch to admin path or OIDC path or fall through to `verifyKubernetesToken`. The exempt set is a `map[string]struct{}` keyed by HTTP method plus path rather than by RPC procedure.

The middleware also accepts tokens from the `access_token` query parameter, used by the UI for `EventSource` connections that cannot set headers. This matches RFC 6750 Section 2.3. Regular handlers continue to read from the `Authorization` header only.

### Admin login

`POST /api/v1alpha1/auth/login` accepts the password as the bearer value in the `Authorization` header, matching Kargo's REST shape. Returns `{"idToken": "<jwt>"}` on success, 403 on bad password, 400 on missing header. Logic is a direct port of `kargo/pkg/server/admin_login_v1alpha1.go:adminLogin` with `gin.Context` swapped for `http.ResponseWriter`/`*http.Request`.

`AdminConfig` lives in `auth/config.go`:

```go
type AdminConfig struct {
  HashedPassword  string        `envconfig:"ADMIN_ACCOUNT_PASSWORD_HASH" required:"true"`
  TokenIssuer     string        `envconfig:"ADMIN_ACCOUNT_TOKEN_ISSUER" required:"true"`
  TokenAudience   string        `envconfig:"ADMIN_ACCOUNT_TOKEN_AUDIENCE" required:"true"`
  TokenSigningKey []byte        `envconfig:"ADMIN_ACCOUNT_TOKEN_SIGNING_KEY" required:"true"`
  TokenTTL        time.Duration `envconfig:"ADMIN_ACCOUNT_TOKEN_TTL" default:"24h"`
}
```

Env var names match Kargo so operators reusing existing tooling for password hash generation can do so without translation.

### Public config

`GET /api/v1alpha1/system/public-config` returns the shape the UI needs to decide what to render on the login page:

```go
type PublicConfig struct {
  AdminAccountEnabled bool        `json:"adminAccountEnabled"`
  OIDCConfig          *OIDCConfig `json:"oidcConfig,omitempty"`
  SkipAuth            bool        `json:"skipAuth"`
}

type OIDCConfig struct {
  IssuerURL   string   `json:"issuerUrl"`
  ClientID    string   `json:"clientId"`
  CLIClientID string   `json:"cliClientId,omitempty"`
  Scopes      []string `json:"scopes,omitempty"`
}
```

When `api.oidc.dex.enabled=true` the API rewrites the OIDC issuer URL to its own external URL plus `/dex` before responding, so clients discover Dex through the proxy. Same logic as Kargo.

### OIDC verification

`oidc/verifier.go` ports `newMultiClientVerifier` and `getKeySet` from `kargo/pkg/server/option/auth.go`. Multiple `oidc.Verifier` instances (one for web client, one optionally for CLI client) are tried in turn; the first that succeeds wins. `getKeySet` discovers the issuer's JWKS, rewriting any internal Dex URL in the response to the proxy URL when Dex is enabled. The implementation is verbatim from Kargo modulo import paths.

### Authorization at the handler boundary

Each protected handler reads `user.InfoFromContext`. If `IsAdmin`, the handler proceeds. Otherwise the handler calls a helper:

```go
auth.AuthorizeNamespaced(ctx, verb, gvr, namespace, name) error
```

The helper iterates over the SAs in `Info.ServiceAccountsByNamespace`, performs a Kubernetes `SubjectAccessReview` impersonating each one in turn, and returns nil on the first allow. Returns 403 if none allow. This matches how Kargo's project handlers do authorization through the resolved SA set.

For list and watch handlers, the user's namespace set is used to filter informer results before they are written to the SSE stream.

### Claim to ServiceAccount mapping

A controller-runtime indexer over `corev1.ServiceAccount` indexes annotations of the form `rbac.magosproject.io/claim.<name>: <comma separated values>`. The indexer key format is `<name>:<value>` for each value, identical to Kargo's `indexer.FormatClaim`. The middleware queries the indexer with the user's verified claims and collects matching SAs, scoping by:

- Namespaces labeled `magosproject.io/project=true`, plus
- Namespaces in `OIDCConfig.GlobalServiceAccountNamespaces`

The two sources are unioned. The implementation is a direct port of Kargo's `listServiceAccounts` with the label key swapped.

### Internal endpoint auth

The workspace controller calls `PUT /internal/apis/magosproject.io/v1alpha1/workspaces/.../runs/...` to record run state. With auth now enforced everywhere, the controller needs a token. Approach:

- `automountServiceAccountToken: true` on the workspace controller pod (chart change).
- `internal/controller/workspace/run_recorder.go` reads its SA token from `/var/run/secrets/kubernetes.io/serviceaccount/token` at every request and adds `Authorization: Bearer <token>`. Cache the token in memory with a refresh on file change (kubelet rotates these).
- API middleware verifies the token via `TokenReview`. On success, the resolved SA identity is in `user.Info`.
- For `/internal/*` paths an additional check: the SA must appear in `INTERNAL_ALLOWED_SAS` (comma separated `namespace/name`). Default value set by the chart points at the workspace controller SA. Operators can extend it.

### Server wiring

`api/internal/api/server.go` is updated:

- `Server` struct gains `authMiddleware *auth.Middleware`.
- `NewServer` accepts `auth.ServerConfig`, builds the middleware, wires it into the chain.
- `Router()` adds the new routes: `POST /api/v1alpha1/auth/login`, `GET /api/v1alpha1/system/public-config`, and conditionally `GET /dex/` (and subpaths) when Dex proxy is configured.
- Handler signatures stay the same; handlers learn to read `user.Info` from context and call `auth.AuthorizeNamespaced` where appropriate.

`api/cmd/api/main.go` loads `auth.ServerConfig` from env via `envconfig` before calling `NewServer`.

## Helm chart changes

### Values

`charts/magos/values.yaml` gains a verbatim copy of Kargo's `api.adminAccount`, `api.secret`, `api.clusterRoles`, `api.oidc`, and `api.oidc.dex` blocks. The `kargo`/`Kargo` strings in comments become `magos`/`Magos`. All defaults, all `@param` docblocks, all knobs preserved. Notable items:

- `api.adminAccount.enabled: true`
- `api.adminAccount.passwordHash: ""` (chart generates if empty and `api.secret.name` is empty)
- `api.adminAccount.tokenSigningKey: ""` (chart generates if empty)
- `api.adminAccount.tokenTTL: 24h`
- `api.secret.name: ""` (set to use an out of band Secret)
- `api.clusterRoles.{admin,projectCreator,user,viewer}.additionalRules: null`
- `api.oidc.enabled: false`, `issuerURL`, `clientID`, `cliClientID`
- `api.oidc.additionalScopes: [groups]` (hardcoded `openid profile email` is added in code)
- `api.oidc.usernameClaim: email`
- `api.oidc.admins.claims`, `projectCreators.claims`, `users.claims`, `viewers.claims`
- `api.oidc.globalServiceAccounts.namespaces: []`
- `api.oidc.dex.enabled: false`, `image.{repository,tag,pullPolicy,pullSecrets}`, `logLevel: INFO`, `logFormat: TEXT`, `skipApprovalScreen: true`, `connectors: []`, `serviceAccount.{labels,annotations}`, `env: []`, `envFrom: []`
- `api.permissiveCORSPolicyEnabled: false`

### Templates

New files under `charts/magos/templates/api/` (mirroring Kargo's per component subdirectory style):

- `api/secret.yaml`: holds `ADMIN_ACCOUNT_PASSWORD_HASH` and `ADMIN_ACCOUNT_TOKEN_SIGNING_KEY`. Uses Helm `lookup` to preserve generated values across upgrades. Suppressed if `api.secret.name` is set.
- `api/configmap.yaml`: non secret env wiring (`OIDC_*`, `DEX_*`, `INTERNAL_ALLOWED_SAS`, `LOCAL_MODE`).
- `api/cluster-roles.yaml`: four built in roles: `magos-admin`, `magos-project-creator`, `magos-user`, `magos-viewer`. The rules are defined for Magos resources (Workspace, Project, Rollout, VariableSet) following the same role split Kargo uses. Each rendered rule list appends `api.clusterRoles.<name>.additionalRules`.
- `api/cluster-role-bindings.yaml`: bindings rendered for each claim group in `api.oidc.{admins,projectCreators,users,viewers}.claims` against the appropriate cluster role.

New directory `charts/magos/templates/dex-server/` copied from `charts/kargo/templates/dex-server/`:

- `deployment.yaml`, `service.yaml`, `secret.yaml`, `service-account.yaml`, `cert.yaml`

The rename pass swaps `kargo` and `akuity.io` for `magos` and `magosproject.io`, and points image refs at the same Dex upstream.

### Existing template changes

- `templates/api-deployment.yaml`: mount the secret and configmap as env, add `automountServiceAccountToken: true`, conditionally pass Dex env when enabled. Moves into `templates/api/deployment.yaml` to match Kargo's per component subdirectory style; the api service, service account, and cluster role templates move with it.
- `templates/api-clusterrole.yaml`: adds:
  - `create` on `tokenreviews.authentication.k8s.io`
  - `create` on `subjectaccessreviews.authorization.k8s.io`
  - `get`, `list`, `watch` on `serviceaccounts` and `namespaces`
- `templates/controllers/workspace-deployment.yaml`: ensures `automountServiceAccountToken: true` and sets `MAGOS_API_URL` so the `RunRecorder` knows where to call.
- `templates/_helpers.tpl`: bcrypt and random signing key generators following Kargo's helpers.

### NOTES.txt

If `api.adminAccount.enabled=true` and the chart generated a random password, NOTES prints a recovery hint pointing at the Secret. Matches Kargo's wording.

## UI changes

### Route structure

`ui/app/routes.ts`:

```ts
export default [
  route("login", "routes/login.tsx"),
  layout("components/ProtectedRoute.tsx", [
    layout("components/AppShell.tsx", [
      index("routes/home.tsx"),
      route("workspaces", "routes/workspaces.tsx"),
      // ... existing routes unchanged
    ]),
  ]),
] satisfies RouteConfig;
```

`ProtectedRoute` reads from the auth context, calls `useGetPublicConfig` once, and redirects to `/login?redirectTo=<path>` when the user is not logged in and `skipAuth` is false.

### Login page

`ui/app/routes/login.tsx` is a Mantine port of Kargo's `pages/login/login.tsx`. Layout:

- Full viewport `Center`, theme aware background, `Paper` card width 360px
- `Stack` with the `IconHexagon` and `magos` wordmark at top, mirroring `AppShell.tsx`
- `AdminLogin` block: `PasswordInput`, `Button` color `magos.5`, calls `POST /api/v1alpha1/auth/login`
- `Divider` labeled "OR" between blocks when both modes are enabled
- `OIDCLogin` block: `Button` "SSO Login" runs PKCE through `oauth4webapi` (same library Kargo uses)
- "Login is disabled. Please contact your system administrator." fallback when neither is enabled

The page sets `document.title` to "Login | Magos" via a new `useDocumentTitle` hook at `ui/app/hooks/useDocumentTitle.ts`.

### Auth module

New directory `ui/app/auth/`:

- `AuthContext.tsx`: context type `{ isLoggedIn, JWTInfo, login, logout }`
- `AuthProvider.tsx`: localStorage source of truth, keys `magos.auth.token` and `magos.auth.refreshToken`, verbatim port of Kargo's `auth-context-provider.tsx`
- `useAuth.tsx`: `useContext` wrapper
- `jwt-utils.ts`: `extractInfoFromJWT(token)` returning `{ sub, exp, iss, aud, email, groups, ... }`, parse only, no verification (server is authoritative)
- `paths.ts`: `paths.login`, `paths.home` constants
- `safeRedirect.ts`: `isSafeRedirectPath` and `redirectToQueryParam = "redirectTo"`
- `ProtectedRoute.tsx` (in `components/` so it can be referenced from `routes.ts`)

The provider is mounted in `root.tsx` between `MantineProvider` and the `Outlet`.

### API client wiring

`ui/app/api/client.ts` gains an `openapi-fetch` middleware that:

- Reads the token from localStorage and sets `Authorization: Bearer <token>` on every request
- On 401 response: clears the stored token and navigates to `/login`

`ui/app/hooks/useSSE*.ts` are updated to append `?access_token=<token>` to the `EventSource` URL, since `EventSource` cannot set headers. The middleware accepts both header and query parameter token sources.

### Header user menu

`ui/app/components/AppShell.tsx` gains a Mantine `Menu` to the left of the GitHub/Discord icons. Trigger is an `ActionIcon` with `IconUserCircle`. Menu items: the user's email or sub, a "Sign out" item that clears localStorage and navigates to `/login`.

### Generated types

The new endpoints carry swaggo annotations. `make generate-swagger` regenerates the OpenAPI spec, `make generate-ui-types` regenerates `ui/app/api/types.gen.ts`. The login page imports the generated request and response types directly; no hand written types.

## Testing

### Go unit tests (testify/assert, small per file)

- `auth/middleware_test.go`: admin issuer dispatches to local verification, OIDC issuer dispatches to the configured verifier, missing token returns error, exempt routes pass through
- `auth/admin_login_test.go`: wrong password 403, empty password 400, correct password returns signed JWT with expected `iss`, `aud`, `sub`, `exp`, AdminConfig nil returns 403
- `auth/public_config_test.go`: response shape varies correctly with the AdminConfig/OIDCConfig/LocalMode combinations
- `auth/oidc/verifier_test.go`: multi client verifier tries each verifier in turn, returns success when any passes, `getKeySet` rewrites Dex URLs in the JWKS response
- `auth/dex/proxy_test.go`: verbatim copy of `kargo/pkg/server/dex/proxy_test.go`
- `auth/user_test.go`: `ContextWithInfo` and `InfoFromContext` round trip
- `auth/indexer/serviceaccount_test.go`: claim key format, indexer returns SAs matching a given claim/value pair
- `auth/authz_test.go`: `AuthorizeNamespaced` returns nil when any SA in the user's set passes SAR, returns 403 when none do, admin bypass returns nil immediately

### Chainsaw integration tests

New directory `test/chainsaw/tests/auth/`:

- `admin-login/`: install chart with `api.adminAccount.enabled=true`, curl login with wrong password (expect 403), with correct password (expect 200 and JWT), then curl `/apis/.../projects` with and without the JWT
- `public-config/`: install chart with various toggle combinations, assert the JSON shape matches
- `controller-internal-auth/`: verify the workspace controller's SA token works against `/internal/...`, verify an unrelated SA token is rejected
- `dex-discovery/`: install chart with `api.oidc.dex.enabled=true` and a `mockCallback` connector, assert `/dex/.well-known/openid-configuration` returns valid JSON

Existing chainsaw tests that hit the API directly get a Bearer admin token in their fixtures. Tests that go through kubectl are unaffected.

### Manual smoke

`make run` flow: `hack/local-values.yaml` sets `api.adminAccount.enabled=true`, password "admin", a fixed dev signing key, `api.oidc.enabled=false`. UI at `http://localhost:5713` redirects to `/login`, accepts "admin", lands on home.

## Migration

`cmd/migrate-projects/main.go` is a single use tool that converts an existing v0 installation to the new model:

1. List all `Project` CRs across all namespaces.
2. For each one:
   - Create a namespace named after the Project, labeled `magosproject.io/project=true`.
   - Create a cluster scoped Project with the same spec.
   - For every `Workspace`, `Rollout`, and `VariableSet` referencing this Project in its old namespace, recreate it in the new namespace.
   - Delete the old namespaced Project.
3. Print a summary, exit nonzero if any step failed.

Documented at `website/contents/docs/operations/upgrading/v0-to-v1.mdx` as a required step before upgrading the chart. The tool is shipped in the standard `magos/job` image so operators can run it as a `Job` in cluster without distributing a separate binary.

## Documentation

- `website/contents/docs/concepts/projects/index.mdx`: lines 6 and 79 rewritten to describe Project as cluster scoped, owning a namespace. The "does not enforce a namespace boundary" sentence is removed.
- `website/contents/docs/operations/authentication/` (new):
  - `admin-account.mdx`: bcrypt hash generation, the recommendation to disable once OIDC is configured, NOTES output explanation
  - `oidc.mdx`: direct OIDC setup, the claim to role and claim to ServiceAccount mappings, annotation reference for `rbac.magosproject.io/claim.<name>`
  - `dex.mdx`: connectors, the `/dex` proxy, recipes for Google, GitHub, Microsoft, GitLab
- `website/contents/docs/operations/upgrading/v0-to-v1.mdx`: migration tool usage.
- `website/contents/docs/reference/api/`: regenerated automatically from swagger.

## Local dev

`hack/local-values.yaml` gains the same admin defaults the chart uses, with a dev only signing key. The Makefile `make run` target extracts `ADMIN_ACCOUNT_*` from the in cluster Secret the same way it already extracts `MAGOS_LOGS_*` and `MAGOS_POSTGRES_*`, so the host run API binary behaves identically to the in cluster one. The host run workspace controller passes a token from its kubeconfig identity on `RunRecorder` calls, so the API's `TokenReview` path validates against the developer's own Kubernetes identity.

## Risks and rollback

The Project scope change is irreversible without restoring from etcd backups, because cluster scoped and namespace scoped resources are different objects in the apiserver. Operators are advised to take a backup before running the migration. If the migration fails partway, the tool's idempotent behavior on rerun lets it pick up where it stopped.

The auth middleware refusing requests on bad config (e.g. missing signing key) is a hard failure at API startup. The chart's helper generates values if the operator provides none, so a fresh install does not hit this. Upgrades that disable both admin and OIDC log a warning and the API rejects all requests, which is the correct fail closed behavior.

## Open questions for the implementation phase

These do not block the spec but are flagged for `writing-plans`:

1. The admin token is HS256, signed with a key in the api Secret. Should the same key sign a separate refresh token, or do admins re-login on expiry? Kargo: re-login. Same here.
2. The dev mode `LocalMode` env (`LOCAL_MODE=true` in `make run`) bypasses all auth. Kargo gates this behind a build tag. We do not; we rely on the chart never setting it and on `hack/local-values.yaml` being development only. Acceptable risk.
3. The CLI surface is out of scope. If a `magos-cli` lands later it reuses both endpoints unchanged.

## Files touched (approximate)

Backend:
- `api/internal/api/auth/`: new package (~10 files)
- `api/internal/api/server.go`: middleware wiring, two new routes
- `api/internal/api/handlers/*.go`: read user info, call AuthorizeNamespaced
- `api/cmd/api/main.go`: load auth config
- `internal/controller/workspace/run_recorder.go`: bearer token from SA
- `internal/controller/project/*`: namespace ensure
- `types/magosproject/v1alpha1/project_types.go`: scope=Cluster
- `cmd/migrate-projects/main.go`: new
- Generated: deepcopy, clientset, OpenAPI, TS types

Chart:
- `charts/magos/values.yaml`
- `charts/magos/templates/api/{secret,configmap,deployment,service,service-account,cluster-roles,cluster-role-bindings}.yaml`
- `charts/magos/templates/dex-server/{deployment,service,secret,service-account,cert}.yaml`
- `charts/magos/templates/_helpers.tpl`

UI:
- `ui/app/routes.ts`
- `ui/app/routes/login.tsx`
- `ui/app/auth/{AuthContext,AuthProvider,useAuth,jwt-utils,paths,safeRedirect}.tsx`
- `ui/app/components/{ProtectedRoute,AppShell}.tsx`
- `ui/app/hooks/{useDocumentTitle,useSSEItem,useSSEList,useSSEStream,useSSEFiltered}.ts`
- `ui/app/api/client.ts`
- `ui/app/api/types.gen.ts` (generated)
- `ui/package.json` (add `oauth4webapi`)

Tests:
- `api/internal/api/auth/*_test.go`
- `test/chainsaw/tests/auth/{admin-login,public-config,controller-internal-auth,dex-discovery}/`

Docs:
- `website/contents/docs/concepts/projects/index.mdx`
- `website/contents/docs/operations/authentication/{admin-account,oidc,dex}.mdx`
- `website/contents/docs/operations/upgrading/v0-to-v1.mdx`
