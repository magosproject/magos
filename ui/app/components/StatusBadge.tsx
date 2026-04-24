import { Badge, Group } from "@mantine/core";
import { IconRefresh } from "@tabler/icons-react";
import { statusColorFor } from "../utils/colors";
import { isPhase, SPINNING_PHASES } from "../utils/phases";

export const spinningStatuses = SPINNING_PHASES;

interface Props {
  status: string;
  size?: string;
}

export default function StatusBadge({ status, size = "sm" }: Props) {
  const color = statusColorFor(status);
  const spinning = isPhase(status) && SPINNING_PHASES.has(status);

  return (
    <Badge
      color={color}
      variant="light"
      size={size}
    >
      <Group gap={4} wrap="nowrap" align="center">
        {spinning && (
          <span className="spin">
            <IconRefresh size={10} />
          </span>
        )}
        {status}
      </Group>
    </Badge>
  );
}
