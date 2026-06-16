import {
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@cbweb3/ui";
import { RefreshCw } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { nocBackendApi } from "../services/api";
import type { NocContainerLog } from "../types";

export function LogViewerPage() {
  const { componentId } = useParams<{ componentId: string }>();
  const [searchParams] = useSearchParams();
  const componentName = searchParams.get("name") ?? componentId ?? "Component";

  const [logs, setLogs] = useState<NocContainerLog[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const fetchLogs = async () => {
    if (!componentId) return;
    setLoading(true);
    setError(null);
    try {
      const data = await nocBackendApi.getComponentLogs(componentId, 500);
      setLogs(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load logs");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchLogs();
    const interval = setInterval(() => void fetchLogs(), 15_000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [componentId]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  return (
    <Card className="flex flex-col h-full">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 shrink-0">
        <div>
          <CardTitle>Container Logs</CardTitle>
          <CardDescription>{componentName}</CardDescription>
        </div>
        <Button variant="outline" size="sm" onClick={() => void fetchLogs()} disabled={loading}>
          <RefreshCw className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </CardHeader>
      <CardContent className="flex-1 overflow-hidden">
        {error && <p className="mb-2 text-sm text-destructive">{error}</p>}
        <div className="h-[calc(100vh-220px)] overflow-y-auto rounded-md border bg-black p-3 font-mono text-xs text-green-400">
          {logs.length === 0 && !loading && (
            <span className="text-muted-foreground">No logs available.</span>
          )}
          {logs.map((log) => (
            <div key={log.id} className="flex gap-2 leading-5">
              <span className="shrink-0 text-muted-foreground">
                {new Date(log.occurred_at).toISOString().slice(11, 23)}
              </span>
              <Badge
                variant={log.stream === "stderr" ? "destructive" : "secondary"}
                className="h-4 shrink-0 px-1 py-0 text-[10px]"
              >
                {log.stream}
              </Badge>
              <span className={log.stream === "stderr" ? "text-red-400" : ""}>{log.log_line}</span>
            </div>
          ))}
          <div ref={bottomRef} />
        </div>
      </CardContent>
    </Card>
  );
}
