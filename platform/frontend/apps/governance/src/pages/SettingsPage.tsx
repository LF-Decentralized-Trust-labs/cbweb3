import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Checkbox,
  Input,
  Label,
  toast,
} from "@cbweb3/ui";
import { useState } from "react";
import { useAuth } from "../hooks";

export function SettingsPage() {
  const { user } = useAuth();
  const [timezone, setTimezone] = useState("America/Sao_Paulo");
  const [dateFormat, setDateFormat] = useState("dd/MM/yyyy HH:mm");
  const [soundAlerts, setSoundAlerts] = useState(true);

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>Settings</CardTitle>
        <CardDescription>Operator session and display preferences.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="rounded-md border border-border p-3 text-sm">
          <p className="font-medium">Current Session</p>
          <p className="text-muted-foreground">User: {user?.username ?? "—"}</p>
          <p className="text-muted-foreground">Role: {user?.role ?? "—"}</p>
        </div>

        <div className="space-y-2">
          <Label>Timezone</Label>
          <Input value={timezone} onChange={(event) => setTimezone(event.target.value)} />
        </div>

        <div className="space-y-2">
          <Label>Date format</Label>
          <Input value={dateFormat} onChange={(event) => setDateFormat(event.target.value)} />
        </div>

        <div className="flex items-center gap-2 rounded-md border border-border p-3">
          <Checkbox checked={soundAlerts} onCheckedChange={(value) => setSoundAlerts(value === true)} />
          <span className="text-sm">Enable dashboard alert sounds</span>
        </div>

        <Button onClick={() => toast.success("Settings saved")}>Save Settings</Button>
      </CardContent>
    </Card>
  );
}
