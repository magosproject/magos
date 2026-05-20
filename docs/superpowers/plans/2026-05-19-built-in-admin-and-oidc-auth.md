# Built-in admin account and OIDC authentication: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add authentication and authorization to Magos: a built-in admin account, optional OIDC single sign on (with bundled Dex), Kubernetes ServiceAccount-based authz, and a UI login page. Convert `Project` to cluster-scope so the Project controller can own a per-Project namespace, matching Kargo's model.

**Out of scope** (per user direction during planning):
- A migration tool for existing v0 installations.
- New chainsaw tests for auth flows. Existing chainsaw fixtures are updated where needed so they keep passing after the Project scope change.
- New unit tests for any of the added code. The user will write these later. The executor verifies code by `go build ./...`, `helm lint`, `helm template`, and `npm run typecheck`, not by running tests.

**Architecture:** Copy Kargo's `pkg/server` auth packages (`dex`, `user`, `oidc`, `indexer`) verbatim into `api/internal/api/auth/` with import renames. Port the connect-rpc handlers and interceptor to stdlib `net/http` adapter shells around the same verification logic. Add a chart subtree for Dex matching Kargo's `dex-server/` templates. Build the UI login page in Mantine with the same component shape Kargo uses in antd.

**Tech Stack:** Go 1.22+, `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, `github.com/coreos/go-oidc/v3`, `github.com/kelseyhightower/envconfig`, controller-runtime, Helm, React Router v7, Mantine, openapi-fetch, `oauth4webapi`.

**Spec:** `docs/superpowers/specs/2026-05-19-built-in-admin-and-oidc-auth-design.md`

**Kargo source root for ports:** `/Users/bschaatsbergen/github/akuity/kargo`

---

## Working agreements for this plan

1. **Git is the user's job.** Commit steps in this plan mark logical units only. The executor (you or a subagent) does NOT run `git add`, `git commit`, `git push`, or any other git command. Stop at each "Commit" step, summarize what changed, and the user will commit and tell you to continue. The same applies to `gh` commands.
2. **No tests are written or run by the executor.** Any plan step that says "Write failing test", "Failing tests", "Write test", "Test:", "Run go test", "Expected: PASS", or that creates a `*_test.go` file is SKIPPED. The user is adding tests themselves to learn the codebase. Replace the verification step with the appropriate build or lint command:
   - Go code: `go build ./...` (from the right module root, `api/` for the api module).
   - Helm: `helm lint charts/magos` and a `helm template` render check.
   - UI: `cd ui && npm run typecheck`.
   The executor still reports the build result before moving on; just no tests.
3. **Test style** (informational, for when the user adds tests later): Go unit tests use `testify/assert`, stay small (one behavior per test), no large table tests unless data is the point of the test. Chainsaw covers integration. UI has no test framework, smoke is manual.
4. **Writing style in docs.** No em or en dashes. Use plain platform-team vocabulary (compliance, owner, enforcement, namespace, role binding).
5. **Verbatim copies.** When this plan says "copy from Kargo", the executor runs `cp` on the source file, then applies the rename substitutions listed in that task. The copy keeps Kargo's Apache 2.0 copyright header intact.
6. **Rename substitutions for copied Go files** (apply in this order, case-sensitive):
   - `github.com/akuity/kargo` -> `github.com/magosproject/magos/api` (or `/magos` for root module, see per-task)
   - `kargo.akuity.io` -> `magosproject.io`
   - `kargoapi` -> `magosapi`
   - `Kargo` -> `Magos` (in comments and identifiers where it's the project name, not the type)
   - `kargo` -> `magos` (in chart values, image names, env, paths)
   The package name (`package server` etc) keeps Kargo's name only if the file is dropped into a same-named directory; otherwise rename to match the destination.
7. **Generated code.** After any change to `types/`, `api/internal/api/handlers/*.go` swaggo comments, or new Go indexer registrations, run `make manifests generate generate-swagger generate-ui-types` before declaring the task done.

---

## File structure

### New files

```
api/internal/api/auth/
  config.go
  middleware.go
  admin_login.go
  public_config.go
  authz.go
  dex/
    proxy.go
  oidc/
    config.go
    verifier.go
  user/
    info.go
  indexer/
    serviceaccount.go

charts/magos/templates/api/secret.yaml
charts/magos/templates/api/configmap.yaml
charts/magos/templates/api/cluster-roles.yaml
charts/magos/templates/api/cluster-role-bindings.yaml
charts/magos/templates/api/deployment.yaml    # moved from templates/api-deployment.yaml
charts/magos/templates/api/service.yaml       # moved from templates/api-service.yaml
charts/magos/templates/api/service-account.yaml  # moved
charts/magos/templates/api/cluster-role.yaml  # moved from templates/api-clusterrole.yaml
charts/magos/templates/api/cluster-role-binding.yaml  # moved

charts/magos/templates/dex-server/deployment.yaml
charts/magos/templates/dex-server/service.yaml
charts/magos/templates/dex-server/secret.yaml
charts/magos/templates/dex-server/service-account.yaml
charts/magos/templates/dex-server/cert.yaml

ui/app/auth/AuthContext.tsx
ui/app/auth/AuthProvider.tsx
ui/app/auth/useAuth.tsx
ui/app/auth/jwt-utils.ts
ui/app/auth/paths.ts
ui/app/auth/safeRedirect.ts
ui/app/components/ProtectedRoute.tsx
ui/app/routes/login.tsx
ui/app/hooks/useDocumentTitle.ts

website/contents/docs/operations/authentication/admin-account.mdx
website/contents/docs/operations/authentication/oidc.mdx
website/contents/docs/operations/authentication/dex.mdx
```

### Modified files

```
types/magosproject/v1alpha1/project_types.go
types/magosproject/v1alpha1/zz_generated.deepcopy.go        # regen
internal/controller/project/project_controller.go
internal/controller/workspace/run_recorder.go
api/internal/api/server.go
api/internal/api/handlers/project_handler.go
api/internal/api/handlers/workspace_handler.go
api/internal/api/handlers/rollout_handler.go
api/internal/api/handlers/variableset_handler.go
api/cmd/api/main.go
api/internal/generated/...                                   # regen
api/internal/api/docs/swagger.json                           # regen

charts/magos/values.yaml
charts/magos/templates/_helpers.tpl
charts/magos/templates/controllers/workspace-deployment.yaml  # exact path TBD by current naming
charts/magos/Chart.yaml                                       # bump version

ui/app/routes.ts
ui/app/root.tsx
ui/app/components/AppShell.tsx
ui/app/api/client.ts
ui/app/api/types.gen.ts                                       # regen
ui/app/hooks/useSSEItem.ts
ui/app/hooks/useSSEList.ts
ui/app/hooks/useSSEStream.ts
ui/app/hooks/useSSEFiltered.ts
ui/package.json

hack/local-values.yaml
Makefile

website/contents/docs/concepts/projects/index.mdx
NOTICE                                                         # new or amend
```

---

## Phase 1: Project becomes cluster scoped

This phase produces a working, testable change before any auth work begins: `kubectl get project` returns cluster-scoped Projects and the Project controller owns a per-Project namespace.

### Task 1.1: Flip Project to cluster scope and regenerate

**Files:**
- Modify: `types/magosproject/v1alpha1/project_types.go`
- Regen: `types/magosproject/v1alpha1/zz_generated.deepcopy.go`
- Regen: `charts/magos/resources/crds/magosproject.io_projects.yaml`
- Regen: `api/internal/generated/clientset/versioned/typed/magosproject/v1alpha1/project.go`

- [ ] **Step 1:** Open `types/magosproject/v1alpha1/project_types.go`. Locate the `// +kubebuilder:object:root=true` marker above the `Project` struct. Add `// +kubebuilder:resource:scope=Cluster` on the line below it. Remove `Namespaced` from the listed scope markers if present.

- [ ] **Step 2:** Run `make manifests generate`. Expected output: regenerated CRD has `scope: Cluster`, regenerated clientset uses cluster-scoped lister and getter for `ProjectInterface`.

- [ ] **Step 3:** Run `grep -rn "Projects(.*Namespace\|Projects(\"" api/ internal/ cmd/`. Expected: zero hits referencing namespace-scoped Project API (the clientset signature changed; the generator already updated callers, but verify any hand-written code is fixed).

- [ ] **Step 4:** Commit. User runs:
  ```
  git add types charts/magos/resources/crds api/internal/generated
  git commit -m "feat: convert Project to cluster scope"
  ```

### Task 1.2: Project controller ensures namespace

**Files:**
- Modify: `internal/controller/project/project_controller.go`
- Modify: `internal/controller/project/project_controller_test.go` (or new if doesn't exist)

- [ ] **Step 1: Write failing unit test.** Add to `project_controller_test.go`:
  ```go
  func TestEnsureNamespace_CreatesLabeledNamespace(t *testing.T) {
      scheme := runtime.NewScheme()
      assert.NoError(t, magosv1alpha1.AddToScheme(scheme))
      assert.NoError(t, corev1.AddToScheme(scheme))
      proj := &magosv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "alpha"}}
      cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(proj).Build()
      r := &ProjectReconciler{Client: cl, Scheme: scheme}
      assert.NoError(t, r.ensureNamespace(context.Background(), proj))
      ns := &corev1.Namespace{}
      assert.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "alpha"}, ns))
      assert.Equal(t, "true", ns.Labels["magosproject.io/project"])
      assert.Len(t, ns.OwnerReferences, 1)
      assert.Equal(t, "alpha", ns.OwnerReferences[0].Name)
      assert.Equal(t, "Project", ns.OwnerReferences[0].Kind)
  }
  ```

- [ ] **Step 2: Run.** `go test ./internal/controller/project -run TestEnsureNamespace_CreatesLabeledNamespace -v`. Expected: FAIL (`ensureNamespace` undefined).

- [ ] **Step 3: Implement.** Add to `project_controller.go`:
  ```go
  func (r *ProjectReconciler) ensureNamespace(ctx context.Context, proj *magosv1alpha1.Project) error {
      ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: proj.Name}}
      _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
          if ns.Labels == nil {
              ns.Labels = map[string]string{}
          }
          ns.Labels["magosproject.io/project"] = "true"
          return controllerutil.SetControllerReference(proj, ns, r.Scheme)
      })
      return err
  }
  ```
  And invoke it from `Reconcile` before any other logic, returning the error to controller-runtime if it fails (`ctrl.Result{}, err`).

- [ ] **Step 4: Run.** Same command. Expected: PASS.

- [ ] **Step 5: Add idempotency test.**
  ```go
  func TestEnsureNamespace_Idempotent(t *testing.T) {
      // same setup as above, call ensureNamespace twice, assert no error and one namespace exists
  }
  ```
  Run: PASS.

- [ ] **Step 6: Commit.** User runs:
  ```
  git add internal/controller/project
  git commit -m "feat(project): ensure a namespace exists for each Project"
  ```

### Task 1.3: Update existing chainsaw fixtures for cluster-scoped Project

**Files:**
- Modify: every `test/chainsaw/tests/**/project*.yaml` that creates a Project (find with `grep -rln "kind: Project$" test/chainsaw/`)
- Modify: `test/chainsaw/tests/**/workspace*.yaml` and similar that set `projectRef`

- [ ] **Step 1: Audit.** Run `grep -rln "kind: Project" test/chainsaw/`. Note every file that creates a Project, and every adjacent file that places resources in a namespace assumed to host the Project.

- [ ] **Step 2: Edit each Project fixture.** Remove `metadata.namespace:` from Project manifests. Add a sibling `Namespace` manifest matching the Project name only if a test needs to create the namespace before the controller exists (envtest in chainsaw tests typically runs controllers, so this is usually unnecessary).

- [ ] **Step 3: Edit Workspace/Rollout/VariableSet fixtures.** For every resource that references a Project via `spec.projectRef.name: foo`, ensure the resource itself has `metadata.namespace: foo` (matching Kargo's expectation that members live in the Project's namespace).

- [ ] **Step 4: Run a smoke chainsaw test.** `./bin/chainsaw test test/chainsaw/tests/workspace/plan-apply-end-to-end`. Expected: PASS.

- [ ] **Step 5: Run all chainsaw tests.** `make test-chainsaw`. Expected: PASS.

- [ ] **Step 6: Commit.** User runs:
  ```
  git add test/chainsaw
  git commit -m "test(chainsaw): adapt fixtures to cluster-scoped Project"
  ```

### Task 1.4: Update concept doc

**Files:**
- Modify: `website/contents/docs/concepts/projects/index.mdx`

- [ ] **Step 1:** Open the file. Rewrite line 6 from "A `Project` is a namespaced Kubernetes resource..." to "A `Project` is a cluster-scoped Kubernetes resource that owns a namespace of the same name and defines a logical boundary for a set of related Workspaces."
- [ ] **Step 2:** Delete the bullet at line 79 starting with "It does not enforce a namespace boundary". Replace it with: "Workspaces, Rollouts, and VariableSets must live in the Project's namespace. The Project controller creates this namespace on first reconcile and labels it `magosproject.io/project=true`."
- [ ] **Step 3:** Update the YAML example around line 26 to remove `namespace: default` from the Project, and adjust the Workspace example around line 40 so `namespace:` matches the Project name.
- [ ] **Step 4:** Run `npm run build` in `~/github/magos/website` (or the project's preview build command) to confirm the MDX still parses. Expected: success.
- [ ] **Step 5: Commit.** User runs:
  ```
  git add website/contents/docs/concepts/projects/index.mdx
  git commit -m "docs(projects): describe Project as cluster scoped"
  ```

---

## Phase 2: Auth backend (no enforcement yet)

This phase adds all the auth code paths but the middleware does not yet wrap the router. Endpoints exist, types are generated, unit tests pass. The chart and UI still see the old unauthenticated API. This lets us land the package, get reviewers used to it, and then flip the switch in Phase 3.

### Task 2.1: Copy `user` package from Kargo

**Files:**
- Source: `/Users/bschaatsbergen/github/akuity/kargo/pkg/server/user/info.go`
- Create: `api/internal/api/auth/user/info.go`
- Create: `api/internal/api/auth/user/info_test.go`

- [ ] **Step 1:** Run `cp /Users/bschaatsbergen/github/akuity/kargo/pkg/server/user/info.go api/internal/api/auth/user/info.go`.
- [ ] **Step 2:** Edit the new file: change the package import paths to use Magos's module if any (this file has none, just stdlib types).
- [ ] **Step 3: Write test** `info_test.go`:
  ```go
  func TestContextWithInfo_RoundTrip(t *testing.T) {
      info := user.Info{IsAdmin: true, Username: "admin"}
      ctx := user.ContextWithInfo(context.Background(), info)
      got, ok := user.InfoFromContext(ctx)
      assert.True(t, ok)
      assert.Equal(t, info, got)
  }
  ```
- [ ] **Step 4:** Run `go test ./api/internal/api/auth/user -v`. Expected: PASS.
- [ ] **Step 5: Commit.** User runs:
  ```
  git add api/internal/api/auth/user
  git commit -m "feat(api): port user.Info package from Kargo"
  ```

### Task 2.2: Copy `dex` package from Kargo

**Files:**
- Source: `/Users/bschaatsbergen/github/akuity/kargo/pkg/server/dex/proxy.go`
- Source: `/Users/bschaatsbergen/github/akuity/kargo/pkg/server/dex/proxy_test.go`
- Create: `api/internal/api/auth/dex/proxy.go`
- Create: `api/internal/api/auth/dex/proxy_test.go`

- [ ] **Step 1:** `cp` both files into `api/internal/api/auth/dex/`.
- [ ] **Step 2:** No import rewrites needed; this file uses only stdlib and `cleanhttp`/`envconfig`. Confirm with `grep "akuity\|kargo" api/internal/api/auth/dex/`.
- [ ] **Step 3:** Run `go test ./api/internal/api/auth/dex -v`. Expected: PASS.
- [ ] **Step 4: Commit.** User runs:
  ```
  git add api/internal/api/auth/dex
  git commit -m "feat(api): port dex reverse proxy from Kargo"
  ```

### Task 2.3: Copy `oidc/config` package from Kargo

**Files:**
- Source: `/Users/bschaatsbergen/github/akuity/kargo/pkg/server/oidc/config.go`
- Create: `api/internal/api/auth/oidc/config.go`

- [ ] **Step 1:** `cp` the file. Apply rename substitution: any reference to `kargoapi` or Kargo identifiers is unlikely here; verify with `grep`.
- [ ] **Step 2:** Run `go build ./api/internal/api/auth/oidc`. Expected: success.
- [ ] **Step 3: Commit.** User runs:
  ```
  git add api/internal/api/auth/oidc
  git commit -m "feat(api): port oidc config from Kargo"
  ```

### Task 2.4: Port multi-client verifier and getKeySet

**Files:**
- Source: `/Users/bschaatsbergen/github/akuity/kargo/pkg/server/option/auth.go` (functions `newMultiClientVerifier`, `getKeySet`, types `goOIDCIDTokenVerifyFn`, `claims`, `oidcExtractClaims`)
- Create: `api/internal/api/auth/oidc/verifier.go`
- Create: `api/internal/api/auth/oidc/verifier_test.go`

- [ ] **Step 1:** Open Kargo's `auth.go` at the function `newMultiClientVerifier`. Copy that function and `getKeySet`, plus the type `goOIDCIDTokenVerifyFn`, `claims`, and helper `oidcExtractClaims` into `verifier.go`. Imports: stdlib, `coreos/go-oidc/v3/oidc`, `hashicorp/go-cleanhttp`, `magos` config types.
- [ ] **Step 2:** Replace `cfg.OIDCConfig`, `cfg.DexProxyConfig` references with `Config` and `DexProxyConfig` parameters (the function should take what it needs, not the whole server config; refactor here so the package boundary is clean).
- [ ] **Step 3: Write a unit test.** Use `httptest.NewServer` to fake a JWKS endpoint and discovery endpoint, then assert that `getKeySet` rewrites a Dex internal URL to the proxy URL. (Kargo doesn't have this test today; this is a small addition.)
  ```go
  func TestGetKeySet_RewritesDexURLs(t *testing.T) {
      // ... see implementation
  }
  ```
- [ ] **Step 4:** Run `go test ./api/internal/api/auth/oidc -v`. Expected: PASS.
- [ ] **Step 5: Commit.** User runs:
  ```
  git add api/internal/api/auth/oidc
  git commit -m "feat(api): port OIDC multi-client verifier from Kargo"
  ```

### Task 2.5: Copy `indexer` ServiceAccount-by-claim index

**Files:**
- Source: `/Users/bschaatsbergen/github/akuity/kargo/pkg/indexer/indexer.go` (only the `ServiceAccountsByOIDCClaimsField` constant, `FormatClaim` helper, and the `indexServiceAccountsByOIDCClaims` function)
- Create: `api/internal/api/auth/indexer/serviceaccount.go`
- Create: `api/internal/api/auth/indexer/serviceaccount_test.go`

- [ ] **Step 1:** Extract those three symbols from Kargo's `indexer.go` into a new file `serviceaccount.go`. Imports: `corev1`, `client.IndexerFunc`, `strings`.
- [ ] **Step 2:** The annotation key Kargo uses is `rbac.kargo.akuity.io/claim.<claim_name>`. Change it to `rbac.magosproject.io/claim.<claim_name>` in the index function.
- [ ] **Step 3: Write test:**
  ```go
  func TestIndexServiceAccountsByOIDCClaims_ExtractsAnnotations(t *testing.T) {
      sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
          Name: "platform-admin", Namespace: "alpha",
          Annotations: map[string]string{
              "rbac.magosproject.io/claim.groups": "platform-team,sre",
              "rbac.magosproject.io/claim.email":  "owner@example.com",
          },
      }}
      keys := IndexServiceAccountsByOIDCClaims(sa)
      assert.ElementsMatch(t, []string{
          "groups:platform-team", "groups:sre", "email:owner@example.com",
      }, keys)
  }
  ```
- [ ] **Step 4:** Run `go test ./api/internal/api/auth/indexer -v`. Expected: PASS.
- [ ] **Step 5: Commit.** User runs:
  ```
  git add api/internal/api/auth/indexer
  git commit -m "feat(api): port ServiceAccount OIDC claim indexer"
  ```

### Task 2.6: Auth config types

**Files:**
- Source for shape: `/Users/bschaatsbergen/github/akuity/kargo/pkg/server/config/config.go`
- Create: `api/internal/api/auth/config.go`
- Create: `api/internal/api/auth/config_test.go`

- [ ] **Step 1: Implement `config.go`:**
  ```go
  package auth

  import (
      "time"

      "github.com/kelseyhightower/envconfig"

      "github.com/magosproject/magos/api/internal/api/auth/dex"
      "github.com/magosproject/magos/api/internal/api/auth/oidc"
  )

  type ServerConfig struct {
      LocalMode      bool
      AdminConfig    *AdminConfig
      OIDCConfig     *oidc.Config
      DexProxyConfig *dex.ProxyConfig
      InternalAllowedSAs []string `envconfig:"INTERNAL_ALLOWED_SAS"`
  }

  type AdminConfig struct {
      HashedPassword  string        `envconfig:"ADMIN_ACCOUNT_PASSWORD_HASH" required:"true"`
      TokenIssuer     string        `envconfig:"ADMIN_ACCOUNT_TOKEN_ISSUER" required:"true"`
      TokenAudience   string        `envconfig:"ADMIN_ACCOUNT_TOKEN_AUDIENCE" required:"true"`
      TokenSigningKey []byte        `envconfig:"ADMIN_ACCOUNT_TOKEN_SIGNING_KEY" required:"true"`
      TokenTTL        time.Duration `envconfig:"ADMIN_ACCOUNT_TOKEN_TTL" default:"24h"`
  }

  func ServerConfigFromEnv() ServerConfig {
      var cfg ServerConfig
      envconfig.MustProcess("", &cfg)
      cfg.LocalMode = getBoolEnv("LOCAL_MODE", false)
      if getBoolEnv("ADMIN_ACCOUNT_ENABLED", false) {
          var a AdminConfig
          envconfig.MustProcess("", &a)
          cfg.AdminConfig = &a
      }
      if getBoolEnv("OIDC_ENABLED", false) {
          c := oidc.ConfigFromEnv()
          cfg.OIDCConfig = &c
      }
      if getBoolEnv("DEX_ENABLED", false) {
          c := dex.ProxyConfigFromEnv()
          cfg.DexProxyConfig = &c
      }
      return cfg
  }
  ```
  Add a small `getBoolEnv` helper in the same file.

- [ ] **Step 2: Test:**
  ```go
  func TestServerConfigFromEnv_AdminDisabled(t *testing.T) {
      t.Setenv("ADMIN_ACCOUNT_ENABLED", "false")
      cfg := auth.ServerConfigFromEnv()
      assert.Nil(t, cfg.AdminConfig)
  }
  func TestServerConfigFromEnv_AdminEnabled(t *testing.T) {
      t.Setenv("ADMIN_ACCOUNT_ENABLED", "true")
      t.Setenv("ADMIN_ACCOUNT_PASSWORD_HASH", "$2a$10$abcdef...")
      t.Setenv("ADMIN_ACCOUNT_TOKEN_ISSUER", "magos")
      t.Setenv("ADMIN_ACCOUNT_TOKEN_AUDIENCE", "magos")
      t.Setenv("ADMIN_ACCOUNT_TOKEN_SIGNING_KEY", "dev-key")
      cfg := auth.ServerConfigFromEnv()
      assert.NotNil(t, cfg.AdminConfig)
      assert.Equal(t, 24*time.Hour, cfg.AdminConfig.TokenTTL)
  }
  ```

- [ ] **Step 3:** Run `go test ./api/internal/api/auth -v -run TestServerConfigFromEnv`. Expected: PASS.

- [ ] **Step 4: Commit.** User runs:
  ```
  git add api/internal/api/auth/config.go api/internal/api/auth/config_test.go
  git commit -m "feat(api): auth ServerConfig"
  ```

### Task 2.7: Admin login handler

**Files:**
- Source: `/Users/bschaatsbergen/github/akuity/kargo/pkg/server/admin_login_v1alpha1.go` (the `adminLogin` gin handler, lines 85-153)
- Create: `api/internal/api/auth/admin_login.go`
- Create: `api/internal/api/auth/admin_login_test.go`

- [ ] **Step 1: Write the failing test first.** `admin_login_test.go`:
  ```go
  func TestAdminLogin_MissingAuthHeader_400(t *testing.T) {
      h := &Handler{Cfg: ServerConfig{AdminConfig: &AdminConfig{
          HashedPassword:  mustBcrypt("hunter2"),
          TokenIssuer:     "magos",
          TokenAudience:   "magos",
          TokenSigningKey: []byte("dev"),
          TokenTTL:        time.Hour,
      }}}
      req := httptest.NewRequest(http.MethodPost, "/api/v1alpha1/auth/login", nil)
      rec := httptest.NewRecorder()
      h.AdminLogin(rec, req)
      assert.Equal(t, http.StatusBadRequest, rec.Code)
  }

  func TestAdminLogin_BadPassword_403(t *testing.T) {
      h := &Handler{Cfg: ServerConfig{AdminConfig: &AdminConfig{
          HashedPassword:  mustBcrypt("hunter2"),
          TokenIssuer:     "magos", TokenAudience: "magos", TokenSigningKey: []byte("dev"), TokenTTL: time.Hour,
      }}}
      req := httptest.NewRequest(http.MethodPost, "/api/v1alpha1/auth/login", nil)
      req.Header.Set("Authorization", "Bearer wrong")
      rec := httptest.NewRecorder()
      h.AdminLogin(rec, req)
      assert.Equal(t, http.StatusForbidden, rec.Code)
  }

  func TestAdminLogin_OK_ReturnsJWT(t *testing.T) {
      // same setup, password "hunter2", expect 200, body JSON contains "idToken" field, JWT parses
  }

  func TestAdminLogin_AdminDisabled_403(t *testing.T) {
      h := &Handler{Cfg: ServerConfig{}} // AdminConfig nil
      req := httptest.NewRequest(http.MethodPost, "/api/v1alpha1/auth/login", nil)
      req.Header.Set("Authorization", "Bearer anything")
      rec := httptest.NewRecorder()
      h.AdminLogin(rec, req)
      assert.Equal(t, http.StatusForbidden, rec.Code)
  }
  ```
  Add a `mustBcrypt` helper at the bottom of the test file.

- [ ] **Step 2:** Run `go test ./api/internal/api/auth -v -run TestAdminLogin`. Expected: FAIL (Handler/AdminLogin undefined).

- [ ] **Step 3: Implement `admin_login.go`:**
  ```go
  package auth

  import (
      "encoding/json"
      "errors"
      "net/http"
      "strings"
      "time"

      "github.com/golang-jwt/jwt/v5"
      "github.com/google/uuid"
      "golang.org/x/crypto/bcrypt"
  )

  type Handler struct {
      Cfg ServerConfig
  }

  type adminLoginResponse struct {
      IDToken string `json:"idToken"`
  }

  // AdminLogin authenticates as the admin user.
  //	@id			AdminLogin
  //	@Summary	Admin login
  //	@Description Authenticate as the admin user if enabled. Send the password as `Authorization: Bearer <password>`.
  //	@Tags		System
  //	@Produce	json
  //	@Success	200	{object}	adminLoginResponse
  //	@Router		/api/v1alpha1/auth/login [post]
  func (h *Handler) AdminLogin(w http.ResponseWriter, r *http.Request) {
      if h.Cfg.AdminConfig == nil {
          writeError(w, http.StatusForbidden, "admin user is not enabled")
          return
      }
      authHdr := r.Header.Get("Authorization")
      if authHdr == "" {
          writeError(w, http.StatusBadRequest, "Authorization header is required")
          return
      }
      const prefix = "Bearer "
      if !strings.HasPrefix(authHdr, prefix) || len(authHdr) == len(prefix) {
          writeError(w, http.StatusBadRequest, "Authorization header must be in format 'Bearer <password>'")
          return
      }
      password := authHdr[len(prefix):]
      if err := bcrypt.CompareHashAndPassword(
          []byte(h.Cfg.AdminConfig.HashedPassword),
          []byte(password),
      ); err != nil {
          writeError(w, http.StatusForbidden, "invalid password")
          return
      }
      now := time.Now()
      tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
          IssuedAt:  jwt.NewNumericDate(now),
          Issuer:    h.Cfg.AdminConfig.TokenIssuer,
          Audience:  []string{h.Cfg.AdminConfig.TokenAudience},
          NotBefore: jwt.NewNumericDate(now),
          Subject:   "admin",
          ID:        uuid.NewString(),
          ExpiresAt: jwt.NewNumericDate(now.Add(h.Cfg.AdminConfig.TokenTTL)),
      })
      signed, err := tok.SignedString(h.Cfg.AdminConfig.TokenSigningKey)
      if err != nil {
          writeError(w, http.StatusInternalServerError, "failed to sign token")
          return
      }
      writeJSON(w, http.StatusOK, adminLoginResponse{IDToken: signed})
  }

  func writeJSON(w http.ResponseWriter, status int, v any) {
      w.Header().Set("Content-Type", "application/json")
      w.WriteHeader(status)
      _ = json.NewEncoder(w).Encode(v)
  }

  func writeError(w http.ResponseWriter, status int, msg string) {
      writeJSON(w, status, map[string]string{"error": msg})
  }
  ```

- [ ] **Step 4:** Run tests again. Expected: PASS.

- [ ] **Step 5: Commit.** User runs:
  ```
  git add api/internal/api/auth/admin_login.go api/internal/api/auth/admin_login_test.go
  git commit -m "feat(api): admin login endpoint"
  ```

### Task 2.8: Public config handler

**Files:**
- Source: `/Users/bschaatsbergen/github/akuity/kargo/pkg/server/get_public_config_v1alpha1.go` (the `getPublicConfig` gin handler)
- Create: `api/internal/api/auth/public_config.go`
- Create: `api/internal/api/auth/public_config_test.go`

- [ ] **Step 1: Failing tests:**
  ```go
  func TestGetPublicConfig_AdminOnly(t *testing.T) {
      h := &Handler{Cfg: ServerConfig{AdminConfig: &AdminConfig{}}}
      // ... call, assert body matches {"adminAccountEnabled":true,"skipAuth":false}
  }
  func TestGetPublicConfig_LocalMode(t *testing.T) {
      h := &Handler{Cfg: ServerConfig{LocalMode: true}}
      // ... assert body shows skipAuth:true
  }
  func TestGetPublicConfig_OIDC(t *testing.T) {
      h := &Handler{Cfg: ServerConfig{OIDCConfig: &oidc.Config{IssuerURL: "https://idp", ClientID: "magos"}}}
      // ... assert oidcConfig present with issuerUrl
  }
  ```

- [ ] **Step 2:** Run, expect FAIL.

- [ ] **Step 3: Implement `public_config.go`:**
  ```go
  type publicConfig struct {
      AdminAccountEnabled bool        `json:"adminAccountEnabled"`
      OIDCConfig          *oidcConfig `json:"oidcConfig,omitempty"`
      SkipAuth            bool        `json:"skipAuth"`
  }

  type oidcConfig struct {
      IssuerURL   string   `json:"issuerUrl"`
      ClientID    string   `json:"clientId"`
      CLIClientID string   `json:"cliClientId,omitempty"`
      Scopes      []string `json:"scopes,omitempty"`
  }

  //	@id GetPublicConfig
  //	@Summary Retrieve public server configuration
  //	@Tags System
  //	@Produce json
  //	@Success 200 {object} publicConfig
  //	@Router /api/v1alpha1/system/public-config [get]
  func (h *Handler) GetPublicConfig(w http.ResponseWriter, _ *http.Request) {
      var oc *oidcConfig
      if h.Cfg.OIDCConfig != nil {
          oc = &oidcConfig{
              IssuerURL:   h.Cfg.OIDCConfig.IssuerURL,
              ClientID:    h.Cfg.OIDCConfig.ClientID,
              CLIClientID: h.Cfg.OIDCConfig.CLIClientID,
              Scopes:      append(h.Cfg.OIDCConfig.DefaultScopes, h.Cfg.OIDCConfig.AdditionalScopes...),
          }
      }
      writeJSON(w, http.StatusOK, publicConfig{
          AdminAccountEnabled: h.Cfg.AdminConfig != nil,
          OIDCConfig:          oc,
          SkipAuth:            h.Cfg.LocalMode,
      })
  }
  ```

- [ ] **Step 4:** Tests PASS.

- [ ] **Step 5: Commit.** User runs:
  ```
  git add api/internal/api/auth/public_config.go api/internal/api/auth/public_config_test.go
  git commit -m "feat(api): public config endpoint"
  ```

### Task 2.9: Authenticate function and middleware

**Files:**
- Source: `/Users/bschaatsbergen/github/akuity/kargo/pkg/server/option/auth.go` (lines 269-565: `listServiceAccounts`, `authenticate`, `verifyIDPIssuedToken`, `verifyKargoIssuedToken`, `verifyKubernetesToken`)
- Create: `api/internal/api/auth/middleware.go`
- Create: `api/internal/api/auth/middleware_test.go`

- [ ] **Step 1: Implement the type.** Top of `middleware.go`:
  ```go
  type Middleware struct {
      cfg            ServerConfig
      client         client.Client
      restConfig     *rest.Config
      oidcTokenVerifyFn goOIDCIDTokenVerifyFn
      parseUnverifiedJWTFn func(string, jwt.Claims) (*jwt.Token, []string, error)
  }

  func NewMiddleware(cfg ServerConfig, cl client.Client, rc *rest.Config) *Middleware {
      m := &Middleware{cfg: cfg, client: cl, restConfig: rc}
      m.parseUnverifiedJWTFn = jwt.NewParser(jwt.WithoutClaimsValidation()).ParseUnverified
      if cfg.OIDCConfig != nil {
          m.oidcTokenVerifyFn = oidc.NewMultiClientVerifier(context.Background(), cfg.OIDCConfig, cfg.DexProxyConfig)
      }
      return m
  }
  ```

- [ ] **Step 2: Port `authenticate`** verbatim from Kargo's lines 347-463. Substitutions:
  - `procedure` parameter becomes `path` parameter (path string).
  - `exemptProcedures` map keyed by path (full list above).
  - References to `kargoapi.LabelKeyProject` and `LabelValueTrue` become `"magosproject.io/project"` and `"true"`.
  - `a.cfg.OIDCConfig.GlobalServiceAccountNamespaces` reads from our `cfg.OIDCConfig.GlobalServiceAccountNamespaces`.

- [ ] **Step 3: Port `listServiceAccounts`** verbatim from Kargo lines 269-339, applying the same label rename.

- [ ] **Step 4: Port `verifyKargoIssuedToken`, `verifyIDPIssuedToken`, `verifyKubernetesToken`** verbatim from Kargo lines 470-565.

- [ ] **Step 5: Implement the `http.Handler` adapter.** Append to `middleware.go`:
  ```go
  func (m *Middleware) Handler(next http.Handler) http.Handler {
      return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          path := r.Method + " " + r.URL.Path
          // Token can be in Authorization header OR ?access_token=... for SSE clients
          hdr := r.Header
          if hdr.Get("Authorization") == "" {
              if qp := r.URL.Query().Get("access_token"); qp != "" {
                  hdr = hdr.Clone()
                  hdr.Set("Authorization", "Bearer "+qp)
              }
          }
          ctx, err := m.authenticate(r.Context(), path, hdr)
          if err != nil {
              writeError(w, http.StatusUnauthorized, err.Error())
              return
          }
          next.ServeHTTP(w, r.WithContext(ctx))
      })
  }
  ```
  The `exemptPaths` map at the top of the file:
  ```go
  var exemptPaths = map[string]struct{}{
      "GET /healthz":                                    {},
      "GET /readyz":                                     {},
      "GET /openapi.json":                               {},
      "GET /docs":                                       {},
      "POST /api/v1alpha1/auth/login":                   {},
      "GET /api/v1alpha1/system/public-config":          {},
  }
  ```
  Plus: any path beginning with `/dex/` is exempt (Dex needs to be discoverable). Check this with `strings.HasPrefix` before the map lookup.

- [ ] **Step 6: Write tests** covering:
  - Exempt route passes through with no token.
  - Missing token on a non-exempt path → 401.
  - Invalid JWT → 401.
  - Valid admin JWT → handler runs, `user.Info.IsAdmin == true`.
  - SA token recognized via TokenReview (fake the verifier).
  - Query-param `access_token` is accepted for SSE-shaped paths.

  Each test uses a tiny fake `next` handler that captures the context.

- [ ] **Step 7:** Run `go test ./api/internal/api/auth -v -run TestMiddleware -run TestAuthenticate`. Expected: PASS.

- [ ] **Step 8: Commit.** User runs:
  ```
  git add api/internal/api/auth/middleware.go api/internal/api/auth/middleware_test.go
  git commit -m "feat(api): auth middleware ported from Kargo"
  ```

### Task 2.10: Authz helper

**Files:**
- Create: `api/internal/api/auth/authz.go`
- Create: `api/internal/api/auth/authz_test.go`

- [ ] **Step 1: Implement.** `authz.go`:
  ```go
  package auth

  import (
      "context"
      "errors"

      authzv1 "k8s.io/api/authorization/v1"
      "k8s.io/apimachinery/pkg/runtime/schema"
      "k8s.io/client-go/kubernetes"

      "github.com/magosproject/magos/api/internal/api/auth/user"
  )

  // AuthorizeNamespaced returns nil if the user in the context is allowed
  // to perform verb on gvr/name in namespace. Admin bypasses.
  func AuthorizeNamespaced(
      ctx context.Context,
      kube kubernetes.Interface,
      verb string,
      gvr schema.GroupVersionResource,
      namespace, name string,
  ) error {
      info, ok := user.InfoFromContext(ctx)
      if !ok {
          return errors.New("no user info in context")
      }
      if info.IsAdmin {
          return nil
      }
      for _, sas := range info.ServiceAccountsByNamespace {
          for sa := range sas {
              review := &authzv1.SubjectAccessReview{
                  Spec: authzv1.SubjectAccessReviewSpec{
                      User: "system:serviceaccount:" + sa.Namespace + ":" + sa.Name,
                      ResourceAttributes: &authzv1.ResourceAttributes{
                          Namespace: namespace,
                          Verb:      verb,
                          Group:     gvr.Group,
                          Resource:  gvr.Resource,
                          Name:      name,
                      },
                  },
              }
              res, err := kube.AuthorizationV1().SubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
              if err != nil {
                  return err
              }
              if res.Status.Allowed {
                  return nil
              }
          }
      }
      return errors.New("forbidden")
  }
  ```

- [ ] **Step 2: Test** with a fake `kubernetes.Interface` (use `k8s.io/client-go/kubernetes/fake`). Three cases: admin bypass, allowed, denied.

- [ ] **Step 3:** Run, PASS.

- [ ] **Step 4: Commit.** User runs:
  ```
  git add api/internal/api/auth/authz.go api/internal/api/auth/authz_test.go
  git commit -m "feat(api): SubjectAccessReview authz helper"
  ```

---

## Phase 3: Wire the middleware and update handlers

### Task 3.1: Update `server.go` to use the middleware

**Files:**
- Modify: `api/internal/api/server.go`

- [ ] **Step 1:** Update the `Server` struct:
  ```go
  type Server struct {
      logger             *slog.Logger
      authHandler        *auth.Handler
      authMiddleware     *auth.Middleware
      projectHandler     *handlers.ProjectHandler
      // ... existing handlers
      dexProxy           http.Handler  // nil if dex disabled
  }
  ```

- [ ] **Step 2:** Update `NewServer` to accept `cfg auth.ServerConfig` and a ctrl-runtime `client.Client` (for the middleware's namespace/SA queries), build the middleware, and conditionally build the Dex proxy.

- [ ] **Step 3:** Update `Router()`:
  ```go
  // Auth routes (exempt from middleware via path map)
  mux.HandleFunc("POST /api/v1alpha1/auth/login", s.authHandler.AdminLogin)
  mux.HandleFunc("GET /api/v1alpha1/system/public-config", s.authHandler.GetPublicConfig)
  if s.dexProxy != nil {
      mux.Handle("/dex/", s.dexProxy)
  }
  // ... existing route registrations unchanged
  var handler http.Handler = mux
  handler = s.loggingMiddleware(handler)
  handler = s.authMiddleware.Handler(handler)
  handler = s.recoveryMiddleware(handler)
  handler = s.corsMiddleware(handler)
  return handler
  ```

- [ ] **Step 4:** Run `go build ./...` from `api/`. Expected: success.

- [ ] **Step 5:** Run `go test ./api/internal/api -v`. Confirm existing handler tests still pass (they don't go through middleware unless they construct it).

- [ ] **Step 6: Commit.** User runs:
  ```
  git add api/internal/api/server.go
  git commit -m "feat(api): wire auth middleware into router"
  ```

### Task 3.2: Update each handler to call `AuthorizeNamespaced`

**Files:**
- Modify: `api/internal/api/handlers/project_handler.go`
- Modify: `api/internal/api/handlers/workspace_handler.go`
- Modify: `api/internal/api/handlers/rollout_handler.go`
- Modify: `api/internal/api/handlers/variableset_handler.go`

For each resource type, follow the same pattern. Here for `WorkspaceHandler.Get`:

- [ ] **Step 1: Add authz call at top of Get:**
  ```go
  func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
      namespace := r.PathValue("namespace")
      name := r.PathValue("name")
      if err := auth.AuthorizeNamespaced(
          r.Context(), h.kube, "get",
          schema.GroupVersionResource{Group: "magosproject.io", Version: "v1alpha1", Resource: "workspaces"},
          namespace, name,
      ); err != nil {
          writeError(w, http.StatusForbidden, err.Error())
          return
      }
      // ... existing logic unchanged
  }
  ```

- [ ] **Step 2:** Repeat for `List` (no name, "list" verb), `Patch` ("patch" verb), `RequestReconcile` ("update" verb on subresource - same gvr but resource: "workspaces" works; refine later if needed), `Events` ("watch" verb), `ListRuns` ("get" verb), `GetRunPhaseLog` ("get" verb), `StreamCurrentRunLog` ("get" verb).

- [ ] **Step 3:** For `RecordRun` and `RecordRunPhase` (the `/internal/*` endpoints), authz is different: instead of `AuthorizeNamespaced`, check that the user's SA is in `cfg.InternalAllowedSAs`. Add a helper `auth.IsInternalAllowed(ctx, allowedList []string) bool` to check `user.Info.ServiceAccountsByNamespace` against the parsed allowlist.

- [ ] **Step 4: Update handler constructor.** Each `*Handler` gets a `kube kubernetes.Interface` field (some already have it). Pass through from `NewServer`.

- [ ] **Step 5:** Build: `go build ./...` from `api/`. Expected: success.

- [ ] **Step 6:** Update or add unit tests where handlers use `AuthorizeNamespaced` (mock with `fake.NewSimpleClientset`).

- [ ] **Step 7:** Run `go test ./api/internal/api/handlers -v`. Expected: PASS.

- [ ] **Step 8: Commit.** User runs:
  ```
  git add api/internal/api/handlers api/internal/api/server.go
  git commit -m "feat(api): handlers enforce SubjectAccessReview"
  ```

### Task 3.3: Update API main to load auth config

**Files:**
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1:** After `api.NewServerWithDefaults(logger)`, refactor: introduce `auth.ServerConfigFromEnv()`, build the ctrl-runtime client, pass both to `api.NewServer`. The `NewServerWithDefaults` function in `server.go` already reads kubeconfig and builds `kubernetes.Interface`; extend it to also build a ctrl-runtime client and the auth config.

- [ ] **Step 2:** Build, expect success. The integration is local to wiring.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add api/cmd/api/main.go api/internal/api/server.go
  git commit -m "feat(api): load auth config in main"
  ```

### Task 3.4: Workspace controller's RunRecorder sends SA token

**Files:**
- Modify: `internal/controller/workspace/run_recorder.go`

- [ ] **Step 1: Read the current file** to know what exists.

- [ ] **Step 2: Add a token reader.** The controller pod has a SA token at `/var/run/secrets/kubernetes.io/serviceaccount/token`. Add:
  ```go
  const saTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

  func (r *HTTPRunRecorder) readToken() (string, error) {
      b, err := os.ReadFile(saTokenPath)
      if err != nil {
          return "", fmt.Errorf("read sa token: %w", err)
      }
      return strings.TrimSpace(string(b)), nil
  }
  ```
  Every HTTP call attaches the token. Cache for 5 minutes to avoid hammering the filesystem; refresh on read error.

- [ ] **Step 3:** Modify each outgoing request:
  ```go
  req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
  if err != nil { return err }
  if tok, err := r.readToken(); err == nil {
      req.Header.Set("Authorization", "Bearer "+tok)
  }
  ```
  The graceful fallback (no header if token unreadable) is intentional for local-dev mode where the controller runs outside the cluster.

- [ ] **Step 4: Local dev path.** In `make run`, the host-run controller does not have a SA token mounted. Read `KUBECONFIG` and use the current-context bearer token (`config.BearerToken` field of `*rest.Config`) instead. Implement a fallback chain: file → kubeconfig → empty.

- [ ] **Step 5: Unit test.** Use `httptest.NewServer` to verify the controller sets the Authorization header when a token file exists.

- [ ] **Step 6:** Run `go test ./internal/controller/workspace -v -run TestRunRecorder`. Expected: PASS.

- [ ] **Step 7: Commit.** User runs:
  ```
  git add internal/controller/workspace
  git commit -m "feat(controller): workspace controller authenticates to API"
  ```

### Task 3.5: Regenerate OpenAPI and UI types

**Files:**
- Regen: `api/internal/api/docs/swagger.json`
- Regen: `ui/app/api/types.gen.ts`

- [ ] **Step 1:** Run `make generate-swagger`. Expected: `swagger.json` updates with `AdminLogin` and `GetPublicConfig`.

- [ ] **Step 2:** Run `make generate-ui-types`. Expected: `types.gen.ts` updates with new paths and schemas.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add api/internal/api/docs ui/app/api/types.gen.ts
  git commit -m "chore: regenerate API types"
  ```

---

## Phase 4: Helm chart

### Task 4.1: Add full values block (Kargo verbatim)

**Files:**
- Modify: `charts/magos/values.yaml`

- [ ] **Step 1:** Open `charts/magos/values.yaml`. Locate the `api:` section (line ~262). Insert under it (or replace existing partial keys) a verbatim copy of Kargo's `api.adminAccount`, `api.secret`, `api.clusterRoles`, `api.oidc`, and `api.oidc.dex` blocks from `/Users/bschaatsbergen/github/akuity/kargo/charts/kargo/values.yaml` lines 140-300.

- [ ] **Step 2:** Apply rename substitution (`kargo`/`Kargo` → `magos`/`Magos` in comments and example values; image refs and external URLs left untouched).

- [ ] **Step 3:** Add `api.permissiveCORSPolicyEnabled: false` per Kargo.

- [ ] **Step 4:** Run `helm lint charts/magos`. Expected: no errors.

- [ ] **Step 5: Commit.** User runs:
  ```
  git add charts/magos/values.yaml
  git commit -m "feat(chart): add full auth + oidc + dex values"
  ```

### Task 4.2: Reorganize api templates into subdirectory

**Files:**
- Move: `charts/magos/templates/api-deployment.yaml` → `charts/magos/templates/api/deployment.yaml`
- Move: `charts/magos/templates/api-service.yaml` → `charts/magos/templates/api/service.yaml`
- Move: `charts/magos/templates/api-serviceaccount.yaml` → `charts/magos/templates/api/service-account.yaml`
- Move: `charts/magos/templates/api-clusterrole.yaml` → `charts/magos/templates/api/cluster-role.yaml`
- Move: `charts/magos/templates/api-clusterrolebinding.yaml` → `charts/magos/templates/api/cluster-role-binding.yaml`

- [ ] **Step 1:** Use `mkdir charts/magos/templates/api && git mv ...`. User runs the `git mv`s; the executor just lists what to move.

  Actually, since the executor does not run git, the executor uses plain `mv` and the user reconciles in the commit. The plan asks the executor to `mv` only, listing commands:
  ```
  mkdir -p charts/magos/templates/api
  mv charts/magos/templates/api-deployment.yaml charts/magos/templates/api/deployment.yaml
  mv charts/magos/templates/api-service.yaml charts/magos/templates/api/service.yaml
  mv charts/magos/templates/api-serviceaccount.yaml charts/magos/templates/api/service-account.yaml
  mv charts/magos/templates/api-clusterrole.yaml charts/magos/templates/api/cluster-role.yaml
  mv charts/magos/templates/api-clusterrolebinding.yaml charts/magos/templates/api/cluster-role-binding.yaml
  ```

- [ ] **Step 2:** Run `helm template charts/magos`. Expected: no errors, output unchanged.

- [ ] **Step 3: Commit.** User runs the appropriate `git mv` equivalents (which preserve history) or just commits the move.

### Task 4.3: Generate-or-preserve admin Secret

**Files:**
- Create: `charts/magos/templates/api/secret.yaml`
- Modify: `charts/magos/templates/_helpers.tpl`

- [ ] **Step 1: Add to `_helpers.tpl`:**
  ```
  {{/*
  Generate or look up a bcrypt password hash and a random signing key.
  */}}
  {{- define "magos.api.adminPasswordHash" -}}
  {{- $secret := lookup "v1" "Secret" .Release.Namespace (printf "%s-api" (include "magos.fullname" .)) -}}
  {{- if .Values.api.adminAccount.passwordHash -}}
  {{ .Values.api.adminAccount.passwordHash }}
  {{- else if $secret -}}
  {{ index $secret.data "ADMIN_ACCOUNT_PASSWORD_HASH" | b64dec }}
  {{- else -}}
  {{ printf "$2a$10$%s" (randAlphaNum 22) | bcrypt }}
  {{- end -}}
  {{- end -}}
  ```
  Plus an equivalent helper for `tokenSigningKey` using `randAlphaNum 32`.

- [ ] **Step 2: Implement `secret.yaml`:**
  ```yaml
  {{- if and .Values.api.adminAccount.enabled (not .Values.api.secret.name) }}
  apiVersion: v1
  kind: Secret
  metadata:
    name: {{ include "magos.fullname" . }}-api
    labels:
      {{- include "magos.labels" . | nindent 4 }}
  type: Opaque
  stringData:
    ADMIN_ACCOUNT_PASSWORD_HASH: {{ include "magos.api.adminPasswordHash" . | quote }}
    ADMIN_ACCOUNT_TOKEN_SIGNING_KEY: {{ include "magos.api.adminTokenSigningKey" . | quote }}
    ADMIN_ACCOUNT_TOKEN_ISSUER: {{ printf "https://%s" (default "magos" .Values.api.externalHost) | quote }}
    ADMIN_ACCOUNT_TOKEN_AUDIENCE: "magos"
  {{- end }}
  ```

- [ ] **Step 3:** `helm template charts/magos --set api.adminAccount.enabled=true`. Verify the Secret renders.

- [ ] **Step 4:** `helm template charts/magos --set api.adminAccount.passwordHash='$2a$10$hardcoded' --set api.adminAccount.enabled=true`. Verify the hash matches.

- [ ] **Step 5: Commit.** User runs:
  ```
  git add charts/magos/templates/api/secret.yaml charts/magos/templates/_helpers.tpl
  git commit -m "feat(chart): generate or preserve admin Secret"
  ```

### Task 4.4: Add api configmap

**Files:**
- Create: `charts/magos/templates/api/configmap.yaml`

- [ ] **Step 1: Implement:**
  ```yaml
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: {{ include "magos.fullname" . }}-api
    labels:
      {{- include "magos.labels" . | nindent 4 }}
  data:
    ADMIN_ACCOUNT_ENABLED: {{ .Values.api.adminAccount.enabled | quote }}
    ADMIN_ACCOUNT_TOKEN_TTL: {{ .Values.api.adminAccount.tokenTTL | quote }}
    OIDC_ENABLED: {{ .Values.api.oidc.enabled | quote }}
    {{- if .Values.api.oidc.enabled }}
    OIDC_ISSUER_URL: {{ .Values.api.oidc.issuerURL | quote }}
    OIDC_CLIENT_ID: {{ .Values.api.oidc.clientID | quote }}
    OIDC_CLI_CLIENT_ID: {{ .Values.api.oidc.cliClientID | quote }}
    OIDC_ADDITIONAL_SCOPES: {{ join "," .Values.api.oidc.additionalScopes | quote }}
    OIDC_USERNAME_CLAIM: {{ .Values.api.oidc.usernameClaim | quote }}
    OIDC_GLOBAL_SERVICE_ACCOUNT_NAMESPACES: {{ join "," .Values.api.oidc.globalServiceAccounts.namespaces | quote }}
    {{- end }}
    DEX_ENABLED: {{ .Values.api.oidc.dex.enabled | quote }}
    {{- if .Values.api.oidc.dex.enabled }}
    DEX_SERVER_ADDRESS: {{ printf "https://%s-dex-server:5556" (include "magos.fullname" .) | quote }}
    DEX_CA_CERT_PATH: "/etc/magos/dex-ca/tls.crt"
    {{- end }}
    INTERNAL_ALLOWED_SAS: {{ printf "%s/%s" .Release.Namespace (include "magos.workspaceControllerServiceAccountName" .) | quote }}
    PERMISSIVE_CORS_POLICY_ENABLED: {{ .Values.api.permissiveCORSPolicyEnabled | quote }}
  ```

- [ ] **Step 2:** `helm template charts/magos`. Render check.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add charts/magos/templates/api/configmap.yaml
  git commit -m "feat(chart): api configmap with auth env"
  ```

