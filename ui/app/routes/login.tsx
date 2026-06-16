import { useEffect, useState } from "react";
import {
  Alert,
  Anchor,
  Button,
  Center,
  Group,
  Paper,
  PasswordInput,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { IconHexagon } from "@tabler/icons-react";
import { useSearchParams } from "react-router";
import {
  adminLogin,
  getAuthConfig,
  getMe,
  isSafeRedirectPath,
  redirectToQueryParam,
  type AuthConfig,
} from "../api/auth";

export default function Login() {
  const [params] = useSearchParams();
  const [config, setConfig] = useState<AuthConfig | null>(null);
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const redirectTo = isSafeRedirectPath(params.get(redirectToQueryParam))
    ? params.get(redirectToQueryParam)
    : "/";

  useEffect(() => {
    let cancelled = false;
    void Promise.all([getAuthConfig(), getMe()])
      .then(([cfg, me]) => {
        if (cancelled) return;
        if (!cfg.enabled || me.authenticated) {
          window.location.replace(redirectTo ?? "/");
          return;
        }
        setConfig(cfg);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "failed to load auth config");
      });
    return () => {
      cancelled = true;
    };
  }, [redirectTo]);

  const submitAdmin = async () => {
    setLoading(true);
    setError(null);
    try {
      await adminLogin(password);
      window.location.replace(redirectTo ?? "/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "login failed");
    } finally {
      setLoading(false);
    }
  };

  const oidcLogin = () => {
    const loginUrl = config?.oidc.loginUrl ?? "/auth/oidc/login";
    const url = new URL(loginUrl, window.location.origin);
    if (redirectTo) url.searchParams.set(redirectToQueryParam, redirectTo);
    window.location.assign(url.toString());
  };

  return (
    <Center mih="100vh" p="md">
      <Paper w="100%" maw={440} p="xl" radius="lg" withBorder>
        <Stack gap="lg">
          <Group gap="sm">
            <IconHexagon size={34} color="var(--mantine-color-magos-5)" stroke={2.5} />
            <Stack gap={0}>
              <Title order={2}>Sign in to Magos</Title>
              <Text size="sm" c="dimmed">
                Use SSO or the configured fallback admin account.
              </Text>
            </Stack>
          </Group>

          {error && <Alert color="red">{error}</Alert>}
          {!config && !error && <Text c="dimmed">Loading authentication config...</Text>}

          {config?.oidc.enabled && (
            <Button size="md" onClick={oidcLogin}>
              Continue with SSO
            </Button>
          )}

          {config?.adminEnabled && (
            <Stack gap="sm">
              <PasswordInput
                label="Admin password"
                value={password}
                onChange={(event) => setPassword(event.currentTarget.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" && password) void submitAdmin();
                }}
              />
              <Button variant={config.oidc.enabled ? "light" : "filled"} loading={loading} disabled={!password} onClick={submitAdmin}>
                Sign in as admin
              </Button>
            </Stack>
          )}

          {config && !config.oidc.enabled && !config.adminEnabled && (
            <Alert color="yellow">Authentication is enabled, but no login method is configured.</Alert>
          )}

          <Anchor href="/docs" size="sm" c="dimmed">
            API documentation
          </Anchor>
        </Stack>
      </Paper>
    </Center>
  );
}
