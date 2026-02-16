<script lang="ts">
    import type { ContainerSummary, Metric } from "../api-types";
    import MetricChart from "../components/MetricChart.svelte";
    import Sparkline from "../components/Sparkline.svelte";

    let { containers, onNavigate } = $props<{
        containers: ContainerSummary[];
        onNavigate: (route: string, params?: any) => void;
    }>();

    let viewMode = $state(typeof localStorage !== 'undefined' ? (localStorage.getItem('hw_container_view') ?? 'list') : 'list');
    let expandedContainer = $state<string | null>(null);

    function toggleView() {
        viewMode = viewMode === 'list' ? 'cards' : 'list';
        if (typeof localStorage !== 'undefined') {
            localStorage.setItem('hw_container_view', viewMode);
        }
    }
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

    const getIntelURL = (labels: Record<string, string>) => {
        if (!labels) return null;
        return labels['harborwatch.intel.url'] || labels['org.opencontainers.image.source'] || labels['org.label-schema.vcs-url'];
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
                const data = await res.json();
                metrics = data || [];
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

    function handleTriggerScan(image: string) {
        onNavigate('security', { target: image });
    }

    function handleCheckUpdate(container: ContainerSummary) {
        onNavigate('updates', { 
            containerId: container.id, 
            targetImage: container.image,
            validateUrl: container.labels?.['harborwatch.validate.url'] || 'http://localhost:18080/health'
        });
    }
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Container Inventory</h2>
        <div class="flex items-center gap-3">
            <div class="flex bg-slate-100 dark:bg-slate-800 p-1 rounded-xl border border-slate-200 dark:border-slate-700">
                <button 
                    onclick={() => { if(viewMode !== 'list') toggleView() }}
                    class="p-1.5 rounded-lg transition-all {viewMode === 'list' ? 'bg-white dark:bg-slate-700 shadow-sm text-brand-600' : 'text-slate-400 hover:text-slate-600'}"
                    title="List View"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
                    </svg>
                </button>
                <button 
                    onclick={() => { if(viewMode !== 'cards') toggleView() }}
                    class="p-1.5 rounded-lg transition-all {viewMode === 'cards' ? 'bg-white dark:bg-slate-700 shadow-sm text-brand-600' : 'text-slate-400 hover:text-slate-600'}"
                    title="Card View"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
                    </svg>
                </button>
            </div>
            <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-xs font-bold text-slate-600 dark:text-slate-400 uppercase">
                {containers.length} Total
            </span>
        </div>
    </div>

    {#if viewMode === 'list'}
    <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden">
        <table class="w-full text-left border-collapse">
            <thead>
                <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 text-xs font-bold uppercase tracking-wider">
                    <th class="px-6 py-4 w-10"></th>
                    <th class="px-6 py-4">ID</th>
                    <th class="px-6 py-4">Name</th>
                    <th class="px-6 py-4">Image</th>
                    <th class="px-6 py-4">Source</th>
                    <th class="px-6 py-4">State</th>
                    <th class="px-6 py-4">Activity</th>
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
                                aria-label={expandedContainer === c.id ? 'Collapse details' : 'Expand details'}
                                title={expandedContainer === c.id ? 'Collapse details' : 'Expand details'}
                            >
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 transition-transform {expandedContainer === c.id ? 'rotate-90' : ''}" viewBox="0 0 20 20" fill="currentColor">
                                    <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd" />
                                </svg>
                            </button>
                        </td>
                        <td class="px-6 py-4 font-mono text-xs text-slate-400">{formatId(c.id)}</td>
                        <td class="px-6 py-4">
                            <div class="flex items-center gap-2">
                                <button 
                                    onclick={() => onNavigate('container-detail', { id: c.id })}
                                    class="font-bold text-slate-900 dark:text-white hover:text-brand-600 transition-colors text-left"
                                >
                                    {c.names?.[0]?.replace(/^\//, '') ?? 'unnamed'}
                                </button>
                                {#if c.updateAvailable}
                                    <span class="px-1.5 py-0.5 bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 rounded text-[8px] font-black uppercase animate-pulse">
                                        Update Available
                                    </span>
                                {/if}
                            </div>
                        </td>
                        <td class="px-6 py-4 text-sm text-slate-600 dark:text-slate-400 truncate max-w-[150px]" title={c.image}>
                            {c.image}
                        </td>
                        <td class="px-6 py-4">
                            {#if getIntelURL(c.labels)}
                                <a href={getIntelURL(c.labels)} target="_blank" class="text-brand-600 dark:text-brand-400 hover:underline flex items-center gap-1">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                                    </svg>
                                    <span class="text-[10px] font-bold uppercase tracking-tight">Repo</span>
                                </a>
                            {:else}
                                <span class="text-[10px] text-slate-400 italic">None</span>
                            {/if}
                        </td>
                        <td class="px-6 py-4">
                            <span class="px-2 py-1 rounded-md text-[10px] font-black uppercase {stateColor(c.state)}">
                                {c.state}
                            </span>
                        </td>
                        <td class="px-6 py-4">
                            <Sparkline containerId={c.id} />
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
                            <div class="flex justify-end gap-1">
                                <button 
                                    onclick={() => handleTriggerScan(c.image)}
                                    class="p-1.5 text-slate-400 hover:text-brand-600 hover:bg-brand-50 dark:hover:bg-brand-900/20 rounded-lg transition-all" 
                                    title="Trigger Scan"
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                                    </svg>
                                </button>
                                <button 
                                    onclick={() => handleCheckUpdate(c)}
                                    class="p-1.5 text-slate-400 hover:text-emerald-600 hover:bg-emerald-50 dark:hover:bg-brand-900/20 rounded-lg transition-all" 
                                    title="Check Update"
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
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
    {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
        {#each containers as c}
            <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden flex flex-col group hover:border-brand-500/50 transition-all">
                <div class="p-5 flex-1">
                    <div class="flex justify-between items-start mb-4">
                        <div class="flex flex-col min-w-0">
                            <h3 class="font-black text-slate-900 dark:text-white truncate text-lg tracking-tight">
                                {c.names?.[0]?.replace(/^\//, '') ?? 'unnamed'}
                            </h3>
                            <span class="text-[10px] font-mono text-slate-400">{formatId(c.id)}</span>
                        </div>
                        <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase {stateColor(c.state)}">
                            {c.state}
                        </span>
                    </div>

                    <div class="space-y-4">
                        <div class="flex flex-col">
                            <span class="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-1">Image</span>
                            <span class="text-xs text-slate-600 dark:text-slate-300 truncate font-medium" title={c.image}>{c.image}</span>
                        </div>

                        <div class="flex flex-col h-[40px] justify-center">
                            <span class="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-1">Activity (1h)</span>
                            <Sparkline containerId={c.id} />
                        </div>

                        {#if c.updateAvailable}
                            <div class="p-3 bg-amber-50 dark:bg-amber-900/10 border border-amber-100 dark:border-amber-900/30 rounded-xl flex items-center gap-3 animate-pulse">
                                <div class="w-2 h-2 rounded-full bg-amber-500"></div>
                                <span class="text-[10px] font-black text-amber-700 dark:text-amber-400 uppercase">Update Available</span>
                            </div>
                        {/if}
                    </div>
                </div>

                <div class="px-5 py-4 bg-slate-50 dark:bg-slate-900/50 border-t border-slate-100 dark:border-slate-700 flex justify-between items-center">
                    <div class="flex gap-2">
                        {#if getIntelURL(c.labels)}
                            <a href={getIntelURL(c.labels)} target="_blank" class="p-2 text-slate-400 hover:text-brand-600 transition-colors" title="Repository">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                                </svg>
                            </a>
                        {/if}
                    </div>
                    <div class="flex gap-2">
                        <button 
                            onclick={() => handleTriggerScan(c.image)}
                            class="px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-500 hover:text-brand-600 transition-all"
                        >
                            Scan
                        </button>
                        <button 
                            onclick={() => onNavigate('container-detail', { id: c.id })}
                            class="px-4 py-1.5 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 shadow-sm rounded-lg text-[10px] font-black uppercase tracking-widest text-slate-700 dark:text-slate-200 hover:border-brand-500 transition-all"
                        >
                            Manage
                        </button>
                    </div>
                </div>
            </div>
        {:else}
            <div class="col-span-full py-12 text-center text-slate-400 italic">No containers found on socket</div>
        {/each}
    </div>
    {/if}
</div>