### Task 4.5: Update api/deployment.yaml to mount Secret + ConfigMap

**Files:**
- Modify: `charts/magos/templates/api/deployment.yaml`

- [ ] **Step 1:** Add `envFrom` to the container:
  ```yaml
  envFrom:
  - secretRef:
      name: {{ if .Values.api.secret.name }}{{ .Values.api.secret.name }}{{ else }}{{ include "magos.fullname" . }}-api{{ end }}
  - configMapRef:
      name: {{ include "magos.fullname" . }}-api
  ```

- [ ] **Step 2:** Add `automountServiceAccountToken: true` on the pod spec.

- [ ] **Step 3:** If `api.oidc.dex.enabled`, add a volume mount for the Dex CA cert (path matches `DEX_CA_CERT_PATH` in the configmap).

- [ ] **Step 4:** `helm template charts/magos --set api.adminAccount.enabled=true`. Verify the deployment references both env sources.

- [ ] **Step 5: Commit.** User runs:
  ```
  git add charts/magos/templates/api/deployment.yaml
  git commit -m "feat(chart): mount auth secret and configmap into api"
  ```

### Task 4.6: Add api cluster role extras

**Files:**
- Modify: `charts/magos/templates/api/cluster-role.yaml`

- [ ] **Step 1:** Append:
  ```yaml
  - apiGroups:
    - authentication.k8s.io
    resources:
    - tokenreviews
    verbs:
    - create
  - apiGroups:
    - authorization.k8s.io
    resources:
    - subjectaccessreviews
    verbs:
    - create
  - apiGroups:
    - ""
    resources:
    - serviceaccounts
    - namespaces
    verbs:
    - get
    - list
    - watch
  {{- with .Values.api.clusterRoles.admin.additionalRules }}
  {{ toYaml . }}
  {{- end }}
  ```

