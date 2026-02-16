<script lang="ts">
    import type { ContainerSummary, Metric } from "../api-types";
    import MetricChart from "../components/MetricChart.svelte";

    let { containers } = $props<{
        containers: ContainerSummary[];
    }>();

    let expandedContainer = $state<string | null>(null);
    let metrics = $state<Metric[]>([]);
    let loadingMetrics = $state(false);
    let aiAnalyzing = $state(false);
    let aiInsight = $state("");

    const formatId = (id: string) => (id.length > 12 ? id.slice(0, 12) : id);
    const stateColor = (state: string) => {
        switch(state.toLowerCase()) {
            case 'running': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400';
            case 'exited': return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400';
            case 'paused': return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400';
            default: return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-400';
        }
    };

    const getPolicy = (labels: Record<string, string>) => {
        if (!labels) return null;
        return labels['harborwatch.update.policy'] || (labels['harborwatch.enable'] === 'true' ? 'auto' : null);
    };

    async function toggleExpand(id: string) {
        if (expandedContainer === id) {
            expandedContainer = null;
            metrics = [];
            return;
        }

        expandedContainer = id;
        loadingMetrics = true;
        try {
            const res = await fetch(`/api/metrics/${id}?duration=6h`);
            if (res.ok) {
                metrics = await res.json();
            }
        } catch (e) {
            console.error("Failed to fetch metrics", e);
        } finally {
            loadingMetrics = false;
        }
    }

    async function analyzeMetrics(id: string) {
        if (metrics.length === 0) return;
        
        aiAnalyzing = true;
        aiInsight = "";
        try {
            const res = await fetch("/api/ai/analyze-metrics", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ containerId: id, metrics })
            });
            if (res.ok) {
                const data = await res.json();
                aiInsight = data.analysis;
            }
        } catch (e) {
            console.error("AI analysis failed", e);
        } finally {
            aiAnalyzing = false;
        }
    }
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Container Inventory</h2>
        <div class="flex gap-2">
            <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-xs font-bold text-slate-600 dark:text-slate-400 uppercase">
                {containers.length} Total
            </span>
        </div>
    </div>

    <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden">
        <table class="w-full text-left border-collapse">
            <thead>
                <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 text-xs font-bold uppercase tracking-wider">
                    <th class="px-6 py-4 w-10"></th>
                    <th class="px-6 py-4">ID</th>
                    <th class="px-6 py-4">Name</th>
                    <th class="px-6 py-4">Image</th>
                    <th class="px-6 py-4">State</th>
                    <th class="px-6 py-4">Policy</th>
                    <th class="px-6 py-4 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                {#each containers as c}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors group {expandedContainer === c.id ? 'bg-slate-50/50 dark:bg-slate-900/30' : ''}">
                        <td class="px-6 py-4">
                            <button 
                                onclick={() => toggleExpand(c.id)}
                                class="text-slate-400 hover:text-brand-600 transition-colors"
                            >
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 transition-transform {expandedContainer === c.id ? 'rotate-90' : ''}" viewBox="0 0 20 20" fill="currentColor">
                                    <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd" />
                                </svg>
                            </button>
                        </td>
                        <td class="px-6 py-4 font-mono text-xs text-slate-400">{formatId(c.id)}</td>
                        <td class="px-6 py-4 font-bold text-slate-900 dark:text-white">
                            {c.names?.[0]?.replace(/^\//, '') ?? 'unnamed'}
                        </td>
                        <td class="px-6 py-4 text-sm text-slate-600 dark:text-slate-400 truncate max-w-[200px]" title={c.image}>
                            {c.image}
                        </td>
                        <td class="px-6 py-4">
                            <span class="px-2 py-1 rounded-md text-[10px] font-black uppercase {stateColor(c.state)}">
                                {c.state}
                            </span>
                        </td>
                        <td class="px-6 py-4">
                            {#if getPolicy(c.labels)}
                                <span class="px-2 py-1 bg-brand-100 text-brand-700 dark:bg-brand-900/30 dark:text-brand-400 rounded-md text-[9px] font-black uppercase tracking-tighter">
                                    {getPolicy(c.labels)}
                                </span>
                            {:else}
                                <span class="text-[10px] text-slate-400 italic">None</span>
                            {/if}
                        </td>
                        <td class="px-6 py-4 text-right">
                            <div class="flex justify-end gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                                <button class="p-2 text-slate-400 hover:text-brand-600 dark:hover:text-brand-400 transition-colors" title="Restart">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                    </svg>
                                </button>
                                <button class="p-2 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 transition-colors" title="Stop">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
                                    </svg>
                                </button>
                            </div>
                        </td>
                    </tr>
                    {#if expandedContainer === c.id}
                        <tr class="bg-slate-50/30 dark:bg-slate-900/20">
                            <td colspan="7" class="px-12 py-8">
                                {#if loadingMetrics}
                                    <div class="flex items-center justify-center py-12 gap-3 text-slate-400">
                                        <div class="w-5 h-5 border-2 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
                                        <span class="text-sm font-bold uppercase tracking-widest">Retrieving Metrics...</span>
                                    </div>
                                {:else if metrics.length > 0}
                                    <div class="grid grid-cols-1 xl:grid-cols-2 gap-8">
                                        <div class="bg-white dark:bg-slate-800/50 p-4 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
                                            <MetricChart {metrics} title="CPU Utilization (6h)" type="cpu" />
                                        </div>
                                        <div class="bg-white dark:bg-slate-800/50 p-4 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
                                            <MetricChart {metrics} title="Memory footprint (6h)" type="memory" />
                                        </div>
                                    </div>

                                    <div class="mt-8 pt-6 border-t border-slate-100 dark:border-slate-800">
                                        <div class="flex items-center justify-between mb-4">
                                            <h4 class="text-sm font-black uppercase text-slate-400 tracking-tighter flex items-center gap-2">
                                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                                                </svg>
                                                AI Performance consultant
                                            </h4>
                                            <button 
                                                onclick={() => analyzeMetrics(c.id)}
                                                disabled={aiAnalyzing}
                                                class="px-6 py-2 bg-brand-600 hover:bg-brand-700 text-white text-[10px] font-black uppercase tracking-widest rounded-xl transition-all shadow-lg shadow-brand-500/20 disabled:opacity-50"
                                            >
                                                {aiAnalyzing ? 'Analyzing Window...' : 'Request Resource Audit'}
                                            </button>
                                        </div>

                                        {#if aiInsight}
                                            <div class="bg-brand-50 dark:bg-brand-900/10 border border-brand-100 dark:border-brand-900/30 rounded-2xl p-6">
                                                <div class="prose dark:prose-invert prose-sm max-w-none text-slate-600 dark:text-slate-300 whitespace-pre-wrap italic leading-relaxed">
                                                    {aiInsight}
                                                </div>
                                            </div>
                                        {/if}
                                    </div>
                                {:else}
                                    <div class="text-center py-12 text-slate-400 italic text-sm">
                                        No performance data recorded for this container yet.
                                    </div>
                                {/if}
                            </td>
                        </tr>
                    {/if}
                {:else}
                    <tr>
                        <td colspan="7" class="px-6 py-12 text-center text-slate-400 italic">No containers found on socket</td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
</div>
