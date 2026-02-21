<script lang="ts">
    import { onMount } from "svelte";
    import type { ContainerSummary, ScanSummary, ImageSummary, DockerEvent } from "../api-types";
    import { configStore } from "../stores/config.svelte";

    let { containers, images, events, onRefresh } = $props<{ 
        containers: ContainerSummary[], 
        images?: ImageSummary[], 
        events?: DockerEvent[],
        onRefresh?: () => void
    }>();
    let scanSummary = $state<ScanSummary | null>(null);
    let fleetAdvice = $state("");
    let analyzingFleet = $state(false);

    async function loadData() {
        try {
            const res = await fetch("/api/scans/summary");
            if (res.ok) scanSummary = await res.json();
        } catch (e) {
            console.error("Failed to load dashboard data", e);
        }
    }

    async function getFleetAdvice() {
        if (containers.length === 0) return;
        analyzingFleet = true;
        try {
            const res = await fetch("/api/ai/fleet-advice", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(containers)
            });
            if (res.ok) {
                const data = await res.json();
                fleetAdvice = data.advice;
            }
        } catch (e) {
            console.error("Fleet analysis failed", e);
        } finally {
            analyzingFleet = false;
        }
    }

    onMount(() => {
        loadData();
    });

    let safeContainers = $derived(containers || []);
    let runningCount = $derived(safeContainers.filter((c: ContainerSummary) => c.state === 'running').length);
    let updateCount = $derived(safeContainers.filter((c: ContainerSummary) => c.updateAvailable).length);
</script>

<div class="space-y-8">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-5 py-1">
            <h2 class="text-3xl font-black text-slate-900 dark:text-white tracking-tighter uppercase">Command Oversight</h2>
            <p class="text-sm text-slate-500 font-medium mt-1">Global posture and intelligence summary for the managed fleet.</p>
        </div>
        <div class="flex gap-2">
            <div class="px-4 py-2 bg-slate-100 dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 flex items-center gap-3">
                <div class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></div>
                <span class="text-[10px] font-black uppercase tracking-widest text-slate-500">System Nominal</span>
            </div>
        </div>
    </div>

    <!-- Quick Stats -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 opacity-0 animate-reveal stagger-1">
        <div class="bg-white dark:bg-slate-800 p-5 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl relative overflow-hidden group">
            <div class="absolute top-0 right-0 p-4 opacity-5 group-hover:scale-110 transition-transform">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" /></svg>
            </div>
            <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-2">Active Assets</p>
            <p class="text-3xl font-black text-slate-900 dark:text-white tracking-tighter">{runningCount}<span class="text-base text-slate-400 ml-2 font-medium">/ {safeContainers.length}</span></p>
        </div>

        <div class="bg-white dark:bg-slate-800 p-5 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl relative overflow-hidden group">
            <div class="absolute top-0 right-0 p-4 opacity-5 group-hover:scale-110 transition-transform">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
            </div>
            <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-2">Updates Detected</p>
            <p class="text-3xl font-black {updateCount > 0 ? 'text-amber-500' : 'text-slate-900 dark:text-white'} tracking-tighter">{updateCount}</p>
        </div>

        <div class="bg-white dark:bg-slate-800 p-5 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl relative overflow-hidden group">
            <div class="absolute top-0 right-0 p-4 opacity-5 group-hover:scale-110 transition-transform text-rose-500">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" /></svg>
            </div>
            <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-2">Vulnerabilities</p>
            <p class="text-3xl font-black {(scanSummary?.critical || 0) > 0 ? 'text-rose-600' : 'text-slate-900 dark:text-white'} tracking-tighter">{scanSummary?.critical || 0}</p>
        </div>

        <div class="bg-white dark:bg-slate-800 p-5 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl relative overflow-hidden group">
            <div class="absolute top-0 right-0 p-4 opacity-5 group-hover:scale-110 transition-transform">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>
            </div>
            <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-2">Security Scans</p>
            <p class="text-3xl font-black text-slate-900 dark:text-white tracking-tighter">{scanSummary?.total || 0}</p>
        </div>
    </div>

    <!-- AI Advisor -->
    {#if configStore.aiActive}
        <div class="bg-slate-900 rounded-2xl p-6 md:p-7 border border-slate-800 shadow-2xl relative overflow-hidden opacity-0 animate-reveal stagger-2">
            <div class="absolute top-0 right-0 w-1/3 h-full bg-gradient-to-l from-brand-600/10 to-transparent pointer-events-none"></div>
            
            <div class="relative z-10 space-y-6">
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-4">
                        <div class="w-11 h-11 rounded-xl bg-brand-600 flex items-center justify-center text-white shadow-xl shadow-brand-500/20">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
                        </div>
                        <div>
                            <h3 class="text-lg font-black text-white uppercase tracking-tight">Fleet Intelligence Advisor</h3>
                            <p class="text-xs text-slate-400 font-medium">Heuristic analysis of your current deployment state.</p>
                        </div>
                    </div>
                    <button 
                        onclick={getFleetAdvice}
                        disabled={analyzingFleet}
                        class="px-7 py-2.5 bg-white hover:bg-slate-100 text-slate-900 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all disabled:opacity-50 active:scale-95 shadow-xl"
                    >
                        {analyzingFleet ? 'Processing Fleet Data...' : 'Generate AI Advice'}
                    </button>
                </div>

                {#if fleetAdvice}
                    <div class="bg-slate-950/50 border border-slate-800 rounded-2xl p-5">
                        <div class="prose prose-invert prose-sm max-w-none text-slate-300 italic leading-relaxed whitespace-pre-wrap">
                            {fleetAdvice}
                        </div>
                    </div>
                {:else}
                    <div class="py-12 text-center">
                        <p class="text-slate-500 text-sm font-medium italic">Request a fresh analysis to see proactive security and maintenance recommendations.</p>
                    </div>
                {/if}
            </div>
        </div>
    {/if}
</div>
