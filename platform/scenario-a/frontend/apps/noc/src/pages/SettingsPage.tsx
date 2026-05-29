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
import { useUiStore } from "../stores";

export function SettingsPage() {
  const { muteAlerts, fallbackPollingSeconds, setMuteAlerts, setFallbackPollingSeconds } = useUiStore();
  const [email, setEmail] = useState("noc-ops@cbweb3.local");
  const [localInterval, setLocalInterval] = useState(String(fallbackPollingSeconds));

  const handleSave = () => {
    const parsed = parseInt(localInterval, 10);
    if (!isNaN(parsed) && parsed >= 5) {
      setFallbackPollingSeconds(parsed);
    }
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
          <Checkbox checked={muteAlerts} onCheckedChange={(value) => setMuteAlerts(value === true)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="poll-interval">Polling Interval (seconds, min 5)</Label>
          <Input
            id="poll-interval"
            type="number"
            min={5}
            value={localInterval}
            onChange={(event) => setLocalInterval(event.target.value)}
          />
          <p className="text-xs text-muted-foreground">Current active interval: {fallbackPollingSeconds}s</p>
        </div>
      </CardContent>
      <CardFooter>
        <Button onClick={handleSave}>Save Settings</Button>
      </CardFooter>
    </Card>
  );
}
