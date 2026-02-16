<script lang="ts">
    import { onMount } from "svelte";
    import type { HealthResponse, ScanSummary, ContainerSummary, ImageSummary, DockerEvent } from "../api-types";

    let { health, containers, images, events, onRefresh } = $props<{
        health: HealthResponse | null;
        containers: ContainerSummary[];
        images: ImageSummary[];
        events: DockerEvent[];
        onRefresh: () => void;
    }>();

    let summary = $state<ScanSummary | null>(null);

    async function loadSummary() {
        try {
            const res = await fetch("/api/scans/summary");
            if (res.ok) summary = await res.json();
        } catch {}
    }

    onMount(() => {
        loadSummary();
    });

    const formatId = (id: string) => (id.length > 12 ? id.slice(0, 12) : id);
    const riskBand = (score: number) => score >= 80 ? "Critical" : score >= 60 ? "High" : score >= 30 ? "Medium" : score > 0 ? "Low" : "None";
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Control Center</h2>
        <button class="px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-lg font-semibold transition-colors flex items-center gap-2 shadow-lg shadow-brand-500/20" onclick={onRefresh}>
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Refresh
        </button>
    </div>

    <!-- Stats Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm transition-all hover:shadow-md">
            <span class="text-xs font-bold text-slate-400 dark:text-slate-500 uppercase tracking-widest">Asset Fleet</span>
            <div class="mt-2 flex items-baseline gap-2">
                <span class="text-3xl font-black text-slate-900 dark:text-white">{containers.length}</span>
                <span class="text-[10px] font-black text-emerald-600 dark:text-emerald-400 uppercase">Containers</span>
            </div>
        </div>
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm transition-all hover:shadow-md">
            <span class="text-xs font-bold text-slate-400 dark:text-slate-500 uppercase tracking-widest">Stored Artifacts</span>
            <div class="mt-2 flex items-baseline gap-2">
                <span class="text-3xl font-black text-slate-900 dark:text-white">{images.length}</span>
                <span class="text-[10px] font-black text-slate-500 uppercase tracking-widest">Images</span>
            </div>
        </div>
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm transition-all hover:shadow-md">
            <span class="text-xs font-bold text-slate-400 dark:text-slate-500 uppercase tracking-widest">Aggregate Risk</span>
            <div class="mt-2 flex items-baseline gap-2">
                <span class="text-3xl font-black {summary && summary.riskScore > 50 ? 'text-rose-600' : 'text-emerald-600'}">
                    {summary ? summary.riskScore : '0'}
                </span>
                <span class="text-[10px] font-black text-slate-500 uppercase tracking-widest">{summary ? riskBand(summary.riskScore) : 'N/A'}</span>
            </div>
        </div>
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm transition-all hover:shadow-md">
            <span class="text-xs font-bold text-slate-400 dark:text-slate-500 uppercase tracking-widest">Appliance Health</span>
            <div class="mt-2 flex items-baseline gap-2">
                <span class="text-xl font-black text-brand-600 dark:text-brand-400 uppercase tracking-tighter">{health?.status ?? 'Connecting...'}</span>
                <span class="text-[10px] font-bold text-slate-400 ml-auto">{health?.version}</span>
            </div>
        </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Live Events -->
        <div class="lg:col-span-2 bg-slate-900 rounded-2xl border border-slate-800 shadow-2xl flex flex-col h-[450px]">
            <div class="p-4 border-b border-slate-800 flex items-center justify-between">
                <h3 class="font-bold text-slate-400 flex items-center gap-2 text-xs uppercase tracking-widest">
                    <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
                    Operational Stream
                </h3>
                <span class="text-[10px] font-mono text-slate-600">docker.sock</span>
            </div>
            <div class="flex-1 overflow-y-auto p-4 space-y-1 font-mono text-[11px]">
                {#each events as e}
                    <div class="flex gap-3 text-slate-400">
                        <span class="text-slate-600">[{new Date(e.time * 1000).toLocaleTimeString()}]</span>
                        <span class="font-bold text-emerald-500 uppercase w-16">{e.action}</span>
                        <span class="text-slate-300 truncate flex-1">{formatId(e.id)}</span>
                        <span class="text-slate-600 truncate max-w-[150px]">via {e.from}</span>
                    </div>
                {:else}
                    <div class="flex items-center justify-center h-full text-slate-600 italic">Listening for engine events...</div>
                {/each}
            </div>
        </div>

        <!-- Security Quick View -->
        <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm p-6">
            <h3 class="font-bold text-slate-900 dark:text-white mb-6 uppercase tracking-widest text-xs flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
                Security Hotspots
            </h3>
            {#if summary}
                <div class="space-y-4">
                    <div class="p-4 bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 rounded-xl">
                        <div class="text-[10px] font-black text-rose-600 dark:text-rose-400 uppercase mb-1">Critical CVEs</div>
                        <div class="text-3xl font-black text-rose-700 dark:text-rose-300">{summary.critical}</div>
                    </div>
                    <div class="p-4 bg-orange-50 dark:bg-orange-900/10 border border-orange-100 dark:border-orange-900/30 rounded-xl">
                        <div class="text-[10px] font-black text-orange-600 dark:text-orange-400 uppercase mb-1">High Severity</div>
                        <div class="text-3xl font-black text-orange-700 dark:text-orange-300">{summary.high}</div>
                    </div>
                    <div class="mt-6 pt-4 border-t border-slate-50 dark:border-slate-700 text-[10px] text-slate-500 dark:text-slate-400 leading-relaxed italic">
                        Latest scan for <code class="bg-slate-100 dark:bg-slate-700 px-1 rounded not-italic font-bold">{summary.target}</code> completed on {new Date(summary.scannedAt * 1000).toLocaleDateString()}.
                    </div>
                </div>
            {:else}
                <div class="flex flex-col items-center justify-center h-64 text-slate-400 gap-3">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 opacity-20" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                    </svg>
                    <span class="text-sm italic">No security data available</span>
                </div>
            {/if}
        </div>
    </div>
</div>
