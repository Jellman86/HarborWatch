<script lang="ts">
    import { onMount } from "svelte";
    import { chart } from 'svelte-apexcharts';
    import type { Metric } from '../api-types';

    let { metrics, title, type = 'cpu' } = $props<{
        metrics: Metric[];
        title: string;
        type: 'cpu' | 'memory';
    }>();

    const CHART_HEIGHT = 250;
    const MIN_CHART_WIDTH = 80;

    const formatBytes = (bytes: number) => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    const toFinite = (value: unknown): number | null => {
        const n = Number(value);
        return Number.isFinite(n) ? n : null;
    };

    const normalizeTimestamp = (value: unknown): number | null => {
        const ts = toFinite(value);
        if (ts === null || ts <= 0) return null;
        // Backend stores seconds; preserve ms values if already present.
        return ts > 1_000_000_000_000 ? ts : ts * 1000;
    };

    let host = $state<HTMLDivElement | null>(null);
    let hostWidth = $state(0);

    const measureHost = () => {
        const width = host?.getBoundingClientRect().width ?? 0;
        hostWidth = Number.isFinite(width) ? Math.max(Math.floor(width), 0) : 0;
    };

    onMount(() => {
        measureHost();
        const onResize = () => measureHost();
        let frame = window.requestAnimationFrame(measureHost);
        let observer: ResizeObserver | null = null;
        if (typeof ResizeObserver !== "undefined" && host) {
            observer = new ResizeObserver(() => measureHost());
            observer.observe(host);
        }
        window.addEventListener("resize", onResize);
        return () => {
            window.cancelAnimationFrame(frame);
            observer?.disconnect();
            window.removeEventListener("resize", onResize);
        };
    });

    let safeMetrics = $derived((metrics || [])
        .map((m: Metric) => {
            const x = normalizeTimestamp(m?.timestamp);
            const rawY = type === 'cpu' ? toFinite(m?.cpuPercent) : toFinite(m?.memoryUsage);
            if (x === null || rawY === null) return null;
            return {
                x,
                y: Math.max(0, rawY)
            };
        })
        .filter((point): point is { x: number; y: number } => point !== null));

    let series = $derived([
        {
            name: type === 'cpu' ? 'CPU %' : 'Memory Usage',
            data: safeMetrics
        }
    ]);

    let canRenderChart = $derived(hostWidth >= MIN_CHART_WIDTH && safeMetrics.length > 0);

    let options = $derived({
        series: series,
        chart: {
            type: 'area',
            width: Math.max(hostWidth, MIN_CHART_WIDTH),
            height: CHART_HEIGHT,
            animations: { enabled: true },
            toolbar: { show: false },
            zoom: { enabled: false },
            redrawOnParentResize: true,
            background: 'transparent',
            foreColor: '#94a3b8'
        },
        dataLabels: {
            enabled: false
        },
        legend: {
            show: false
        },
        title: {
            text: title,
            align: 'left',
            style: {
                fontSize: '12px',
                fontWeight: '900',
                fontFamily: 'Montserrat',
                color: '#64748b'
            }
        },
        stroke: {
            curve: 'smooth',
            width: 2,
            colors: [type === 'cpu' ? '#0ea5e9' : '#8b5cf6']
        },
        fill: {
            type: 'gradient',
            gradient: {
                shadeIntensity: 1,
                opacityFrom: 0.45,
                opacityTo: 0.05,
                stops: [20, 100],
                colorStops: [
                    {
                        offset: 0,
                        color: type === 'cpu' ? '#0ea5e9' : '#8b5cf6',
                        opacity: 0.4
                    },
                    {
                        offset: 100,
                        color: type === 'cpu' ? '#0ea5e9' : '#8b5cf6',
                        opacity: 0
                    }
                ]
            }
        },
        xaxis: {
            type: 'datetime',
            labels: {
                datetimeUTC: false,
                style: { fontSize: '10px' }
            },
            axisBorder: { show: false },
            axisTicks: { show: false }
        },
        yaxis: {
            labels: {
                style: { fontSize: '10px' },
                formatter: (val: number) => {
                    const safeVal = Number.isFinite(val) ? val : 0;
                    return type === 'cpu' ? safeVal.toFixed(1) + '%' : formatBytes(safeVal);
                }
            }
        },
        grid: {
            borderColor: '#334155',
            strokeDashArray: 4,
            xaxis: { lines: { show: true } },
            yaxis: { lines: { show: true } }
        },
        theme: {
            mode: 'dark'
        },
        tooltip: {
            x: { format: 'dd MMM HH:mm' },
            theme: 'dark'
        }
    });
</script>

<div class="w-full" bind:this={host}>
    {#if canRenderChart}
        {#key `${type}:${hostWidth}:${safeMetrics.length}:${safeMetrics[0]?.x ?? 0}:${safeMetrics[safeMetrics.length - 1]?.x ?? 0}`}
            <div class="w-full h-[250px]" use:chart={options}></div>
        {/key}
    {:else if safeMetrics.length > 0}
        <div class="w-full h-[250px] flex items-center justify-center text-slate-500 italic text-xs bg-slate-50 dark:bg-slate-900/50 rounded-2xl border border-dashed border-slate-200 dark:border-slate-800">
            Preparing chart...
        </div>
    {:else}
        <div class="w-full h-[250px] flex items-center justify-center text-slate-500 italic text-xs bg-slate-50 dark:bg-slate-900/50 rounded-2xl border border-dashed border-slate-200 dark:border-slate-800">
            No telemetry data available for this window.
        </div>
    {/if}
    </div>
