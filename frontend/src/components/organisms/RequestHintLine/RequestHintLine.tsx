import { Text } from "@/components/atoms/Text";

import styles from "./RequestHintLine.module.css";

export interface RequestHintLineProps {
  /** The request the screen was rendered from, for example GET /payment-references?status=paid */
  request: string;
}

/**
 * Shows the API call behind the screen, a detail the reference design kept at the foot
 * of the table. It doubles as a hint that the filters really are the query string.
 */
export function RequestHintLine({ request }: RequestHintLineProps) {
  return (
    <Text size="xs" muted technical className={styles.line}>
      {request}
    </Text>
  );
}
