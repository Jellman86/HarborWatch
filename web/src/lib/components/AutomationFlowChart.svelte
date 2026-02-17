<script lang="ts">
    import { onMount, tick } from "svelte";

    interface FlowStep {
        label: string;
        state?: "active" | "idle" | "warning";
    }

    interface FlowNode {
        key: string;
        kind: "terminal" | "step";
        label: string;
        logicalIndex: number;
        stepNumber?: number;
        state?: "active" | "idle" | "warning";
    }

    interface PositionedFlowNode extends FlowNode {
        row: number;
        col: number;
    }

    interface NodeGeometry {
        x: number;
        y: number;
        top: number;
        right: number;
        bottom: number;
        left: number;
        row: number;
    }

    let { title, subtitle = "", steps = [], accent = "#0ea5e9" } = $props<{
        title: string;
        subtitle?: string;
        steps: FlowStep[];
        accent?: string;
    }>();

    const NODE_MIN_WIDTH = 164;
    const GRID_COL_GAP = 14;
    const GRID_ROW_GAP = 26;
    const MAX_SINGLE_ROW_NODES = 6;
    const CONNECTOR_GAP_X = 10;
    const CONNECTOR_GAP_Y = 6;

    let host = $state<HTMLDivElement | null>(null);
    let hostWidth = $state(0);
    let connectorPaths = $state<string[]>([]);
    let connectorMarkerId = $derived(`flow-arrow-${title.toLowerCase().replace(/[^a-z0-9]+/g, "-") || "default"}`);

    let flowNodes = $derived<FlowNode[]>([
        { key: "start", kind: "terminal", label: "Start", logicalIndex: 0 },
        ...steps.map((step, idx) => ({
            key: `step-${idx}`,
            kind: "step" as const,
            label: step.label,
            logicalIndex: idx + 1,
            stepNumber: idx + 1,
            state: step.state
        })),
        { key: "end", kind: "terminal", label: "End", logicalIndex: steps.length + 1 }
    ]);

    let singleRowCapacity = $derived(
        Math.min(
            MAX_SINGLE_ROW_NODES,
            Math.max(3, Math.floor((hostWidth + GRID_COL_GAP) / (NODE_MIN_WIDTH + GRID_COL_GAP)))
        )
    );
    let useTwoRows = $derived(flowNodes.length > singleRowCapacity);
    let columns = $derived(useTwoRows ? Math.ceil(flowNodes.length / 2) : flowNodes.length);

    let positionedNodes = $derived<PositionedFlowNode[]>((() => {
        const cols = Math.max(columns, 1);
        if (!useTwoRows) {
            return flowNodes.map((node, idx) => ({ ...node, row: 0, col: idx }));
        }

        const firstRowCount = cols;
        const firstRow = flowNodes.slice(0, firstRowCount).map((node, idx) => ({ ...node, row: 0, col: idx }));
        const secondRow = flowNodes
            .slice(firstRowCount)
            .map((node, idx, arr) => ({ ...node, row: 1, col: arr.length - 1 - idx }));
        return [...firstRow, ...secondRow];
    })());

    let gridStyle = $derived(
        `grid-template-columns: repeat(${Math.max(columns, 1)}, minmax(0, 1fr)); column-gap: ${GRID_COL_GAP}px; row-gap: ${GRID_ROW_GAP}px;`
    );

    function nodeClasses(kind: FlowNode["kind"], state?: string): string {
        if (kind === "terminal") {
            return "border-slate-300 bg-white text-slate-600 dark:bg-slate-900 dark:border-slate-600 dark:text-slate-300";
        }
        switch (state) {
            case "active":
                return "border-transparent text-white shadow-lg";
            case "warning":
                return "border-amber-200 bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:border-amber-900/40 dark:text-amber-300";
            default:
                return "border-slate-200 bg-white text-slate-500 dark:bg-slate-900/40 dark:border-slate-700 dark:text-slate-300";
        }
    }

    function badgeClasses(state?: string): string {
        switch (state) {
            case "active":
                return "bg-white/20 text-white";
            case "warning":
                return "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300";
            default:
                return "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300";
        }
    }

    function statusLabel(state?: string): string {
        if (state === "active") return "active";
        if (state === "warning") return "partial";
        return "idle";
    }

    function nodeStyle(node: PositionedFlowNode): string {
        const placement = `grid-column:${node.col + 1}; grid-row:${node.row + 1};`;
        if (node.kind === "step" && node.state === "active") {
            return `${placement} background:${accent};`;
        }
        return placement;
    }

    function clamp(value: number, min: number, max: number): number {
        return Math.max(min, Math.min(max, value));
    }

    async function updateGeometry() {
        const width = host?.getBoundingClientRect().width ?? 0;
        hostWidth = Number.isFinite(width) ? Math.max(Math.floor(width), 0) : 0;
        if (!host || flowNodes.length <= 1) {
            connectorPaths = [];
            return;
        }

        await tick();
        if (!host) {
            connectorPaths = [];
            return;
        }

        const base = host.getBoundingClientRect();
        const maxX = Math.max(base.width - 3, 3);
        const maxY = Math.max(base.height - 3, 3);
        const nodesByIndex = new Map<number, NodeGeometry>();
        const nodes = host.querySelectorAll<HTMLElement>("[data-logical-index]");
        nodes.forEach((el) => {
            const idx = Number(el.dataset.logicalIndex);
            if (!Number.isFinite(idx)) return;
            const box = el.getBoundingClientRect();
            const row = Number(el.dataset.row);
            const centerX = clamp(box.left - base.left + box.width / 2, 3, maxX);
            const centerY = clamp(box.top - base.top + box.height / 2, 3, maxY);
            nodesByIndex.set(idx, {
                x: centerX,
                y: centerY,
                left: clamp(box.left - base.left, 3, maxX),
                right: clamp(box.right - base.left, 3, maxX),
                top: clamp(box.top - base.top, 3, maxY),
                bottom: clamp(box.bottom - base.top, 3, maxY),
                row: Number.isFinite(row) ? row : 0
            });
        });

        const paths: string[] = [];
        for (let i = 0; i < flowNodes.length - 1; i++) {
            const from = nodesByIndex.get(i);
            const to = nodesByIndex.get(i + 1);
            if (!from || !to) continue;

            if (from.row === to.row) {
                const leftToRight = to.x >= from.x;
                const startX = clamp(leftToRight ? from.right + CONNECTOR_GAP_X : from.left - CONNECTOR_GAP_X, 3, maxX);
                const endX = clamp(leftToRight ? to.left - CONNECTOR_GAP_X : to.right + CONNECTOR_GAP_X, 3, maxX);
                if (Math.abs(endX - startX) > 2) {
                    paths.push(`M ${startX} ${from.y} L ${endX} ${to.y}`);
                } else {
                    paths.push(`M ${from.x} ${from.y} L ${to.x} ${to.y}`);
                }
            } else {
                const down = to.row > from.row;
                const startY = clamp(down ? from.bottom + CONNECTOR_GAP_Y : from.top - CONNECTOR_GAP_Y, 3, maxY);
                const endY = clamp(down ? to.top - CONNECTOR_GAP_Y : to.bottom + CONNECTOR_GAP_Y, 3, maxY);
                const midY = clamp((startY + endY) / 2, 3, maxY);
                paths.push(`M ${from.x} ${startY} L ${from.x} ${midY} L ${to.x} ${midY} L ${to.x} ${endY}`);
            }
        }
        connectorPaths = paths;
    }

    onMount(() => {
        void updateGeometry();
        const onResize = () => void updateGeometry();
        let observer: ResizeObserver | null = null;
        if (typeof ResizeObserver !== "undefined" && host) {
            observer = new ResizeObserver(() => void updateGeometry());
            observer.observe(host);
        }
        window.addEventListener("resize", onResize);
        return () => {
            observer?.disconnect();
            window.removeEventListener("resize", onResize);
        };
    });

    $effect(() => {
        positionedNodes;
        steps.length;
        void updateGeometry();
    });
