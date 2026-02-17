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
    let metrics = $state<Metric[]>([]);
    let loadingMetrics = $state(false);
    let aiAnalyzing = $state(false);
    let aiInsight = $state("");
    let sparklineMetrics = $state<Record<string, Metric[]>>({});
    let lastSparklineKey = $state("");

    function toggleView() {
        viewMode = viewMode === 'list' ? 'cards' : 'list';
        if (typeof localStorage !== 'undefined') {
            localStorage.setItem('hw_container_view', viewMode);
        }
    }

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

    let safeContainers = $derived(containers || []);

    async function loadSparklineMetrics(ids: string[]) {
        if (ids.length === 0) {
            sparklineMetrics = {};
            return;
        }
        try {
            const res = await fetch("/api/metrics/batch", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ ids, duration: "1h" })
            });
            if (!res.ok) return;
            const data = await res.json();
            sparklineMetrics = data || {};
        } catch (e) {
            console.error("Failed to fetch sparkline metrics batch", e);
        }
    }

    $effect(() => {
        const ids = safeContainers.map(c => c.id).filter(Boolean);
        const key = ids.join(",");
        if (key === lastSparklineKey) return;
        lastSparklineKey = key;
        loadSparklineMetrics(ids);
    });
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">Fleet Inventory</h2>
            <p class="text-xs text-slate-500 font-medium">Real-time status of all managed container assets.</p>
        </div>
        <div class="flex items-center gap-3">
            <div class="flex bg-slate-100 dark:bg-slate-800 p-1 rounded-xl border border-slate-200 dark:border-slate-700 shadow-inner">
                <button 
                    onclick={() => { if(viewMode !== 'list') toggleView() }}
                    class="p-1.5 rounded-lg transition-all {viewMode === 'list' ? 'bg-white dark:bg-slate-700 shadow-md text-brand-600' : 'text-slate-400 hover:text-slate-600'}"
                    title="List View"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
                    </svg>
                </button>
                <button 
                    onclick={() => { if(viewMode !== 'cards') toggleView() }}
                    class="p-1.5 rounded-lg transition-all {viewMode === 'cards' ? 'bg-white dark:bg-slate-700 shadow-md text-brand-600' : 'text-slate-400 hover:text-slate-600'}"
                    title="Card View"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
                    </svg>
                </button>
            </div>
            <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-[10px] font-black text-slate-600 dark:text-slate-400 uppercase tracking-widest border border-slate-200 dark:border-slate-700">
                {safeContainers.length} Total
            </span>
        </div>
    </div>

    {#if viewMode === 'list'}
    <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl overflow-hidden opacity-0 animate-reveal stagger-1">
        <table class="w-full text-left border-collapse">
            <thead>
                <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 text-[10px] font-black uppercase tracking-widest border-b border-slate-100 dark:border-slate-700">
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
                {#each safeContainers as c, i}
                    <tr 
                        class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors group {expandedContainer === c.id ? 'bg-slate-50/50 dark:bg-slate-900/30' : ''} opacity-0 animate-reveal"
                        style="animation-delay: {0.1 + (i * 0.05)}s"
                    >
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
                        <td class="px-6 py-4 font-mono text-[10px] text-slate-400">{formatId(c.id)}</td>
                        <td class="px-6 py-4">
                            <div class="flex items-center gap-2">
                                <button 
                                    onclick={() => onNavigate('container-detail', { id: c.id })}
                                    class="font-bold text-slate-900 dark:text-white hover:text-brand-600 transition-colors text-left truncate max-w-[120px]"
                                >
                                    {c.names?.[0]?.replace(/^\//, '') ?? 'unnamed'}
                                </button>
                                {#if c.updateAvailable}
                                    <span class="px-1.5 py-0.5 bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 rounded-[4px] text-[8px] font-black uppercase animate-pulse">
                                        Update
                                    </span>
                                {/if}
                            </div>
                        </td>
                        <td class="px-6 py-4 text-xs text-slate-600 dark:text-slate-400 truncate max-w-[150px]" title={c.image}>
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
                            <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase {stateColor(c.state)}">
                                {c.state}
                            </span>
                        </td>
                        <td class="px-6 py-4">
                            <Sparkline metrics={sparklineMetrics[c.id] || []} />
                        </td>
                        <td class="px-6 py-4">
                            {#if getPolicy(c.labels)}
                                <span class="px-2 py-1 bg-brand-100 text-brand-700 dark:bg-brand-900/30 dark:text-brand-400 rounded-md text-[9px] font-black uppercase tracking-tighter">
                                    {getPolicy(c.labels)}
                                </span>
                            {:else}
                                <span class="text-[10px] text-slate-400 italic font-medium">Standard</span>
                            {/if}
                        </td>
                        <td class="px-6 py-4 text-right">
                            <div class="flex justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                                <button 
                                    onclick={() => onNavigate('container-detail', { id: c.id })}
                                    class="p-1.5 text-slate-400 hover:text-brand-600 hover:bg-brand-50 dark:hover:bg-brand-900/20 rounded-lg transition-all" 
                                    title="Manage Asset"
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                                    </svg>
                                </button>
                            </div>
                        </td>
                    </tr>
                    {#if expandedContainer === c.id}
                        <tr class="bg-slate-50/30 dark:bg-slate-900/20">
                            <td colspan="9" class="px-12 py-8 animate-reveal">
                                {#if loadingMetrics}
                                    <div class="flex items-center justify-center py-12 gap-3 text-slate-400">
                                        <div class="w-5 h-5 border-2 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
                                        <span class="text-sm font-bold uppercase tracking-widest">Retrieving Metrics...</span>
                                    </div>
                                {:else if metrics.length > 0}
                                    {#key expandedContainer}
                                        <div class="grid grid-cols-1 xl:grid-cols-2 gap-8">
                                            <div class="bg-white dark:bg-slate-800/50 p-4 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
                                                <MetricChart {metrics} title="CPU Utilization (6h)" type="cpu" />
                                            </div>
                                            <div class="bg-white dark:bg-slate-800/50 p-4 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
                                                <MetricChart {metrics} title="Memory footprint (6h)" type="memory" />
                                            </div>
                                        </div>
                                    {/key}

                                    <div class="mt-8 pt-6 border-t border-slate-100 dark:border-slate-800">
                                        <div class="flex items-center justify-between mb-4">
                                            <h4 class="text-xs font-black uppercase text-slate-400 tracking-widest flex items-center gap-2">
                                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                                                </svg>
                                                AI Performance consultant
                                            </h4>
                                            <button 
                                                onclick={() => analyzeMetrics(c.id)}
                                                disabled={aiAnalyzing}
                                                class="px-6 py-2 bg-brand-600 hover:bg-brand-700 text-white text-[9px] font-black uppercase tracking-widest rounded-xl transition-all shadow-lg shadow-brand-500/20 disabled:opacity-50"
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
                        <td colspan="9" class="px-6 py-12 text-center text-slate-400 italic">No containers found on socket</td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
    {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
        {#each safeContainers as c, i}
            <div 
                class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl overflow-hidden flex flex-col group hover:border-brand-500 transition-all opacity-0 animate-reveal"
                style="animation-delay: {0.1 + (i * 0.05)}s"
            >
                <div class="p-6 flex-1">
                    <div class="flex justify-between items-start mb-6">
                        <div class="flex flex-col min-w-0">
                            <button 
                                onclick={() => onNavigate('container-detail', { id: c.id })}
                                class="font-black text-slate-900 dark:text-white truncate text-xl tracking-tight hover:text-brand-600 transition-colors text-left"
                            >
                                {c.names?.[0]?.replace(/^\//, '') ?? 'unnamed'}
                            </button>
                            <span class="text-[10px] font-mono text-slate-400 mt-1 uppercase tracking-widest">{formatId(c.id)}</span>
                        </div>
                        <span class="px-2 py-1 rounded-lg text-[9px] font-black uppercase {stateColor(c.state)}">
                            {c.state}
                        </span>
                    </div>

                    <div class="space-y-6">
                        <div class="flex flex-col">
                            <span class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-1">Image Artifact</span>
                            <span class="text-xs text-slate-600 dark:text-slate-300 truncate font-bold" title={c.image}>{c.image}</span>
                        </div>

                        <div class="flex flex-col h-[40px] justify-center bg-slate-50 dark:bg-slate-900/50 rounded-xl p-2 border border-slate-100 dark:border-slate-800">
                            <Sparkline metrics={sparklineMetrics[c.id] || []} />
                        </div>

                        {#if c.updateAvailable}
                            <div class="p-3 bg-amber-50 dark:bg-amber-900/10 border border-amber-100 dark:border-amber-900/30 rounded-2xl flex items-center gap-3 animate-pulse">
                                <div class="w-2 h-2 rounded-full bg-amber-500"></div>
                                <span class="text-[9px] font-black text-amber-700 dark:text-amber-400 uppercase tracking-widest">Update Detected</span>
                            </div>
                        {/if}
                    </div>
                </div>

                <div class="px-6 py-4 bg-slate-50 dark:bg-slate-900/50 border-t border-slate-100 dark:border-slate-700 flex justify-between items-center">
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
                            class="px-4 py-2 text-[10px] font-black uppercase tracking-widest text-slate-500 hover:text-brand-600 transition-all"
                        >
                            Scan
                        </button>
                        <button 
                            onclick={() => onNavigate('container-detail', { id: c.id })}
                            class="px-5 py-2 bg-brand-600 text-white rounded-xl text-[10px] font-black uppercase tracking-widest shadow-lg shadow-brand-500/20 hover:bg-brand-700 transition-all"
                        >
                            Manage
                        </button>
                    </div>
                </div>
            </div>
        {:else}
            <div class="col-span-full py-12 text-center text-slate-400 italic bg-slate-50 dark:bg-slate-900/50 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-800">
                No containers found on socket
            </div>
        {/each}
    </div>
    {/if}
</div>
