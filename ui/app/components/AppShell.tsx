import {
  AppShell,
  Anchor,
  Avatar,
  Burger,
  ActionIcon,
  Container,
  Group,
  NavLink,
  Stack,
  Text,
  Tooltip,
  useMantineTheme,
  useMantineColorScheme,
} from "@mantine/core";
import { useDisclosure, useLocalStorage } from "@mantine/hooks";
import {
  IconStack2,
  IconFolderOpen,
  IconChevronsLeft,
  IconChevronsRight,
  IconSettings,
  IconShield,
  IconUsers,
  IconKey,
  IconSun,
  IconMoon,
  IconBrandGithub,
  IconBrandDiscord,
  IconBraces,
  IconHexagon,
  IconArrowsShuffle,
} from "@tabler/icons-react";
import { Link, Outlet, useLocation } from "react-router";
import { currentUser } from "../mock-data/user";
import BlinkingCursor from "./BlinkingCursor";

const navItems = [
  { label: "Projects", icon: IconFolderOpen, to: "/projects" },
  { label: "Workspaces", icon: IconStack2, to: "/workspaces" },
  { label: "Rollouts", icon: IconArrowsShuffle, to: "/rollouts" },
  { label: "Variable Sets", icon: IconBraces, to: "/variable-sets" },
];

const adminNavItems = [
  { label: "Users", icon: IconUsers, to: "/admin/users" },
  { label: "Groups", icon: IconShield, to: "/admin/groups" },
  { label: "Permissions", icon: IconKey, to: "/admin/permissions" },
];

