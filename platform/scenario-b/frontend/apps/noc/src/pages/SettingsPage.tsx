import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
  Checkbox,
  Input,
  Label,
  toast,
} from "@cbweb3/ui";
import { useState } from "react";

export function SettingsPage() {
  const [email, setEmail] = useState("noc-ops@cbweb3.local");
  const [criticalOnly, setCriticalOnly] = useState(true);
  const [refreshSeconds, setRefreshSeconds] = useState("5");

  const handleSave = () => {
    toast.success("NOC settings updated");
  };

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>Settings</CardTitle>
        <CardDescription>Configure operator notifications and data refresh behavior.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="alert-email">Alert Email</Label>
          <Input id="alert-email" value={email} onChange={(event) => setEmail(event.target.value)} />
        </div>
        <div className="flex items-center justify-between rounded-md border border-border p-3">
          <div>
            <p className="text-sm font-medium">Critical Alerts Only</p>
            <p className="text-xs text-muted-foreground">Send notifications only for CRITICAL incidents.</p>
          </div>
          <Checkbox checked={criticalOnly} onCheckedChange={(value) => setCriticalOnly(value === true)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="poll-interval">Polling Interval (seconds)</Label>
          <Input
            id="poll-interval"
            type="number"
            min={1}
            value={refreshSeconds}
            onChange={(event) => setRefreshSeconds(event.target.value)}
          />
        </div>
      </CardContent>
      <CardFooter>
        <Button onClick={handleSave}>Save Settings</Button>
      </CardFooter>
    </Card>
  );
}
