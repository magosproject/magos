import { Anchor, Breadcrumbs as MantineBreadcrumbs, Text } from "@mantine/core";
import { Link } from "react-router";

interface Crumb {
  label: string;
  to?: string;
}

interface Props {
  crumbs: Crumb[];
}

export default function Breadcrumbs({ crumbs }: Props) {
  return (
    <MantineBreadcrumbs
      separatorMargin={6}
      separator={
        <Text size="sm" lh={1} c="dimmed">
          /
        </Text>
      }
    >
      {crumbs.map((crumb, i) => {
        const isLast = i === crumbs.length - 1;
        if (isLast || !crumb.to) {
          return (
            <Text key={crumb.label} size="sm" lh={1} c={isLast ? "bright" : "dimmed"}>
              {crumb.label}
            </Text>
          );
        }
        return (
          <Anchor key={crumb.label} component={Link} to={crumb.to} size="sm" lh={1} c="dimmed">
            {crumb.label}
          </Anchor>
        );
      })}
    </MantineBreadcrumbs>
  );
}