- [ ] **Step 2:** Render check via `helm template`.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add charts/magos/templates/api/cluster-role.yaml
  git commit -m "feat(chart): grant api access to TokenReview and SAR"
  ```

### Task 4.7: Add Magos built-in cluster roles

**Files:**
- Create: `charts/magos/templates/api/cluster-roles.yaml`
- Create: `charts/magos/templates/api/cluster-role-bindings.yaml`

- [ ] **Step 1: Implement `cluster-roles.yaml`** with four ClusterRoles: `magos-admin`, `magos-project-creator`, `magos-user`, `magos-viewer`. The rule sets:
  - `magos-admin`: `*` verbs on `projects, workspaces, rollouts, variablesets` in apiGroup `magosproject.io`; plus `get,list,watch` on `secrets`, `configmaps` in `core`.
  - `magos-project-creator`: `magos-user` rules plus `create` on `projects`.
  - `magos-user`: `get,list,watch` on `projects` (cluster-scoped) and on `workspaces, rollouts, variablesets`.
  - `magos-viewer`: same as `magos-user` (alias for compatibility with Kargo's naming).
  Each role appends `.Values.api.clusterRoles.<name>.additionalRules` if set.

- [ ] **Step 2: Implement `cluster-role-bindings.yaml`** that iterates over `api.oidc.{admins,projectCreators,users,viewers}.claims` and renders a binding for each claim group. Match Kargo's approach (the bindings use `User` subjects with the OIDC username claim format).

- [ ] **Step 3:** Render-test via `helm template charts/magos --set api.oidc.enabled=true --set 'api.oidc.admins.claims.groups[0]=platform-admin'`.

- [ ] **Step 4: Commit.** User runs:
  ```
  git add charts/magos/templates/api/cluster-roles.yaml charts/magos/templates/api/cluster-role-bindings.yaml
  git commit -m "feat(chart): magos built in cluster roles and bindings"
  ```

### Task 4.8: Copy Dex server templates

**Files:**
- Source dir: `/Users/bschaatsbergen/github/akuity/kargo/charts/kargo/templates/dex-server/`
- Create dir: `charts/magos/templates/dex-server/`

- [ ] **Step 1:** Copy all files: `cp /Users/bschaatsbergen/github/akuity/kargo/charts/kargo/templates/dex-server/*.yaml charts/magos/templates/dex-server/`.

- [ ] **Step 2:** In each file: search-and-replace `kargo` with `magos`, `Kargo` with `Magos`, `akuity.io` with `magosproject.io`, `.Values.kargo` with `.Values.magos` if present, helper template names `kargo.fullname` etc. with `magos.fullname`. Also rename label `app.kubernetes.io/instance: kargo` to `app.kubernetes.io/instance: magos` only if hardcoded; usually it's interpolated.

- [ ] **Step 3:** Reference path: `.Values.api.oidc.dex.*` (matches our values structure).

- [ ] **Step 4:** Render-test: `helm template charts/magos --set api.oidc.dex.enabled=true`. Expected: Dex resources present.

- [ ] **Step 5: Commit.** User runs:
  ```
  git add charts/magos/templates/dex-server
  git commit -m "feat(chart): bundle Dex server templates from Kargo"
  ```

### Task 4.9: Workspace controller deployment mounts SA token

**Files:**
- Modify: `charts/magos/templates/deployment.yaml` (or wherever the workspace controller deployment lives - verify via `grep -ln workspace charts/magos/templates`)

- [ ] **Step 1:** Add `automountServiceAccountToken: true` to the workspace controller's pod spec. Add env `MAGOS_API_URL: http://{{ include "magos.fullname" . }}-api`.

- [ ] **Step 2:** Render-test: confirm token automount is true.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add charts/magos/templates/deployment.yaml
  git commit -m "feat(chart): workspace controller mounts SA token for api auth"
  ```

### Task 4.10: NOTES.txt admin recovery hint

**Files:**
- Create or modify: `charts/magos/templates/NOTES.txt`

- [ ] **Step 1:** Add a block:
  ```
  {{- if and .Values.api.adminAccount.enabled (not .Values.api.adminAccount.passwordHash) (not .Values.api.secret.name) }}
  Magos was installed with the built in admin account enabled and a chart generated password.

  To recover the password hash:
    kubectl get secret -n {{ .Release.Namespace }} {{ include "magos.fullname" . }}-api -o jsonpath='{.data.ADMIN_ACCOUNT_PASSWORD_HASH}' | base64 -d

  Set api.adminAccount.passwordHash in values.yaml to a known hash if you want a memorable password.
  Disable the admin account once you have OIDC configured by setting api.adminAccount.enabled=false.
  {{- end }}
  ```

- [ ] **Step 2:** Render-test and confirm hint appears.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add charts/magos/templates/NOTES.txt
  git commit -m "feat(chart): admin recovery hint in NOTES"
  ```

---

## Phase 5: UI

### Task 5.1: Install oauth4webapi

**Files:**
- Modify: `ui/package.json`
- Modify: `ui/package-lock.json`

- [ ] **Step 1:** `cd ui && npm install oauth4webapi@^3`. Match the version Kargo uses in `kargo/ui/package.json` for parity. Note: this is one of the few packages that must be present at build time.

- [ ] **Step 2: Commit.** User runs:
  ```
  git add ui/package.json ui/package-lock.json
  git commit -m "ui: add oauth4webapi"
  ```

### Task 5.2: Auth module

**Files:**
- Create: `ui/app/auth/AuthContext.tsx`
- Create: `ui/app/auth/AuthProvider.tsx`
- Create: `ui/app/auth/useAuth.tsx`
- Create: `ui/app/auth/jwt-utils.ts`
- Create: `ui/app/auth/paths.ts`
- Create: `ui/app/auth/safeRedirect.ts`

- [ ] **Step 1: `paths.ts`:**
  ```ts
  export const paths = { login: "/login", home: "/" } as const;
  ```

- [ ] **Step 2: `safeRedirect.ts`:**
  ```ts
  export const redirectToQueryParam = "redirectTo";
  export function isSafeRedirectPath(p: string | null | undefined): p is string {
    if (!p) return false;
    if (!p.startsWith("/") || p.startsWith("//")) return false;
    try {
      // ensure it's a path, not a URL
      const u = new URL(p, "http://x");
      return u.host === "x" && !p.includes("\n");
    } catch {
      return false;
    }
  }
  ```

- [ ] **Step 3: `jwt-utils.ts`:** verbatim port of Kargo's `ui/src/features/auth/jwt-utils.ts` (no rewrites needed).

- [ ] **Step 4: `AuthContext.tsx`** and **`AuthProvider.tsx`:** ports of Kargo's `auth-context.tsx` and `auth-context-provider.tsx`. Adapt the storage keys to `magos.auth.token` / `magos.auth.refreshToken`.

- [ ] **Step 5: `useAuth.tsx`:** thin `useContext` wrapper.

- [ ] **Step 6:** Run `npm run typecheck` in `ui/`. Expected: PASS.

- [ ] **Step 7: Commit.** User runs:
  ```
  git add ui/app/auth
  git commit -m "ui: add auth module ported from Kargo"
  ```

### Task 5.3: ProtectedRoute

**Files:**
- Create: `ui/app/components/ProtectedRoute.tsx`

- [ ] **Step 1: Implement:**
  ```tsx
  import { Navigate, Outlet } from "react-router";
  import { useAuth } from "~/auth/useAuth";
  import { paths } from "~/auth/paths";
  import { redirectToQueryParam } from "~/auth/safeRedirect";
  import apiClient from "~/api/client";
  import { useEffect, useState } from "react";

  type PublicConfig = { skipAuth: boolean };

  export default function ProtectedRoute() {
    const { isLoggedIn } = useAuth();
    const [skipAuth, setSkipAuth] = useState<boolean | null>(null);
    useEffect(() => {
      apiClient.GET("/api/v1alpha1/system/public-config").then((r) => {
        setSkipAuth((r.data as PublicConfig | undefined)?.skipAuth ?? false);
      });
    }, []);
    if (skipAuth === null) return null;
    if (!skipAuth && !isLoggedIn) {
      const here = window.location.pathname;
      const qs = here !== paths.home ? `?${redirectToQueryParam}=${encodeURIComponent(here)}` : "";
      return <Navigate to={`${paths.login}${qs}`} replace />;
    }
    return <Outlet />;
  }
  ```

- [ ] **Step 2:** `npm run typecheck`. Expected: PASS.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add ui/app/components/ProtectedRoute.tsx
  git commit -m "ui: add ProtectedRoute"
  ```

### Task 5.4: Login route

**Files:**
- Create: `ui/app/routes/login.tsx`
- Create: `ui/app/hooks/useDocumentTitle.ts`

- [ ] **Step 1: `useDocumentTitle.ts`:**
  ```ts
  import { useEffect } from "react";
  export function useDocumentTitle(parts: string[]) {
    useEffect(() => {
      document.title = [...parts, "Magos"].join(" | ");
    }, [parts.join(":")]);
  }
  ```

- [ ] **Step 2: Implement `login.tsx`.** A Mantine-styled login page. Logic mirrors Kargo's `pages/login/login.tsx`:
  - Reads `useGetPublicConfig` data
  - If `skipAuth`: `<Navigate to={paths.home} replace />`
  - If `isLoggedIn`: `<Navigate to={redirectTo || paths.home} replace />`
  - Renders `AdminLogin` and `OIDCLogin` blocks based on `data?.adminAccountEnabled` and `data?.oidcConfig`
  - Renders empty state when neither is enabled

  `AdminLogin` (inline in the same file or a sibling component):
  ```tsx
  function AdminLogin() {
    const { login } = useAuth();
    const [pwd, setPwd] = useState("");
    const [loading, setLoading] = useState(false);
    const [err, setErr] = useState<string>();
    const onSubmit = async (e: FormEvent) => {
      e.preventDefault();
      setLoading(true); setErr(undefined);
      try {
        const res = await fetch("/api/v1alpha1/auth/login", {
          method: "POST",
          headers: { Authorization: `Bearer ${pwd}` },
        });
        if (!res.ok) throw new Error("invalid password");
        const body = await res.json();
        login(body.idToken);
      } catch (e: any) {
        setErr(e.message);
      } finally { setLoading(false); }
    };
    return (
      <form onSubmit={onSubmit}>
        <Stack gap="sm">
          <PasswordInput label="Password" value={pwd} onChange={(e) => setPwd(e.currentTarget.value)} required />
          {err && <Text c="red" size="sm">{err}</Text>}
          <Button type="submit" loading={loading} color="magos.5" fullWidth>Login</Button>
        </Stack>
      </form>
    );
  }
  ```

  `OIDCLogin`: port of Kargo's `oidc-login.tsx`, swapping `notification.error` for Mantine's `notifications.show` and `Button` for Mantine's `Button`. PKCE flow uses `oauth4webapi` identically.

- [ ] **Step 3:** `npm run typecheck` in `ui/`. PASS.

- [ ] **Step 4: Commit.** User runs:
  ```
  git add ui/app/routes/login.tsx ui/app/hooks/useDocumentTitle.ts
  git commit -m "ui: login page with admin + OIDC"
  ```

### Task 5.5: Route table update

**Files:**
- Modify: `ui/app/routes.ts`

- [ ] **Step 1:** Replace the existing `routes.ts` content with:
  ```ts
  import { type RouteConfig, index, route, layout } from "@react-router/dev/routes";

  export default [
    route("login", "routes/login.tsx"),
    layout("components/ProtectedRoute.tsx", [
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
    ]),
  ] satisfies RouteConfig;
  ```

- [ ] **Step 2:** `npm run typecheck`. PASS.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add ui/app/routes.ts
  git commit -m "ui: gate routes behind ProtectedRoute"
  ```

### Task 5.6: Mount AuthProvider in root

**Files:**
- Modify: `ui/app/root.tsx`

- [ ] **Step 1:** Wrap the `<Outlet />` in the existing `App` (or the `<MantineProvider>` children) with `<AuthProvider>` from `~/auth/AuthProvider`.

- [ ] **Step 2:** `npm run typecheck`. PASS.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add ui/app/root.tsx
  git commit -m "ui: mount AuthProvider in root"
  ```

### Task 5.7: API client attaches Authorization header

**Files:**
- Modify: `ui/app/api/client.ts`

- [ ] **Step 1: Replace with:**
  ```ts
  import createClient from "openapi-fetch";
  import type { paths } from "./types.gen";
  import { paths as routePaths } from "~/auth/paths";

  const apiClient = createClient<paths>({ baseUrl: "/" });

  apiClient.use({
    onRequest({ request }) {
      const token = localStorage.getItem("magos.auth.token");
      if (token) request.headers.set("Authorization", `Bearer ${token}`);
      return request;
    },
    onResponse({ response }) {
      if (response.status === 401) {
        localStorage.removeItem("magos.auth.token");
        if (window.location.pathname !== routePaths.login) {
          window.location.href = routePaths.login;
        }
      }
      return response;
    },
  });

  export default apiClient;
  ```

- [ ] **Step 2:** `npm run typecheck`. PASS.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add ui/app/api/client.ts
  git commit -m "ui: attach Authorization header on every API call"
  ```

### Task 5.8: SSE hooks add access_token query param

**Files:**
- Modify: `ui/app/hooks/useSSEItem.ts`
- Modify: `ui/app/hooks/useSSEList.ts`
- Modify: `ui/app/hooks/useSSEStream.ts`
- Modify: `ui/app/hooks/useSSEFiltered.ts`

- [ ] **Step 1:** Each hook constructs an `EventSource(url)`. Wrap the URL construction to append `?access_token=...` if a token is in localStorage. Helper:
  ```ts
  function withAccessToken(url: string): string {
    const token = localStorage.getItem("magos.auth.token");
    if (!token) return url;
    const sep = url.includes("?") ? "&" : "?";
    return `${url}${sep}access_token=${encodeURIComponent(token)}`;
  }
  ```
  Apply in each hook.

- [ ] **Step 2:** `npm run typecheck`. PASS.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add ui/app/hooks
  git commit -m "ui: SSE hooks pass token via query parameter"
  ```

### Task 5.9: AppShell header user menu

**Files:**
- Modify: `ui/app/components/AppShell.tsx`

- [ ] **Step 1:** Import `IconUserCircle` from `@tabler/icons-react` and `Menu` from `@mantine/core`. Use `useAuth` for `JWTInfo` and `logout`.

- [ ] **Step 2:** In the right-side `Group gap="xs"` (where GitHub/Discord icons live), insert before them:
  ```tsx
  <Menu position="bottom-end" withArrow>
    <Menu.Target>
      <ActionIcon variant="subtle" color="magos.5">
        <IconUserCircle size={20} />
      </ActionIcon>
    </Menu.Target>
    <Menu.Dropdown>
      {JWTInfo?.email && <Menu.Label>{JWTInfo.email}</Menu.Label>}
      <Menu.Item color="red" onClick={() => { logout(); navigate("/login"); }}>Sign out</Menu.Item>
    </Menu.Dropdown>
  </Menu>
  ```
  Use `useNavigate` from `react-router`.

- [ ] **Step 3:** `npm run typecheck`. PASS.

- [ ] **Step 4: Commit.** User runs:
  ```
  git add ui/app/components/AppShell.tsx
  git commit -m "ui: header user menu"
  ```

### Task 5.10: UI smoke

- [ ] **Step 1:** From the repo root, `make run` (assuming Phase 4 chart changes are in `hack/local-values.yaml`).
- [ ] **Step 2:** Open `http://localhost:5713`. Expected: redirected to `/login`.
- [ ] **Step 3:** Submit password "admin". Expected: redirected to `/`.
- [ ] **Step 4:** Click "Sign out". Expected: back at `/login`.
- [ ] **Step 5:** Click an SSE-heavy view (a Workspace detail page). Expected: live log stream works.
- [ ] **Step 6:** Report findings. No commit (manual test only).

---

## Phase 6: Documentation

### Task 6.1: Admin account doc

**Files:**
- Create: `website/contents/docs/operations/authentication/admin-account.mdx`

- [ ] **Step 1: Write** ~150-200 lines covering: when to enable, how to set a known password (bcrypt instructions with `htpasswd -bnBC 10 "" 'your-password'`), how to disable, recovery from NOTES.
- [ ] **Step 2:** `npm run build` in website. PASS.
- [ ] **Step 3: Commit.** User runs:
  ```
  git add website/contents/docs/operations/authentication/admin-account.mdx
  git commit -m "docs: admin account"
  ```

### Task 6.2: OIDC doc

**Files:**
- Create: `website/contents/docs/operations/authentication/oidc.mdx`

- [ ] **Step 1:** Cover: enabling OIDC, claims, the `rbac.magosproject.io/claim.<name>` annotation on ServiceAccounts, role bindings via `api.oidc.{admins,projectCreators,users,viewers}.claims`, `globalServiceAccounts.namespaces`.
- [ ] **Step 2:** Build. PASS.
- [ ] **Step 3: Commit.**

### Task 6.3: Dex doc

**Files:**
- Create: `website/contents/docs/operations/authentication/dex.mdx`

- [ ] **Step 1:** Cover: Dex purpose, connectors list with Google/GitHub/Microsoft examples, the `/dex` proxy URL.
- [ ] **Step 2:** Build. PASS.
- [ ] **Step 3: Commit.**

### Task 6.4: NOTICE attribution

**Files:**
- Create or modify: `NOTICE`

- [ ] **Step 1:** Add or extend with:
  ```
  Magos contains code derived from Kargo (https://github.com/akuity/kargo),
  copyright the Kargo Authors, licensed under the Apache License 2.0.
  Files derived: api/internal/api/auth/{dex,user,oidc,indexer}, and the
  Helm templates under charts/magos/templates/dex-server and parts of
  charts/magos/templates/api.
  ```

- [ ] **Step 2: Commit.** User runs:
  ```
  git add NOTICE
  git commit -m "chore: attribute Kargo for ported code"
  ```

---

## Phase 7: Local dev and final wiring

### Task 7.1: hack/local-values.yaml

**Files:**
- Modify: `hack/local-values.yaml`

- [ ] **Step 1:** Add:
  ```yaml
  api:
    adminAccount:
      enabled: true
      passwordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy" # bcrypt of "admin"
      tokenSigningKey: "dev-only-do-not-use-in-production"
      tokenTTL: 24h
  ```
  The bcrypt hash above is the canonical "admin" example used in many projects.

- [ ] **Step 2: Commit.** User runs:
  ```
  git add hack/local-values.yaml
  git commit -m "dev: enable admin login in local-values"
  ```

### Task 7.2: Makefile passes env to host-run API

**Files:**
- Modify: `Makefile`

- [ ] **Step 1:** In the `run` target, before launching the API binary, extract the admin env from the in-cluster Secret (similar to existing `MAGOS_LOGS_*` extraction):
  ```
  ADMIN_ENV := $(shell kubectl get secret -n magos-system magos-api -o jsonpath='{.data}' | ...)
  ```
  Or simpler: source from `hack/local-values.yaml` directly.

- [ ] **Step 2:** Test the loop: `make run`, hit `/login`, log in.

- [ ] **Step 3: Commit.** User runs:
  ```
  git add Makefile
  git commit -m "dev: pass admin env to host run api in make run"
  ```

---

## Self-review checklist (executor: complete before declaring done)

- [ ] Every spec requirement (excluding the out-of-scope migration tool and new chainsaw tests) has at least one task implementing it (auth backend, chart, UI, Project scope change, docs).
- [ ] No "TBD" or "TODO" placeholders remain in this plan.
- [ ] Type names are consistent: `Handler`, `Middleware`, `ServerConfig`, `AdminConfig`, `oidcConfig` (Go) vs `OIDCConfig` (TS) line up with code in different languages.
- [ ] All env var names match Kargo exactly: `ADMIN_ACCOUNT_*`, `OIDC_*`, `DEX_*`, `LOCAL_MODE`.
- [ ] All label keys match the rename: `magosproject.io/project=true`, `rbac.magosproject.io/claim.<name>`.
- [ ] All localStorage keys: `magos.auth.token`, `magos.auth.refreshToken`.
- [ ] All exempt paths in the middleware match the new route table.
- [ ] The chainsaw tests reference real, renderable chart paths.
- [ ] Migration tool's idempotency is exercised.
- [ ] NOTICE attribution is present.
