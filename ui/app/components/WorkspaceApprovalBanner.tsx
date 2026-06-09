import { Alert, Button, Group, Stack, Text } from "@mantine/core";
import { IconCheck, IconX } from "@tabler/icons-react";
import { useState } from "react";
import apiClient from "../api/client";
import type { Workspace } from "../api/types";
import { isApprovalPending } from "../utils/phases";
import ApprovalDecisionModal, { type ApprovalMode } from "./ApprovalDecisionModal";

interface Props {
  workspace: Workspace;
}

export default function WorkspaceApprovalBanner({ workspace }: Props) {
  const [modal, setModal] = useState<ApprovalMode | null>(null);

  const namespace = workspace.metadata?.namespace;
  const name = workspace.metadata?.name;
  const runID = workspace.status?.currentRunID;
  const phase = workspace.status?.phase;
  const decision = workspace.metadata?.annotations?.["magosproject.io/approval-decision"];

  const pending = isApprovalPending(workspace);
  const transientApproved = phase === "Planned" && decision === "approved";
  const transientRejected = phase === "Planned" && decision === "rejected";
  const finalRejected = phase === "Rejected";

  if (!pending && !transientApproved && !transientRejected && !finalRejected) return null;
  if (!namespace || !name || !runID) return null;

  async function submit(mode: ApprovalMode, reason: string) {
    const path =
      mode === "approve"
        ? "/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs/{runID}/approve"
        : "/apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs/{runID}/reject";

    const { error } = await apiClient.POST(path, {
      params: { path: { namespace: namespace!, name: name!, runID: runID! } },
      body: { reason },
    });
    if (error) {
      throw new Error(typeof error === "string" ? error : "Decision failed");
    }
  }

  if (transientApproved) {
    return (
      <Alert color="green" icon={<IconCheck size={16} />} title="Approved, applying soon">
        <Text size="sm" c="dimmed">
          Run <code>{runID}</code> · decision recorded
        </Text>
      </Alert>
    );
  }

  if (transientRejected || finalRejected) {
    return (
      <Alert color="red" icon={<IconX size={16} />} title="Plan rejected">
        <Text size="sm" c="dimmed">
          Run <code>{runID}</code> · see run history for details
        </Text>
      </Alert>
    );
  }

  return (
    <>
      <Alert color="orange" title="Plan ready, waiting for approval">
        <Stack gap="xs">
          <Text size="sm" c="dimmed">
            Run <code>{runID}</code>
          </Text>
          <Group gap="xs">
            <Button
              color="green"
              leftSection={<IconCheck size={16} />}
              size="sm"
              onClick={() => setModal("approve")}
            >
              Approve
            </Button>
            <Button
              color="red"
              variant="light"
              leftSection={<IconX size={16} />}
              size="sm"
              onClick={() => setModal("reject")}
            >
              Reject
            </Button>
          </Group>
        </Stack>
      </Alert>

      {modal && (
        <ApprovalDecisionModal
          opened
          mode={modal}
          runID={runID}
          onClose={() => setModal(null)}
          onSubmit={(reason) => submit(modal, reason)}
        />
      )}
    </>
  );
}
