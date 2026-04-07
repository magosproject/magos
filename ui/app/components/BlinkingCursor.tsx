import { Text } from "@mantine/core";
import { useBlinkVisible } from "../hooks/useBlinkVisible";

interface Props {
  size?: string;
  fw?: number;
}

export default function BlinkingCursor({ size = "xl", fw = 700 }: Props) {
  const visible = useBlinkVisible();
  return (
    <Text
      fw={fw}
      size={size}
      c="magos.5"
      style={{ fontFamily: "monospace", opacity: visible ? 1 : 0 }}
    >
      _
    </Text>
  );
}

