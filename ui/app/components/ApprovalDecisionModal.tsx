import { Button, Group, Modal, Stack, Text, Textarea } from "@mantine/core";
import { useState } from "react";

export type ApprovalMode = "approve" | "reject";

interface Props {
  opened: boolean;
  mode: ApprovalMode;
  runID: string;
  onClose: () => void;
  onSubmit: (reason: string) => Promise<void>;
}

const reasonMaxLen = 1024;

export default function ApprovalDecisionModal({ opened, mode, runID, onClose, onSubmit }: Props) {
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isReject = mode === "reject";
  const trimmed = reason.trim();
  const reasonInvalid = isReject && trimmed.length === 0;
  const overLimit = trimmed.length > reasonMaxLen;

  async function handleSubmit() {
    if (reasonInvalid || overLimit) return;
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit(trimmed);
      setReason("");
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Decision failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={isReject ? "Reject this plan?" : "Approve this plan?"}
      centered
    >
      <Stack gap="sm">
        <Text size="xs" c="dimmed">
          Run <code>{runID}</code>
        </Text>
        <Textarea
          label={`Reason ${isReject ? "(required)" : "(optional)"}`}
          placeholder={
            isReject
              ? "Explain why this plan should not be applied"
              : "Optional context for the audit trail"
          }
          minRows={3}
          value={reason}
          onChange={(e) => setReason(e.currentTarget.value)}
          error={
            overLimit
              ? `Reason must be at most ${reasonMaxLen} characters`
              : undefined
          }
        />
        {trimmed.length > 800 && (
          <Text size="xs" c={overLimit ? "red" : "dimmed"}>
            {trimmed.length} / {reasonMaxLen}
          </Text>
        )}
        {error && <Text c="red" size="sm">{error}</Text>}
        <Group justify="flex-end" gap="xs">
          <Button variant="default" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button
            color={isReject ? "red" : "green"}
            onClick={handleSubmit}
            loading={submitting}
            disabled={reasonInvalid || overLimit}
          >
            {isReject ? "Reject plan" : "Approve plan"}
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}
