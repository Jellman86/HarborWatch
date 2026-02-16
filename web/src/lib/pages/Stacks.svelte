<script lang="ts">
    import { onMount } from "svelte";

    // Component State
    let stacks = $state<any[]>([]);
    let loading = $state(true);
    let error = $state("");

    async function loadStacks() {
        loading = true;
        error = "";
        try {
            const res = await fetch("/api/portainer/stacks");
            if (res.ok) {
                stacks = await res.json();
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
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Stack Explorer</h2>
        <button 
            onclick={loadStacks}
            class="px-4 py-2 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-lg font-semibold transition-colors flex items-center gap-2"
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
        <div class="bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 rounded-2xl p-8 text-center space-y-4">
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
        <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 p-12 text-center">
            <p class="text-slate-400 italic">No stacks discovered in Portainer.</p>
        </div>
    {:else}
        <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden">
            <table class="w-full text-left border-collapse">
                <thead>
                    <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 text-xs font-bold uppercase tracking-wider">
                        <th class="px-6 py-4">ID</th>
                        <th class="px-6 py-4">Name</th>
                        <th class="px-6 py-4">Type</th>
                        <th class="px-6 py-4">Endpoint</th>
                        <th class="px-6 py-4">Status</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                    {#each stacks as s}
                        <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors">
                            <td class="px-6 py-4 font-mono text-xs text-slate-400">{s.Id}</td>
                            <td class="px-6 py-4 font-bold text-slate-900 dark:text-white">{s.Name}</td>
                            <td class="px-6 py-4 text-sm text-slate-600 dark:text-slate-400">{stackType(s.Type)}</td>
                            <td class="px-6 py-4 font-mono text-xs text-slate-400">EP-{s.EndpointId}</td>
                            <td class="px-6 py-4">
                                <span class="px-2 py-1 rounded-md text-[10px] font-black uppercase {statusColor(s.Status)}">
                                    {s.Status === 1 ? 'Active' : 'Inactive'}
                                </span>
                            </td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    {/if}
</div>
