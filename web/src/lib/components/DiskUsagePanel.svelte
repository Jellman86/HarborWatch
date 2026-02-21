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
    <div class="space-y-6">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div class="p-6 rounded-3xl bg-slate-50 dark:bg-slate-900/40 border border-slate-200 dark:border-slate-700 flex flex-col justify-between">
                <div>
                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Container Footprint</p>
                    <p class="text-3xl font-black text-slate-900 dark:text-white mt-1">{formatBytes(rootFs)}</p>
                </div>
                <p class="text-[11px] text-slate-500 mt-4 leading-relaxed">
                    Total space taken by the container's root file system, including all read-only image layers.
                </p>
            </div>

            <div class="p-6 rounded-3xl bg-orange-50 dark:bg-orange-900/10 border border-orange-100 dark:border-orange-900/30 flex flex-col justify-between">
                <div>
                    <p class="text-[10px] font-black uppercase tracking-widest text-orange-600 dark:text-orange-400">Writable Layer</p>
                    <p class="text-3xl font-black text-orange-700 dark:text-orange-300 mt-1">{formatBytes(writable)}</p>
                </div>
                <p class="text-[11px] text-orange-800/70 dark:text-orange-300/70 mt-4 leading-relaxed">
                    Active changes made since the container started. High usage here often suggests missing persistent volumes.
                </p>
            </div>
        </div>

        <div class="space-y-3">
            <div class="flex items-center justify-between px-2">
                <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Persistence Map</p>
                <span class="px-2 py-0.5 bg-slate-100 dark:bg-slate-800 rounded text-[10px] font-bold text-slate-500 uppercase">{mounts} Active Mounts</span>
            </div>
            
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                <div class="p-4 rounded-2xl bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 shadow-sm flex items-center justify-between">
                    <div>
                        <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Host Free</p>
                        <p class="text-sm font-bold text-slate-900 dark:text-white">{formatBytes(hostAvailable)}</p>
                    </div>
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                    </svg>
                </div>
            </div>
        </div>
    </div>
{/if}
