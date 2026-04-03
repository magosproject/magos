import { Badge } from "@mantine/core";
import { IconHexagon } from "@tabler/icons-react";

interface KubeBadgeProps {
  label: string;
}

export default function KubeBadge({ label }: KubeBadgeProps) {
  return (
    <Badge
      variant="outline"
      color="blue"
      leftSection={<IconHexagon size={12} />}
      styles={{ root: { textTransform: "none" } }}
    >
      {label}
    </Badge>
  );
}
