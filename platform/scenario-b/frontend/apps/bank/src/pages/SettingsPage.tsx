// SPDX-License-Identifier: Apache-2.0

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";

export function SettingsPage() {
  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Environment</CardTitle>
          <CardDescription>Frontend structural mode</CardDescription>
        </CardHeader>
        <CardContent>
        {/* These statements must describe what this portal actually does. The Bank portal
            reads no mock toggle and has no WebSocket client: every screen talks to the real
            API Gateway over HTTP. The bank manual's data-source line must agree with this. */}
        <ul className="list-disc pl-5 text-sm">
          <li>API mode: live backend — all screens call the real API Gateway</li>
          <li>Authentication: backend-managed HTTP-only cookies</li>
          <li>Updates: status is refreshed by polling the API Gateway</li>
        </ul>
        </CardContent>
      </Card>
    </div>
  );
}
