// SPDX-License-Identifier: Apache-2.0

import { Card, CardContent, CardDescription, CardHeader, CardTitle, Separator } from "@cbweb3/ui";

export function SettingsPage() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Security Settings</CardTitle>
        <CardDescription>Treasury portal operational security profile.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <div>
          <p className="font-medium">RBAC</p>
          <p className="text-muted-foreground">Route and action guards require TREASURY role claim.</p>
        </div>
        <Separator />
        <div>
          <p className="font-medium">Credential Policy</p>
          <p className="text-muted-foreground">Mint operations require approved funding request and valid issuer credentials.</p>
        </div>
        <Separator />
        <div>
          <p className="font-medium">Session Handling</p>
          <p className="text-muted-foreground">No local storage/session storage used for auth tokens or sensitive payloads.</p>
        </div>
      </CardContent>
    </Card>
  );
}