</script>

<div class="space-y-3">
    <div>
        <h4 class="text-sm font-black text-slate-800 dark:text-slate-100 tracking-tight uppercase">{title}</h4>
        {#if subtitle}
            <p class="text-xs text-slate-500 mt-1">{subtitle}</p>
        {/if}
    </div>

    {#if steps.length > 0}
        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white/70 dark:bg-slate-900/25 p-3">
            <div class="relative w-full" bind:this={host}>
                {#if connectorPaths.length > 0}
                    <svg class="pointer-events-none absolute inset-0 h-full w-full" style={`color:${accent};`} aria-hidden="true">
                        <defs>
                            <marker id={connectorMarkerId} markerWidth="8" markerHeight="8" refX="6.6" refY="4" orient="auto" markerUnits="strokeWidth">
                                <path d="M 0 0 L 8 4 L 0 8 z" fill="currentColor"></path>
                            </marker>
                        </defs>
                        {#each connectorPaths as path}
                            <path d={path} fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" marker-end={`url(#${connectorMarkerId})`} opacity="0.58"></path>
                        {/each}
                    </svg>
                {/if}

                <div class="grid w-full items-stretch" style={gridStyle}>
                    {#each positionedNodes as node (node.key)}
                        <div
                            class="min-w-0 rounded-xl border px-3 py-3 transition-all {nodeClasses(node.kind, node.state)}"
                            style={nodeStyle(node)}
                            data-logical-index={node.logicalIndex}
                            data-row={node.row}
                        >
                            {#if node.kind === "terminal"}
                                <p class="text-[10px] font-black uppercase tracking-[0.18em] text-center">{node.label}</p>
                            {:else}
                                <p class="text-[9px] font-black uppercase tracking-[0.18em] opacity-80">Step {node.stepNumber}</p>
                                <p class="mt-1 text-[11px] font-bold uppercase tracking-wide">{node.label}</p>
                                <span class="mt-2 inline-block px-2 py-0.5 rounded-full text-[9px] font-black uppercase tracking-wider {badgeClasses(node.state)}">
                                    {statusLabel(node.state)}
                                </span>
                            {/if}
                        </div>
                    {/each}
                </div>
            </div>
        </div>
    {:else}
        <div class="h-[120px] rounded-xl border border-dashed border-slate-200 dark:border-slate-700 flex items-center justify-center text-xs italic text-slate-400">
            No flow steps configured.
        </div>
    {/if}
</div>
