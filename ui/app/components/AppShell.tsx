import {
  AppShell,
  Anchor,
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
  IconSun,
  IconMoon,
  IconBrandGithub,
  IconBrandDiscord,
  IconBraces,
  IconHexagon,
  IconArrowsShuffle,
  IconSettings,
} from "@tabler/icons-react";
import { Link, Outlet, useLocation } from "react-router";
import BlinkingCursor from "./BlinkingCursor";

const navItems = [
  { label: "Projects", icon: IconFolderOpen, to: "/projects" },
  { label: "Workspaces", icon: IconStack2, to: "/workspaces" },
  { label: "Rollouts", icon: IconArrowsShuffle, to: "/rollouts" },
  { label: "Variable Sets", icon: IconBraces, to: "/variable-sets" },
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
          <Group gap="sm">
            <Burger opened={mobileOpen} onClick={toggleMobile} hiddenFrom="sm" size="sm" />
            <Group gap="xs" wrap="nowrap" align="center">
              <IconHexagon
                size={28}
                color="var(--mantine-color-magos-5)"
                stroke={2.5}
                style={{ flexShrink: 0 }}
              />
              {!collapsed && (
                <Group gap={0} align="baseline">
                  <Text
                    fw={900}
                    size="22px"
                    style={{
                      color: "var(--mantine-color-text)",
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


        </Stack>

        <Stack
          gap={4}
          p="xs"
          align={collapsed ? "center" : "stretch"}
          visibleFrom="sm"
          style={{ borderTop: `1px solid ${theme.colors.magos[9]}` }}
        >
          {renderNavItem({ label: "Settings", icon: IconSettings, to: "/settings" })}
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
