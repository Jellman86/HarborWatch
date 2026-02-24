<script lang="ts">
    import { onMount } from "svelte";
    import PaginationBar from "../components/PaginationBar.svelte";
    import { toasts } from "../stores/ToastStore";

    let { onNavigate } = $props<{
        onNavigate: (route: string, params?: any) => void;
    }>();

    // Component State
    let stacks = $state<any[]>([]);
    let loading = $state(true);
    let redeploying = $state<Record<number, boolean>>({});
    let error = $state("");
    let pageIndex = $state(0);
    const pageSize = 12;

    async function loadStacks() {
        loading = true;
        error = "";
        try {
            const res = await fetch("/api/portainer/stacks");
            if (res.ok) {
                const data = await res.json();
                stacks = data || [];
            } else if (res.status === 503) {
                error = "Portainer integration is not configured. Enable it in Settings.";
            } else {
                const data = await res.json();
                error = data.message || "Failed to load stacks";
            }
        } catch (e) {
            error = "Could not connect to backend";
        } finally {
            loading = false;
        }
    }

    async function redeployStack(id: number) {
        if (redeploying[id]) return;
        redeploying[id] = true;
        toasts.info(`Triggering redeploy for stack ${id}...`);
        try {
            const res = await fetch(`/api/portainer/stacks/${id}/redeploy`, {
                method: "POST"
            });
            const data = await res.json();
            if (res.ok) {
                toasts.success(data.message || "Redeploy triggered successfully");
                await loadStacks();
            } else {
                toasts.error(data.message || "Redeploy failed");
            }
        } catch (e) {
            toasts.error("Failed to trigger redeploy");
        } finally {
            redeploying[id] = false;
        }
    }

    onMount(() => {
        loadStacks();
    });

    const stackType = (type: number) => {
        switch(type) {
            case 1: return "Swarm";
            case 2: return "Compose";
            default: return "Unknown";
        }
    };

    const statusColor = (status: number) => {
        switch(status) {
            case 1: return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"; // Active
            case 2: return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400"; // Inactive
            default: return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-400";
        }
    };

    const totalPages = $derived(Math.max(1, Math.ceil(stacks.length / pageSize)));
    const pagedStacks = $derived(stacks.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));
    const pageStart = $derived(stacks.length === 0 ? 0 : (pageIndex * pageSize) + 1);
    const pageEnd = $derived(Math.min((pageIndex + 1) * pageSize, stacks.length));

    $effect(() => {
        stacks.length;
        if (pageIndex > totalPages - 1) {
            pageIndex = Math.max(0, totalPages - 1);
        }
    });
</script>

<style>
    .stack-card-container {
        position: relative;
        z-index: 1;
        border-radius: 1.5rem;
        box-shadow: none;
    }
    .stack-card-container::before,
    .stack-card-container::after {
        content: '';
        position: absolute;
        border-radius: 1.5rem; /* Matches rounded-3xl */
        border: 1px solid theme('colors.slate.200');
        background: theme('colors.slate.50');
        transition: all 0.3s ease;
    }

    .stack-card-container::before {
        top: 8px;
        left: 8px;
        right: -8px;
        bottom: -8px;
        z-index: -1;
    }
    .stack-card-container::after {
        top: 16px;
        left: 16px;
        right: -16px;
        bottom: -16px;
        z-index: -2;
        opacity: 0.5;
        box-shadow: 0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1);
    }

    :global(.dark) .stack-card-container,
    :global(.dark) .stack-card-container::before,
    :global(.dark) .stack-card-container::after {
        border-color: theme('colors.slate.700');
        background: theme('colors.slate.800');
        box-shadow: none;
    }

    :global(.dark) .stack-card-container::after {
        box-shadow: 0 10px 15px -3px rgb(0 0 0 / 0.4);
    }
    
    .stack-card-container:hover::before {
        top: 10px;
        left: 10px;
        right: -10px;
        bottom: -10px;
    }
    .stack-card-container:hover::after {
        top: 20px;
        left: 20px;
        right: -20px;
        bottom: -20px;
    }
</style>

