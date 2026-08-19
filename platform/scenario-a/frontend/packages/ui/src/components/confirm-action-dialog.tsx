// SPDX-License-Identifier: Apache-2.0

import { Button } from "./button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./dialog";

export type ConfirmActionField = {
  label: string;
  value: string;
};

type ConfirmActionDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  /** One sentence saying what is about to happen, and whether it can be undone. */
  description: string;
  /**
   * The values that will actually be submitted. Showing them is the point: the
   * operator confirms the request the server will receive, not a summary of it.
   */
  fields: ConfirmActionField[];
  confirmLabel: string;
  cancelLabel?: string;
  /** Marks an irreversible action; renders the confirm button as destructive. */
  destructive?: boolean;
  busy?: boolean;
  onConfirm: () => void;
};

/**
 * A confirmation step for an action that moves money.
 *
 * Mint, burn and swap used to execute on a single click (finding R2-M-8), and a
 * burn cannot be undone on-chain — reversing one means a fresh, separately
 * authorised issuance. The amount field is free text, so a stray zero destroyed
 * ten times the intended supply with no step in between.
 */
export function ConfirmActionDialog({
  open,
  onOpenChange,
  title,
  description,
  fields,
  confirmLabel,
  cancelLabel = "Cancel",
  destructive = false,
  busy = false,
  onConfirm,
}: ConfirmActionDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <dl className="grid gap-2 rounded-md border border-border bg-muted/40 p-3 text-sm">
          {fields.map((field) => (
            <div key={field.label} className="flex items-start justify-between gap-4">
              <dt className="text-muted-foreground">{field.label}</dt>
              <dd className="font-medium break-all text-right">{field.value}</dd>
            </div>
          ))}
        </dl>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
            {cancelLabel}
          </Button>
          <Button variant={destructive ? "destructive" : "default"} onClick={onConfirm} disabled={busy}>
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
