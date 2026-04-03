import { Badge, Group } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { statusColor } from "../utils/colors";

interface Props {
  status: string;
  size?: string;
}

export default function StatusBadge({ status, size = "sm" }: Props) {
  return (
    <Badge color={statusColor[status]} variant="light" size={size}>
      <Group gap={4} wrap="nowrap" align="center">
        {status === "provisioning" && (
          <span className="spin">
            <IconRefresh size={10} />
          </span>
        )}
        {status}
      </Group>
    </Badge>
  );
}