<div class="space-y-6">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">Stack Explorer</h2>
            <p class="text-xs text-slate-500 font-medium">Remote orchestration discovery via Portainer API.</p>
        </div>
        <button 
            onclick={loadStacks}
            class="w-full md:w-auto px-4 py-2 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-xl font-bold transition-all border border-slate-200 dark:border-slate-700 shadow-sm flex items-center justify-center gap-2"
        >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Refresh
        </button>
    </div>

    {#if loading}
        <div class="flex flex-col items-center justify-center py-20 gap-4 text-slate-400">
            <div class="w-8 h-8 border-4 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
            <span class="text-sm font-black uppercase tracking-widest">Querying Portainer...</span>
        </div>
    {:else if error}
        <div class="bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 rounded-3xl p-8 text-center space-y-4 animate-reveal">
            <div class="w-16 h-16 bg-rose-100 dark:bg-rose-900/30 rounded-full flex items-center justify-center mx-auto text-rose-600 dark:text-rose-400">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
            </div>
            <div>
                <h3 class="text-lg font-bold text-slate-900 dark:text-white">Integration Error</h3>
                <p class="text-sm text-slate-500 mt-1 max-w-md mx-auto">{error}</p>
            </div>
        </div>
    {:else if stacks.length === 0}
        <div class="bg-white dark:bg-slate-800 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-700 p-16 text-center animate-reveal">
            <p class="text-slate-400 italic font-medium">No stacks discovered in the configured Portainer endpoint.</p>
        </div>
    {:else}
        <PaginationBar
            summaryText={`Showing ${pageStart}-${pageEnd} of ${stacks.length}`}
            pageText={`Page ${pageIndex + 1}/${totalPages}`}
            canPrev={pageIndex > 0}
            canNext={pageIndex < totalPages - 1}
            onPrev={() => pageIndex = Math.max(0, pageIndex - 1)}
            onNext={() => pageIndex = Math.min(totalPages - 1, pageIndex + 1)}
        />
        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-10 opacity-0 animate-reveal stagger-1">
            {#each pagedStacks as s, i}
                <div 
                    class="stack-card-container opacity-0 animate-reveal"
                    style="animation-delay: {0.1 + (i * 0.05)}s"
                >
                    <article class="bg-white dark:bg-slate-800 rounded-[1.5rem] border border-slate-200 dark:border-slate-700 overflow-hidden h-full flex flex-col group hover:border-brand-500 transition-all">
                        <div class="p-6 flex-1 space-y-4 text-left">
                            <div class="flex items-start justify-between gap-3">
                                <div class="min-w-0 flex-1">
                                    <h3 class="font-black text-slate-900 dark:text-white truncate text-xl tracking-tight group-hover:text-brand-600 transition-colors" title={s.Name}>
                                        {s.Name}
                                    </h3>
                                    <p class="text-[10px] font-mono text-slate-400 mt-1 uppercase tracking-widest">ID: {s.Id}</p>
                                </div>
                                <span class="px-2.5 py-1 rounded-full text-[10px] font-black uppercase tracking-widest {statusColor(s.Status)}">
                                    {s.Status === 1 ? 'Active' : 'Inactive'}
                                </span>
                            </div>

                            <div class="grid grid-cols-2 gap-4 py-2 border-y border-slate-100 dark:border-slate-700/50">
                                <div class="space-y-1">
                                    <p class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Engine Type</p>
                                    <p class="text-xs font-bold text-slate-700 dark:text-slate-200">{stackType(s.Type)}</p>
                                </div>
                                <div class="space-y-1">
                                    <p class="text-[9px] font-black text-slate-400 uppercase tracking-widest">Endpoint</p>
                                    <p class="text-xs font-bold text-slate-700 dark:text-slate-200 uppercase tracking-tighter">EP-{s.EndpointId}</p>
                                </div>
                            </div>

                            <div class="flex items-center gap-2 pt-2">
                                <span class="px-2 py-1 bg-brand-50 dark:bg-brand-900/20 rounded-lg text-[9px] font-black text-brand-700 dark:text-brand-400 uppercase border border-brand-100 dark:border-brand-900/30">
                                    Compose Project
                                </span>
                            </div>
                        </div>

                        <div class="px-6 py-4 bg-slate-50 dark:bg-slate-900/50 border-t border-slate-100 dark:border-slate-700 flex justify-between items-center">
                            <button 
                                onclick={() => onNavigate('containers', { search: s.Name })}
                                class="px-4 py-2 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 text-[10px] font-black uppercase tracking-widest rounded-xl transition-all border border-slate-200 dark:border-slate-700 shadow-sm"
                            >
                                Manage Fleet
                            </button>
                            {#if s.Type === 2}
                                <button 
                                    onclick={() => redeployStack(s.Id)}
                                    disabled={redeploying[s.Id]}
                                    class="px-5 py-2.5 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest rounded-xl transition-all shadow-lg shadow-brand-500/20 flex items-center gap-2"
                                >
                                    {#if redeploying[s.Id]}
                                        <div class="w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                                    {:else}
                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                        </svg>
                                    {/if}
                                    Redeploy
                                </button>
                            {/if}
                        </div>
                    </article>
                </div>
            {/each}
        </div>
    {/if}
</div>
