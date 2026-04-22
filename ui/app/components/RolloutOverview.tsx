import { Anchor, Group, SimpleGrid, Stack, Text } from "@mantine/core";
import type { CSSProperties } from "react";
import { Link } from "react-router";
import type { Rollout } from "../api/types";
import InfoCard from "./InfoCard";
import StatusBadge from "./StatusBadge";

interface RolloutOverviewProps {
  namespace: string;
  rollout: Rollout;
  completedSteps: number;
  totalSteps: number;
  flash?: boolean;
  flashStyle?: CSSProperties;
}

export default function RolloutOverview({
  namespace,
  rollout,
  completedSteps,
  totalSteps,
  flash,
  flashStyle,
}: RolloutOverviewProps) {
  return (
    <Stack gap="md">
      <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="md">
        <InfoCard label="Phase" className={flash ? "flash-highlight" : undefined} style={flashStyle}>
          <StatusBadge status={rollout.status?.phase ?? ""} size="md" />
        </InfoCard>
        <InfoCard label="Project">
          {rollout.spec?.projectRef ? (
            <Anchor component={Link} to={`/projects/${namespace}/${rollout.spec.projectRef}`} size="sm">
              {rollout.spec.projectRef}
            </Anchor>
          ) : (
            <Text size="sm" c="dimmed">—</Text>
          )}
        </InfoCard>
        <InfoCard label="Progress">
          <Text size="sm">
            {completedSteps}/{totalSteps} completed
          </Text>
        </InfoCard>
        {rollout.status?.reason && (
          <InfoCard label="Reason">
            <Text size="sm" c="dimmed">
              {rollout.status.reason}
            </Text>
          </InfoCard>
        )}
      </SimpleGrid>

      {rollout.status?.message && (
        <Text size="sm" c="dimmed" fs="italic">
          {rollout.status.message}
        </Text>
      )}
    </Stack>
  );
}
