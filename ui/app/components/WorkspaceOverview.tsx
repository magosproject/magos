import { Anchor, Badge, Group, SimpleGrid, Text } from "@mantine/core";
import { IconFolder, IconGitBranch } from "@tabler/icons-react";
import type { CSSProperties } from "react";
import { Link } from "react-router";
import type { Workspace } from "../api/types";
import { formatDateTime } from "../utils/formatDateTime";
import InfoCard from "./InfoCard";
import StatusBadge from "./StatusBadge";
import { repoIcon } from "../utils/repoIcon";
import { commitUrl, revisionUrl, terraformReleaseUrl } from "../utils/repoUrls";

function formatDate(iso: string) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "medium" });
}

function AppliedRevisionValue({
  repoURL,
  observedRevision,
}: {
  repoURL: string;
  observedRevision: string;
}) {
  if (!observedRevision)
    return (
      <Text size="sm" c="dimmed">
        —
      </Text>
    );

  const isSHA = observedRevision.length === 40;
  if (isSHA) {
    const href = commitUrl(repoURL, observedRevision);
    return href ? (
      <Anchor href={href} target="_blank" size="sm" ff="monospace">
        {observedRevision.slice(0, 7)}
      </Anchor>
    ) : (
      <Text size="sm" c="dimmed" ff="monospace">
        {observedRevision.slice(0, 7)}
      </Text>
    );
  }

  const href = revisionUrl(repoURL, observedRevision);
  return href ? (
    <Anchor href={href} target="_blank" size="sm">
      {observedRevision}
    </Anchor>
  ) : (
    <Text size="sm" c="dimmed">
      {observedRevision}
    </Text>
  );
}

function TerraformVersionValue({ version }: { version: string }) {
  if (!version)
    return (
      <Text size="sm" c="dimmed">
        —
      </Text>
    );

  const href = terraformReleaseUrl(version);
  return href ? (
    <Anchor href={href} target="_blank" size="sm">
      {version}
    </Anchor>
  ) : (
    <Text size="sm" c="dimmed">
      {version}
    </Text>
  );
}

interface WorkspaceOverviewProps {
  namespace: string;
  workspace: Workspace;
  phaseLabel: string;
  projectName: string;
  flash?: boolean;
  flashStyle?: CSSProperties;
}

export default function WorkspaceOverview({
  namespace,
  workspace,
  phaseLabel,
  projectName,
  flash,
  flashStyle,
}: WorkspaceOverviewProps) {
  const repoURL = workspace.spec?.source?.repoURL ?? "";
  const observedRevision = workspace.status?.observedRevision ?? "";
  const tfVersion = workspace.spec?.terraform?.version ?? "";

  return (
    <SimpleGrid cols={{ base: 1, sm: 2, md: 3 }} spacing="md">
      <InfoCard label="Status" className={flash ? "flash-highlight" : undefined} style={flashStyle}>
        <StatusBadge status={phaseLabel} size="md" />
      </InfoCard>

      <InfoCard label="Project">
        {projectName ? (
          <Anchor component={Link} to={`/projects/${namespace}/${projectName}`} size="sm">
            {projectName}
          </Anchor>
        ) : (
          <Text size="sm" c="dimmed" fs="italic">
            None
          </Text>
        )}
      </InfoCard>

      <InfoCard label="Repository">
        <Group gap={6} wrap="nowrap">
          {repoIcon(repoURL, 14)}
          <Anchor href={repoURL} target="_blank" size="sm" truncate>
            {repoURL.replace(/^https?:\/\//, "")}
          </Anchor>
        </Group>
      </InfoCard>

      <InfoCard label="Path">
        <Group gap={6} wrap="nowrap">
          <IconFolder size={14} />
          <Text size="sm" c="dimmed" truncate>
            {workspace.spec?.source?.path ?? "—"}
          </Text>
        </Group>
      </InfoCard>

      <InfoCard label="Applied Ref">
        <Group gap={6} wrap="nowrap">
          <IconGitBranch size={14} />
          <AppliedRevisionValue repoURL={repoURL} observedRevision={observedRevision} />
        </Group>
      </InfoCard>

      <InfoCard label="Terraform Version">
        <TerraformVersionValue version={tfVersion} />
      </InfoCard>

      <InfoCard label="Auto Apply">
        <Badge color={workspace.spec?.autoApply ? "magos" : "gray"} variant="light" size="sm">
          {workspace.spec?.autoApply ? "enabled" : "disabled"}
        </Badge>
      </InfoCard>

      {workspace.status?.lastReconcileTime && (
        <InfoCard label="Last reconcile">
          <Text size="sm">{formatDateTime(workspace.status.lastReconcileTime)}</Text>
        </InfoCard>
      )}

      {workspace.status?.nextReconcileTime && (
        <InfoCard label="Next reconcile">
          <Text size="sm">{formatDateTime(workspace.status.nextReconcileTime)}</Text>
        </InfoCard>
      )}
    </SimpleGrid>
  );
}
