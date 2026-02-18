<script lang="ts">
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
            idx += 1;
        }
        return `${size.toFixed(idx === 0 ? 0 : 2)} ${units[idx]}`;
    };

    const nonNegativeFinite = (value: unknown): number => {
        const n = Number(value);
        return Number.isFinite(n) ? Math.max(0, n) : 0;
    };

    let writable = $derived(nonNegativeFinite(diskUsage?.writableBytes));
    let rootFs = $derived(nonNegativeFinite(diskUsage?.rootFsBytes));
    let mounts = $derived(nonNegativeFinite(diskUsage?.mountCount));
    let hostTotal = $derived(nonNegativeFinite(diskUsage?.hostTotalBytes));
    let hostAvailable = $derived(nonNegativeFinite(diskUsage?.hostAvailableBytes));
    let hostUsed = $derived(nonNegativeFinite(diskUsage?.hostUsedBytes));

    let rootReference = $derived(hostTotal > 0 ? hostTotal : Math.max(rootFs, 1));
    let writableReference = $derived(hostTotal > 0 ? hostTotal : Math.max(rootFs, 1));

    let rootPercent = $derived(Math.max(0, Math.min(100, (rootFs / Math.max(rootReference, 1)) * 100)));
    let writablePercent = $derived(Math.max(0, Math.min(100, (writable / Math.max(writableReference, 1)) * 100)));

    let rootHeading = $derived(hostTotal > 0 ? "RootFS vs Host" : "RootFS footprint");
    let writableHeading = $derived(hostTotal > 0 ? "Writable vs Host" : "Writable vs RootFS");
</script>

{#if !diskUsage}
    <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-6 text-xs italic text-slate-500">
        Disk usage data unavailable for this container.
    </div>
{:else}
    <div class="space-y-4">
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 p-4 space-y-3">
                <div class="flex items-center justify-between">
                    <p class="text-[11px] font-black uppercase tracking-wide text-slate-500">{rootHeading}</p>
                    <p class="text-xs font-bold text-slate-700 dark:text-slate-200">{rootPercent.toFixed(1)}%</p>
                </div>
                <div class="h-3 rounded-full bg-slate-200 dark:bg-slate-700 overflow-hidden">
                    <div class="h-full bg-sky-500" style={`width:${rootPercent.toFixed(2)}%`}></div>
                </div>
                <div class="text-[11px] text-slate-500">{formatBytes(rootFs)} used of {formatBytes(rootReference)}</div>
            </div>

            <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 p-4 space-y-3">
                <div class="flex items-center justify-between">
                    <p class="text-[11px] font-black uppercase tracking-wide text-slate-500">{writableHeading}</p>
                    <p class="text-xs font-bold text-slate-700 dark:text-slate-200">{writablePercent.toFixed(1)}%</p>
                </div>
                <div class="h-3 rounded-full bg-slate-200 dark:bg-slate-700 overflow-hidden">
                    <div class="h-full bg-orange-500" style={`width:${writablePercent.toFixed(2)}%`}></div>
                </div>
                <div class="text-[11px] text-slate-500">{formatBytes(writable)} used of {formatBytes(writableReference)}</div>
            </div>
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
