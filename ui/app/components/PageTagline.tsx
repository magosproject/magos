import { Group, Text } from "@mantine/core";
import BlinkingCursor from "./BlinkingCursor";

interface Props {
  text: string;
}

export default function PageTagline({ text }: Props) {
  return (
    <Group gap={4} align="center">
      <Text
        size="xl"
        fw={700}
        variant="gradient"
        gradient={{ from: "magos.4", to: "magos.7", deg: 45 }}
        style={{ fontFamily: "monospace", letterSpacing: -0.5 }}
      >
        {text}
      </Text>
      <BlinkingCursor />
    </Group>
  );
}

