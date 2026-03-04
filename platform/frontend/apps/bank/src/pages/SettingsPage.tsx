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
        <ul className="list-disc pl-5 text-sm">
          <li>API mode: mock services enabled</li>
          <li>Authentication: backend-managed HTTP-only cookies</li>
          <li>WebSocket: simulated relay events for UI flow</li>
        </ul>
        </CardContent>
      </Card>
    </div>
  );
}
