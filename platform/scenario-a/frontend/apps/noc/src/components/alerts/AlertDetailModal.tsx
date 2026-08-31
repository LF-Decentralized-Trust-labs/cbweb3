// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  Separator,
} from "@cbweb3/ui";
import { CheckCheck, XCircle } from "lucide-react";
import { useEffect } from "react";
import { useAlertStore } from "../../stores";

const severityVariant: Record<string, "default" | "secondary" | "warning" | "destructive"> = {
  INFO: "secondary",
  WARNING: "warning",
  HIGH: "destructive",
  CRITICAL: "destructive",
};

const healthVariant: Record<string, "default" | "secondary" | "warning" | "destructive"> = {
  HEALTHY: "default",
  DEGRADED: "warning",
  OFFLINE: "destructive",
  UNKNOWN: "secondary",
};

type Props = {
  alertId: string | null;
  onClose: () => void;
};

export function AlertDetailModal({ alertId, onClose }: Props) {
  const { selectedAlert, selectedAlertStatus, selectAlert, acknowledgeAlert, dismissAlert } =
    useAlertStore();

  useEffect(() => {
    if (alertId) void selectAlert(alertId);
  }, [alertId, selectAlert]);

  const open = Boolean(alertId);
  const loading = selectedAlertStatus === "loading";

  async function handleAcknowledge() {
    if (!selectedAlert) return;
    await acknowledgeAlert(selectedAlert.id);
  }

  async function handleDismiss() {
    if (!selectedAlert) return;
    await dismissAlert(selectedAlert.id);
    onClose();
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Alert Details</DialogTitle>
        </DialogHeader>

        {loading && <p className="text-sm text-muted-foreground py-4 text-center">Loading…</p>}

        {!loading && selectedAlert && (
          <div className="space-y-4 text-sm">
            {/* Title + badges */}
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-semibold flex-1">{selectedAlert.title}</span>
              <Badge variant={severityVariant[selectedAlert.severity] ?? "secondary"}>
                {selectedAlert.severity}
              </Badge>
              <Badge variant={selectedAlert.state === "ACTIVE" ? "destructive" : "default"}>
                {selectedAlert.state}
              </Badge>
              {selectedAlert.acknowledged_by && (
                <Badge variant="secondary">ACK: {selectedAlert.acknowledged_by}</Badge>
              )}
            </div>

            <Separator />

            {/* Timestamps */}
            <div className="grid grid-cols-2 gap-x-4 gap-y-1">
              <span className="text-muted-foreground">Created</span>
              <span>{new Date(selectedAlert.created_at).toLocaleString()}</span>
              {selectedAlert.resolved_at && (
                <>
                  <span className="text-muted-foreground">Resolved</span>
                  <span>{new Date(selectedAlert.resolved_at).toLocaleString()}</span>
                </>
              )}
              <span className="text-muted-foreground">Root cause sig</span>
              <span className="font-mono text-xs break-all">{selectedAlert.root_cause_sig}</span>
            </div>

            <Separator />

            {/* Component info */}
            <div>
              <p className="font-medium mb-2">Component</p>
              <div className="rounded-md border border-border p-3 space-y-1">
                <div className="flex items-center justify-between">
                  <span className="font-medium">{selectedAlert.component.name}</span>
                  <Badge variant={healthVariant[selectedAlert.component.health_status] ?? "secondary"}>
                    {selectedAlert.component.health_status}
                  </Badge>
                </div>
                <p className="text-muted-foreground text-xs">{selectedAlert.component.type}</p>
                <p className="text-muted-foreground text-xs font-mono">{selectedAlert.component.endpoint}</p>
                {selectedAlert.component.last_checked_at && (
                  <p className="text-muted-foreground text-xs">
                    Last checked: {new Date(selectedAlert.component.last_checked_at).toLocaleString()}
                  </p>
                )}
                {selectedAlert.component.last_block_number != null && (
                  <p className="text-muted-foreground text-xs">
                    Block: #{selectedAlert.component.last_block_number}
                  </p>
                )}
              </div>
            </div>

            {/* Actions */}
            {selectedAlert.state === "ACTIVE" && (
              <div className="flex gap-2 justify-end pt-2">
                {!selectedAlert.acknowledged_by && (
                  <Button variant="outline" size="sm" onClick={() => void handleAcknowledge()}>
                    <CheckCheck className="h-3.5 w-3.5 mr-1" />
                    Acknowledge
                  </Button>
                )}
                <Button variant="destructive" size="sm" onClick={() => void handleDismiss()}>
                  <XCircle className="h-3.5 w-3.5 mr-1" />
                  Dismiss
                </Button>
              </div>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
