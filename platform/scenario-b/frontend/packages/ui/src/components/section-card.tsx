// SPDX-License-Identifier: Apache-2.0

import type { PropsWithChildren } from "react";
import { cn } from "../lib/utils";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./card";

type SectionCardProps = PropsWithChildren<{
  title: string;
  description?: string;
  className?: string;
}>;

export function SectionCard({ title, description, className, children }: SectionCardProps) {
  return (
    <Card className={cn(className)}>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description ? <CardDescription>{description}</CardDescription> : null}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}
