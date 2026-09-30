import { Card, CardKicker, CardMeta } from "@/components/atoms/Card";
import { Figure } from "@/components/atoms/Heading";

import styles from "./StatCard.module.css";

export interface StatCardProps {
  label: string;
  value: string | number;
  meta: string;
}

export function StatCard({ label, value, meta }: StatCardProps) {
  return (
    <Card className={styles.card}>
      <CardKicker>{label}</CardKicker>
      <Figure>{value}</Figure>
      <CardMeta>{meta}</CardMeta>
    </Card>
  );
}
