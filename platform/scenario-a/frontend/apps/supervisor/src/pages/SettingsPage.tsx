// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Checkbox, toast } from "@cbweb3/ui";
import { useState } from "react";

export function SettingsPage() {
  const [enableRealtime, setEnableRealtime] = useState(true);
  const [strictSession, setStrictSession] = useState(true);
  const [maskSensitive, setMaskSensitive] = useState(true);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Supervisor Settings</CardTitle>
          <CardDescription>Session security and operational preferences.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between rounded-md border border-border p-3">
            <div>
              <p className="font-medium">Realtime Telemetry (SSE)</p>
              <p className="text-xs text-muted-foreground">Receive imbalance and governance alerts instantly.</p>
            </div>
            <Checkbox checked={enableRealtime} onCheckedChange={(checked) => setEnableRealtime(Boolean(checked))} />
          </div>

          <div className="flex items-center justify-between rounded-md border border-border p-3">
            <div>
              <p className="font-medium">Strict Session Management</p>
              <p className="text-xs text-muted-foreground">Require periodic re-authentication for privileged operations.</p>
            </div>
            <Checkbox checked={strictSession} onCheckedChange={(checked) => setStrictSession(Boolean(checked))} />
          </div>

          <div className="flex items-center justify-between rounded-md border border-border p-3">
            <div>
              <p className="font-medium">Mask Sensitive Data</p>
              <p className="text-xs text-muted-foreground">Prevent accidental exposure of decrypted payloads in UI surfaces.</p>
            </div>
            <Checkbox checked={maskSensitive} onCheckedChange={(checked) => setMaskSensitive(Boolean(checked))} />
          </div>

          <div className="flex items-center gap-3">
            <Button
              onClick={() => {
                toast("Settings saved", { description: "Supervisor preferences updated for this session." });
              }}
            >
              Save Preferences
            </Button>
            <Badge variant="outline">Session-only preferences</Badge>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