export default function Shell() {
  const [mobileOpen, { toggle: toggleMobile }] = useDisclosure(false);
  const [collapsed, setCollapsed] = useLocalStorage({ key: "nav-collapsed", defaultValue: false });
  const toggleCollapsed = () => setCollapsed((c) => !c);
  const location = useLocation();
  const theme = useMantineTheme();
  const { colorScheme, toggleColorScheme } = useMantineColorScheme();

  const navWidth = collapsed ? 60 : 220;

  const renderNavItem = (item: (typeof navItems)[number], extraProps?: Record<string, unknown>) => {
    const isActive = location.pathname.startsWith(item.to);
    const link = (
      <NavLink
        key={item.to}
        component={Link}
        to={item.to}
        label={
          <Text
            size="sm"
            style={{
              opacity: collapsed ? 0 : 1,
              transition: "opacity 150ms ease",
              whiteSpace: "nowrap",
              overflow: "hidden",
            }}
          >
            {item.label}
          </Text>
        }
        leftSection={<item.icon size={18} />}
        active={isActive}
        color="magos.5"
        variant="light"
        style={{ borderRadius: 6 }}
        {...extraProps}
      />
    );
    return collapsed ? (
      <Tooltip key={item.to} label={item.label} position="right">
        {link}
      </Tooltip>
    ) : (
      link
    );
  };

  return (
    <AppShell
      header={{ height: 56 }}
      navbar={{
        width: navWidth,
        breakpoint: "sm",
        collapsed: { mobile: !mobileOpen },
      }}
      padding="md"
      transitionDuration={0}
    >
      <AppShell.Header style={{ borderBottom: `1px solid ${theme.colors.magos[9]}` }}>
        <Group h="100%" px="md" justify="space-between">
          <Group gap="sm" w={navWidth - 32}>
            <Burger opened={mobileOpen} onClick={toggleMobile} hiddenFrom="sm" size="sm" />
            <Group gap="xs" wrap="nowrap" align="center">
              <IconHexagon
                size={collapsed ? 24 : 28}
                color="var(--mantine-color-magos-5)"
                stroke={2.5}
              />
              {!collapsed && (
                <Group gap={0} align="baseline">
                  <Text
                    fw={900}
                    size="22px"
                    style={{
                      color: colorScheme === "dark" ? "white" : "black",
                      letterSpacing: 3,
                    }}
                  >
                    magos
                  </Text>
                  <BlinkingCursor size="22px" fw={900} />
                </Group>
              )}
            </Group>
          </Group>

          <Group gap="xs">
            <Anchor href="#" size="sm" c="dimmed" underline="hover" style={{ fontWeight: 500 }}>
              API Reference
            </Anchor>
            <Tooltip label="Toggle color scheme">
              <ActionIcon variant="subtle" color="magos.5" onClick={() => toggleColorScheme()}>
                {colorScheme === "dark" ? <IconSun size={18} /> : <IconMoon size={18} />}
              </ActionIcon>
            </Tooltip>
            <Tooltip label="GitHub">
              <ActionIcon
                component="a"
                href="https://github.com"
                target="_blank"
                rel="noopener noreferrer"
                variant="subtle"
                color="magos.5"
              >
                <IconBrandGithub size={18} />
              </ActionIcon>
            </Tooltip>
            <Tooltip label="Discord">
              <ActionIcon
                component="a"
                href="https://discord.com"
                target="_blank"
                rel="noopener noreferrer"
                variant="subtle"
                color="magos.5"
              >
                <IconBrandDiscord size={18} />
              </ActionIcon>
            </Tooltip>
          </Group>
        </Group>
      </AppShell.Header>

      <AppShell.Navbar
        style={{
          borderRight: `1px solid ${theme.colors.magos[9]}`,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        <Stack gap={4} p="xs" style={{ flex: 1 }}>
          {navItems.map((item) => renderNavItem(item))}

          {currentUser.role === "admin" && (
            <>
              <div
                style={{ borderBottom: `1px solid ${theme.colors.magos[9]}`, margin: "4px 0" }}
              />
              <Text
                size="xs"
                c="dimmed"
                px={4}
                style={{
                  display: collapsed ? "none" : "block",
                  whiteSpace: "nowrap",
                  overflow: "hidden",
                }}
              >
                Admin
              </Text>
              {adminNavItems.map((item) => renderNavItem(item))}
            </>
          )}
        </Stack>

        <Stack
          gap={0}
          p="xs"
          align="stretch"
          style={{ borderTop: `1px solid ${theme.colors.magos[9]}` }}
        >
          <Tooltip label={currentUser.email} position="right" disabled={!collapsed}>
            <Group justify="space-between" px={4} py={6} wrap="nowrap" w="100%">
              <Group gap="xs" wrap="nowrap" style={{ minWidth: 0, flex: 1 }}>
                <Avatar size={28} color="magos" radius="xl" style={{ flexShrink: 0 }}>
                  {currentUser.name[0]}
                </Avatar>
                <Stack
                  gap={0}
                  style={{
                    minWidth: 0,
                    opacity: collapsed ? 0 : 1,
                    transition: "opacity 150ms ease",
                    overflow: "hidden",
                  }}
                >
                  <Text size="xs" fw={600} lh={1.2} truncate>
                    {currentUser.name}
                  </Text>
                  <Text size="xs" c="dimmed" lh={1.2} truncate>
                    {currentUser.email}
                  </Text>
                </Stack>
              </Group>
              <Tooltip label="Settings" disabled={collapsed}>
                <ActionIcon
                  component={Link}
                  to="/settings"
                  variant="subtle"
                  color="magos"
                  size="sm"
                  style={{
                    flexShrink: 0,
                    opacity: collapsed ? 0 : 1,
                    pointerEvents: collapsed ? "none" : undefined,
                  }}
                >
                  <IconSettings size={16} />
                </ActionIcon>
              </Tooltip>
            </Group>
          </Tooltip>
        </Stack>

        <Stack
          gap={0}
          p="xs"
          visibleFrom="sm"
          style={{ borderTop: `1px solid ${theme.colors.magos[9]}` }}
        >
          <Tooltip label={collapsed ? "Expand menu" : "Collapse menu"} position="right">
            <ActionIcon
              variant="subtle"
              color="magos"
              onClick={toggleCollapsed}
              style={{ width: "100%" }}
            >
              {collapsed ? <IconChevronsRight size={16} /> : <IconChevronsLeft size={16} />}
            </ActionIcon>
          </Tooltip>
        </Stack>
      </AppShell.Navbar>

      <AppShell.Main>
        <Container size={1600} px={0}>
          <Outlet />
        </Container>
      </AppShell.Main>
    </AppShell>
  );
}
