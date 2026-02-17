<script lang="ts">
    import { chart } from "svelte-apexcharts";
    import type { ContainerDiskUsage } from "../api-types";

    let { diskUsage } = $props<{ diskUsage?: ContainerDiskUsage }>();

    const formatBytes = (value?: number): string => {
        const n = Number(value || 0);
        if (!Number.isFinite(n) || n <= 0) return "0 B";
        const units = ["B", "KB", "MB", "GB", "TB"];
        let idx = 0;
        let size = n;
        while (size >= 1024 && idx < units.length - 1) {
            size /= 1024;
            idx++;
        }
        return `${size.toFixed(idx === 0 ? 0 : 2)} ${units[idx]}`;
    };

    let writable = $derived(Math.max(0, Number(diskUsage?.writableBytes || 0)));
    let rootFs = $derived(Math.max(0, Number(diskUsage?.rootFsBytes || 0)));
    let mounts = $derived(Math.max(0, Number(diskUsage?.mountCount || 0)));
    let hostTotal = $derived(Math.max(0, Number(diskUsage?.hostTotalBytes || 0)));
    let hostAvailable = $derived(Math.max(0, Number(diskUsage?.hostAvailableBytes || 0)));
    let hostUsed = $derived(Math.max(0, Number(diskUsage?.hostUsedBytes || 0)));

    let rootShare = $derived(hostTotal > 0 ? Math.min(rootFs, hostTotal) : rootFs);
    let writableShare = $derived(hostTotal > 0 ? Math.min(writable, hostTotal) : writable);

    let rootSeries = $derived(hostTotal > 0 ? [rootShare, Math.max(hostTotal - rootShare, 0)] : [rootFs, Math.max(rootFs - writable, 0)]);
    let writableSeries = $derived(hostTotal > 0 ? [writableShare, Math.max(hostTotal - writableShare, 0)] : [writable, Math.max(rootFs - writable, 0)]);

    function donutOptions(title: string, labels: string[], colors: string[]) {
        return {
            series: [0, 0],
            chart: {
                type: "donut",
                height: 210,
                toolbar: { show: false },
                background: "transparent",
                foreColor: "#64748b"
            },
            labels,
            colors,
            legend: {
                show: true,
                position: "bottom",
                fontSize: "11px",
                labels: { colors: ["#64748b"] }
            },
            dataLabels: { enabled: false },
            stroke: { width: 0 },
            title: {
                text: title,
                align: "left",
                style: {
                    fontSize: "12px",
                    fontWeight: "900",
                    fontFamily: "Montserrat",
                    color: "#64748b"
                }
            },
            plotOptions: {
                pie: {
                    donut: {
                        size: "68%"
                    }
                }
            },
            tooltip: {
                y: {
                    formatter: (val: number) => formatBytes(val)
                }
            }
        };
    }

    let rootOptions = $derived({
        ...donutOptions(
            hostTotal > 0 ? "RootFS vs Host" : "RootFS Breakdown",
            hostTotal > 0 ? ["Container RootFS", "Remaining Host"] : ["Container RootFS", "Writable Delta"],
            ["#0ea5e9", "#cbd5e1"]
        ),
        series: rootSeries
    });
    let writableOptions = $derived({
        ...donutOptions(
            hostTotal > 0 ? "Writable vs Host" : "Writable Share",
            hostTotal > 0 ? ["Writable Layer", "Remaining Host"] : ["Writable Layer", "RootFS Remainder"],
            ["#f97316", "#cbd5e1"]
        ),
        series: writableSeries
    });
</script>

{#if !diskUsage}
    <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-6 text-xs italic text-slate-500">
        Disk usage data unavailable for this container.
    </div>
{:else}
    <div class="space-y-4">
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {#key `${rootSeries[0]}:${rootSeries[1]}`}
                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 p-3">
                    <div use:chart={rootOptions}></div>
                </div>
            {/key}
            {#key `${writableSeries[0]}:${writableSeries[1]}`}
                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 p-3">
                    <div use:chart={writableOptions}></div>
                </div>
            {/key}
        </div>
        <div class="grid grid-cols-2 md:grid-cols-4 gap-2 text-xs">
            <div class="rounded-xl bg-slate-50 dark:bg-slate-900/50 p-3 border border-slate-100 dark:border-slate-700">
                <div class="text-[10px] font-black uppercase text-slate-500">Writable</div>
                <div class="font-bold text-slate-700 dark:text-slate-200">{formatBytes(writable)}</div>
            </div>
            <div class="rounded-xl bg-slate-50 dark:bg-slate-900/50 p-3 border border-slate-100 dark:border-slate-700">
                <div class="text-[10px] font-black uppercase text-slate-500">RootFS</div>
                <div class="font-bold text-slate-700 dark:text-slate-200">{formatBytes(rootFs)}</div>
            </div>
            <div class="rounded-xl bg-slate-50 dark:bg-slate-900/50 p-3 border border-slate-100 dark:border-slate-700">
                <div class="text-[10px] font-black uppercase text-slate-500">Host Used</div>
                <div class="font-bold text-slate-700 dark:text-slate-200">{formatBytes(hostUsed)}</div>
            </div>
            <div class="rounded-xl bg-slate-50 dark:bg-slate-900/50 p-3 border border-slate-100 dark:border-slate-700">
                <div class="text-[10px] font-black uppercase text-slate-500">Host Free / Mounts</div>
                <div class="font-bold text-slate-700 dark:text-slate-200">{formatBytes(hostAvailable)} / {mounts}</div>
            </div>
        </div>
    </div>
{/if}
