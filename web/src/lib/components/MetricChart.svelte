<script lang="ts">
    import { onMount } from "svelte";
    import type { Metric } from "../api-types";

    let { metrics, title, type = "cpu" } = $props<{
        metrics: Metric[];
        title: string;
        type: "cpu" | "memory";
    }>();

    const CHART_HEIGHT = 250;
    const MIN_CHART_WIDTH = 260;
    const MAX_RENDER_POINTS = 240;
    const PAD_TOP = 16;
    const PAD_RIGHT = 14;
    const PAD_BOTTOM = 30;
    const PAD_LEFT = 54;

    const toFinite = (value: unknown): number | null => {
        const n = Number(value);
        return Number.isFinite(n) ? n : null;
    };

    const normalizeTimestamp = (value: unknown): number | null => {
        const ts = toFinite(value);
        if (ts === null || ts <= 0) return null;
        return ts > 1_000_000_000_000 ? ts : ts * 1000;
    };

    const formatBytes = (bytes: number) => {
        const safe = Number.isFinite(bytes) ? Math.max(bytes, 0) : 0;
        if (safe === 0) return "0 B";
        const k = 1024;
        const sizes = ["B", "KB", "MB", "GB", "TB"];
        const i = Math.min(Math.floor(Math.log(safe) / Math.log(k)), sizes.length - 1);
        return `${(safe / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
    };

    function downsamplePoints(points: Array<{ x: number; y: number }>, maxPoints: number): Array<{ x: number; y: number }> {
        if (points.length <= maxPoints) return points;
        const stride = Math.max(1, Math.ceil(points.length / maxPoints));
        const sampled: Array<{ x: number; y: number }> = [];
        for (let i = 0; i < points.length; i += stride) {
            sampled.push(points[i]);
        }
        const last = points[points.length - 1];
        if (sampled[sampled.length - 1] !== last) sampled.push(last);
        return sampled;
    }

    let host = $state<HTMLDivElement | null>(null);
    let hostWidth = $state(0);

    const measureHost = () => {
        const width = host?.getBoundingClientRect().width ?? 0;
        hostWidth = Number.isFinite(width) ? Math.max(Math.floor(width), 0) : 0;
    };

    onMount(() => {
        measureHost();
        const onResize = () => measureHost();
        let observer: ResizeObserver | null = null;
        if (typeof ResizeObserver !== "undefined" && host) {
            observer = new ResizeObserver(() => measureHost());
            observer.observe(host);
        }
        window.addEventListener("resize", onResize);
        return () => {
            observer?.disconnect();
            window.removeEventListener("resize", onResize);
        };
    });

    let chartWidth = $derived(Number.isFinite(hostWidth) ? Math.max(hostWidth, MIN_CHART_WIDTH) : MIN_CHART_WIDTH);
    let innerWidth = $derived(Math.max(chartWidth - PAD_LEFT - PAD_RIGHT, 1));
    let innerHeight = $derived(Math.max(CHART_HEIGHT - PAD_TOP - PAD_BOTTOM, 1));
    let baselineY = $derived(PAD_TOP + innerHeight);

    let safeMetrics = $derived(
        (metrics || [])
            .map((m: Metric) => {
                const x = normalizeTimestamp(m?.timestamp);
                const rawY = type === "cpu" ? toFinite(m?.cpuPercent) : toFinite(m?.memoryUsage);
                if (x === null || rawY === null) return null;
                const boundedY = type === "cpu"
                    ? Math.max(0, Math.min(100, rawY))
                    : Math.max(0, rawY);
                return { x, y: boundedY };
            })
            .filter((point: { x: number; y: number } | null): point is { x: number; y: number } => point !== null)
            .sort((a: { x: number; y: number }, b: { x: number; y: number }) => a.x - b.x)
    );

    let renderMetrics = $derived(downsamplePoints(safeMetrics, MAX_RENDER_POINTS));

    let bounds = $derived((() => {
        if (renderMetrics.length === 0) {
            const now = Date.now();
            return { minX: now - 1, maxX: now, maxY: 1 };
        }
        const minX = renderMetrics[0].x;
        const maxXRaw = renderMetrics[renderMetrics.length - 1].x;
        const maxX = maxXRaw > minX ? maxXRaw : minX + 1;
        const observedMaxY = renderMetrics.reduce((max, p) => (p.y > max ? p.y : max), 0);
        const normalizedMax = type === "cpu"
            ? Math.max(100, observedMaxY * 1.1, 1)
            : Math.max(observedMaxY * 1.1, 1);
        return { minX, maxX, maxY: normalizedMax };
    })());

    const scaleX = (x: number): number => {
        const ratio = (x - bounds.minX) / (bounds.maxX - bounds.minX);
        return PAD_LEFT + Math.max(0, Math.min(1, ratio)) * innerWidth;
    };

    const scaleY = (y: number): number => {
        const ratio = y / bounds.maxY;
        return PAD_TOP + (1 - Math.max(0, Math.min(1, ratio))) * innerHeight;
    };

    let points = $derived(renderMetrics.map((p) => ({ x: scaleX(p.x), y: scaleY(p.y), rawX: p.x, rawY: p.y })));

    let linePath = $derived(
        points.length > 0
            ? points.map((p, idx) => `${idx === 0 ? "M" : "L"} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`).join(" ")
            : ""
    );

    let areaPath = $derived((() => {
        if (points.length === 0) return "";
        const first = points[0];
        const last = points[points.length - 1];
        return `${linePath} L ${last.x.toFixed(2)} ${baselineY.toFixed(2)} L ${first.x.toFixed(2)} ${baselineY.toFixed(2)} Z`;
    })());

    let yTickValues = $derived(Array.from({ length: 5 }, (_, idx) => bounds.maxY * (4 - idx) / 4));

    const axisLabel = (value: number): string => {
        if (type === "cpu") return `${value.toFixed(0)}%`;
        return formatBytes(value);
    };

    const latestPoint = $derived(points.length > 0 ? points[points.length - 1] : null);
    const latestValueLabel = $derived(latestPoint
        ? (type === "cpu" ? `${latestPoint.rawY.toFixed(1)}%` : formatBytes(latestPoint.rawY))
        : "No data");
</script>

<div class="w-full" bind:this={host}>
    {#if points.length > 0}
        <div class="w-full h-[250px] rounded-2xl border border-slate-100 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/35 p-2">
            <div class="flex items-center justify-between px-2 pb-1">
                <p class="text-[11px] font-black uppercase tracking-wider text-slate-500">{title}</p>
                <p class="text-[10px] font-black text-slate-500">Latest: {latestValueLabel}</p>
            </div>
            <svg class="w-full h-[210px]" viewBox={`0 0 ${chartWidth} ${CHART_HEIGHT}`} preserveAspectRatio="none" aria-label={title} role="img">
                {#each yTickValues as tick}
                    {@const y = scaleY(tick)}
                    <line x1={PAD_LEFT} y1={y} x2={PAD_LEFT + innerWidth} y2={y} stroke="currentColor" stroke-opacity="0.12" stroke-width="1" class="text-slate-500" />
                    <text x={PAD_LEFT - 6} y={y + 3} text-anchor="end" font-size="9" class="fill-slate-500 dark:fill-slate-400">{axisLabel(tick)}</text>
                {/each}

                <line x1={PAD_LEFT} y1={PAD_TOP} x2={PAD_LEFT} y2={baselineY} stroke="currentColor" stroke-opacity="0.2" class="text-slate-500" />
                <line x1={PAD_LEFT} y1={baselineY} x2={PAD_LEFT + innerWidth} y2={baselineY} stroke="currentColor" stroke-opacity="0.2" class="text-slate-500" />

                <path d={areaPath} fill={type === "cpu" ? "#0ea5e933" : "#8b5cf633"} />
                <path d={linePath} fill="none" stroke={type === "cpu" ? "#0ea5e9" : "#8b5cf6"} stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />

                {#if latestPoint}
                    <circle cx={latestPoint.x} cy={latestPoint.y} r="3" fill={type === "cpu" ? "#0ea5e9" : "#8b5cf6"} />
                {/if}
            </svg>
        </div>
    {:else}
        <div class="w-full h-[250px] flex items-center justify-center text-slate-500 italic text-xs bg-slate-50 dark:bg-slate-900/50 rounded-2xl border border-dashed border-slate-200 dark:border-slate-800">
            No telemetry data available for this window.
        </div>
    {/if}
</div>
