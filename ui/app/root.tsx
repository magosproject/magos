import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
} from "react-router";
import { type ReactNode } from "react";
import type { Route } from "./+types/root";
import {
  Anchor,
  Center,
  ColorSchemeScript,
  MantineProvider,
  Stack,
  Text,
  Title,
  createTheme,
  mantineHtmlProps,
} from "@mantine/core";
import "@mantine/core/styles.css";
import "./app.css";

export const links: Route.LinksFunction = () => [
  { rel: "preconnect", href: "https://fonts.googleapis.com" },
  { rel: "preconnect", href: "https://fonts.gstatic.com", crossOrigin: "anonymous" },
  {
    rel: "stylesheet",
    href: "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap",
  },
];

const theme = createTheme({
  primaryColor: "magos",
  primaryShade: 5,
  colors: {
    magos: [
      "#fefce8",
      "#fef9c3",
      "#fef08a",
      "#fde047",
      "#facc15",
      "#f4e05e", // Primary hex
      "#ca8a04",
      "#a16207",
      "#854d0e",
      "#713f12",
    ],
  },
  fontFamily: "Inter, sans-serif",
  fontSizes: {
    xs: "13px",
    sm: "15px",
    md: "17px",
    lg: "19px",
    xl: "22px",
  },
});

export function Layout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" {...mantineHtmlProps} data-mantine-color-scheme="dark">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <ColorSchemeScript defaultColorScheme="dark" />
        <Meta />
        <Links />
      </head>
      <body>
        <MantineProvider theme={theme} defaultColorScheme="dark">
          {children}
        </MantineProvider>
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function App() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let status = 500;
  let message = "An unexpected error occurred.";

  if (isRouteErrorResponse(error)) {
    status = error.status;
    message =
      error.status === 404
        ? "The page you're looking for doesn't exist."
        : error.statusText || message;
  } else if (error instanceof Error) {
    message = error.message;
  }

  return (
    <Center h="100vh">
      <Stack align="center" gap="xs">
        <Title order={1} c="dimmed" style={{ fontSize: 72, lineHeight: 1 }}>
          {status}
        </Title>
        <Text size="lg" fw={500}>
          {status === 404 ? "Page not found" : "Something went wrong"}
        </Text>
        <Text size="sm" c="dimmed" ta="center" maw={400}>
          {message}
        </Text>
        <Anchor href="/" size="sm" mt="xs">
          Back to home
        </Anchor>
      </Stack>
    </Center>
  );
}
