<script lang="ts">
    import type { ContainerSummary } from "../api-types";

    let { containers } = $props<{
        containers: ContainerSummary[];
    }>();

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
                    <th class="px-6 py-4">ID</th>
                    <th class="px-6 py-4">Name</th>
                    <th class="px-6 py-4">Image</th>
                    <th class="px-6 py-4">State</th>
                    <th class="px-6 py-4">Status</th>
                    <th class="px-6 py-4">Policy</th>
                    <th class="px-6 py-4 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                {#each containers as c}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors group">
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
                        <td class="px-6 py-4 text-xs text-slate-500 dark:text-slate-500">
                            {c.status}
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
                {:else}
                    <tr>
                        <td colspan="6" class="px-6 py-12 text-center text-slate-400 italic">No containers found on socket</td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
</div>
