import "@xyflow/react/dist/style.css";
import {
  Background,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  useEdgesState,
  useNodesState,
} from "@xyflow/react";
import type { Node, Edge, NodeProps } from "@xyflow/react";
import { Badge, Card, CardContent, CardHeader, CardTitle } from "@cbweb3/ui";
import { useEffect, useMemo } from "react";
import { useTopology } from "../hooks";
import type { TopologyNode } from "../types";

// ─── Custom node component ────────────────────────────────────────────────────

function ComponentNode({ data }: NodeProps) {
  const node = data as TopologyNode & { __typename?: string };
  const statusVariant =
    node.status === "HEALTHY" ? "default" : node.status === "DEGRADED" ? "warning" : "destructive";
  const kindColor: Record<string, string> = {
    BESU: "bg-blue-500",
    PALADIN: "bg-purple-500",
    CACTI: "bg-orange-500",
  };
  const dot = kindColor[node.kind as string] ?? "bg-gray-400";

  return (
    <div className="rounded-lg border border-border bg-card px-3 py-2 shadow-sm min-w-[140px]">
      <Handle type="target" position={Position.Top} />
      <div className="flex items-center gap-2 mb-1">
        <span className={`h-2 w-2 rounded-full flex-shrink-0 ${dot}`} />
        <span className="text-xs font-semibold truncate">{node.label}</span>
      </div>
      <div className="flex items-center justify-between gap-2">
        <span className="text-[10px] text-muted-foreground">{node.kind}</span>
        <Badge variant={statusVariant} className="text-[10px] px-1 py-0">{node.status}</Badge>
      </div>
      <div className="text-[9px] text-muted-foreground mt-0.5 truncate">{node.spoke_name}</div>
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}

const nodeTypes = { component: ComponentNode };

// ─── Layout helpers ───────────────────────────────────────────────────────────

const X_SPACING = 200;
const Y_SPACING = 160;
const COL_WIDTH = 220;

function buildLayout(topologyNodes: TopologyNode[]): Node[] {
  // Group by spoke, then assign (x, y) positions.
  const groups = new Map<string, TopologyNode[]>();
  for (const n of topologyNodes) {
    const key = n.spoke_id;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(n);
  }

  const flowNodes: Node[] = [];
  let spokeCol = 0;
  for (const [, members] of groups) {
    members.forEach((n, row) => {
      flowNodes.push({
        id: n.id,
        type: "component",
        position: { x: spokeCol * (COL_WIDTH + X_SPACING), y: row * Y_SPACING },
        data: n as unknown as Record<string, unknown>,
      });
    });
    spokeCol++;
  }
  return flowNodes;
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export function TopologyPage() {
  const { nodes: topologyNodes, edges: topologyEdges, fetch } = useTopology();

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const initialNodes = useMemo(() => buildLayout(topologyNodes), [topologyNodes]);
  const initialEdges = useMemo<Edge[]>(() =>
    topologyEdges.map((e) => ({
      id: e.id,
      source: e.from,
      target: e.to,
      style: { stroke: e.healthy ? "#22c55e" : "#ef4444", strokeWidth: 2 },
    })),
    [topologyEdges],
  );

  const [flowNodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [flowEdges, setEdges, onEdgesChange] = useEdgesState(initialEdges);

  useEffect(() => { setNodes(initialNodes); }, [initialNodes, setNodes]);
  useEffect(() => { setEdges(initialEdges); }, [initialEdges, setEdges]);

  const healthyLinks = topologyEdges.filter((e) => e.healthy).length;

  return (
    <div className="grid gap-4 xl:grid-cols-4">
      <div className="xl:col-span-3 h-[600px] rounded-lg border border-border overflow-hidden">
        <ReactFlow
          nodes={flowNodes}
          edges={flowEdges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.2 }}
        >
          <Background />
          <Controls />
          <MiniMap nodeColor={(n) => {
            const kind = (n.data as TopologyNode).kind;
            if (kind === "BESU") return "#3b82f6";
            if (kind === "PALADIN") return "#a855f7";
            if (kind === "CACTI") return "#f97316";
            return "#94a3b8";
          }} />
        </ReactFlow>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Topology Health</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div className="flex items-center justify-between">
            <span>Total Nodes</span>
            <Badge variant="secondary">{topologyNodes.length}</Badge>
          </div>
          <div className="flex items-center justify-between">
            <span>Total Links</span>
            <Badge variant="secondary">{topologyEdges.length}</Badge>
          </div>
          <div className="flex items-center justify-between">
            <span>Healthy Links</span>
            <Badge variant="default">{healthyLinks}</Badge>
          </div>
          <div className="flex items-center justify-between">
            <span>Unhealthy Links</span>
            <Badge variant="destructive">{topologyEdges.length - healthyLinks}</Badge>
          </div>
          <div className="mt-4 space-y-1">
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Legend</p>
            <div className="flex items-center gap-2 text-xs"><span className="h-2 w-2 rounded-full bg-blue-500" /> BESU</div>
            <div className="flex items-center gap-2 text-xs"><span className="h-2 w-2 rounded-full bg-purple-500" /> PALADIN</div>
            <div className="flex items-center gap-2 text-xs"><span className="h-2 w-2 rounded-full bg-orange-500" /> CACTI Relay</div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

