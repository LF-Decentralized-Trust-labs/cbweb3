// SPDX-License-Identifier: Apache-2.0

import * as React from "react";
import * as ProgressPrimitive from "@radix-ui/react-progress";
import { cn } from "../lib/utils";

type ProgressProps = React.ComponentProps<typeof ProgressPrimitive.Root> & {
  value: number;
};

const Progress = React.forwardRef<
  React.ElementRef<typeof ProgressPrimitive.Root>,
  ProgressProps
>(({ value, className, ...props }, ref) => {
  const normalized = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0;

  return (
    <ProgressPrimitive.Root
      ref={ref}
      className={cn("relative h-2 w-full overflow-hidden rounded-full bg-secondary", className)}
      {...props}
    >
      <ProgressPrimitive.Indicator className="h-full w-full flex-1 bg-primary transition-all" style={{ transform: `translateX(-${100 - normalized}%)` }} />
    </ProgressPrimitive.Root>
  );
});
Progress.displayName = "Progress";

export { Progress };
