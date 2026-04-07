import { Badge } from "@mantine/core";

interface KubeBadgeProps {
  label: string;
}

export default function KubeBadge({ label }: KubeBadgeProps) {
  return (
    <Badge
      variant="outline"
      color="blue"
      styles={{ root: { textTransform: "none", fontFamily: "monospace", letterSpacing: -0.3 } }}
    >
      {label}
    </Badge>
  );
}
