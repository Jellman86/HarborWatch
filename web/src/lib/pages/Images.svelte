<script lang="ts">
    import type { ImageSummary } from "../api-types";

    let { images } = $props<{
        images: ImageSummary[];
    }>();

    let pruning = $state(false);

    const formatId = (id: string) => (id.length > 12 ? id.replace('sha256:', '').slice(0, 12) : id);
    const formatSize = (size: number) => size > 1024 * 1024 * 1024 ? `${(size / 1073741824).toFixed(2)} GB` : size > 1024 * 1024 ? `${(size / 1048576).toFixed(2)} MB` : `${size} B`;

    async function handlePrune() {
        if (!confirm("Are you sure you want to prune unused images? This will remove all dangling images.")) return;
        
        pruning = true;
        try {
            const res = await fetch("/api/scheduler/run", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ id: "docker_system_prune" })
            });
            if (res.ok) {
                alert("Prune task triggered successfully.");
            }
        } catch (e) {
            alert("Failed to trigger prune task.");
        } finally {
            pruning = false;
        }
    }
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <div>
            <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Image Repository</h2>
            <p class="text-sm text-slate-500 mt-1">Manage local container images and artifacts.</p>
        </div>
        <div class="flex items-center gap-3">
            <button 
                onclick={handlePrune}
                disabled={pruning}
                class="px-4 py-2 bg-rose-50 hover:bg-rose-100 dark:bg-rose-900/20 dark:hover:bg-rose-900/30 text-rose-600 dark:text-rose-400 rounded-xl font-bold text-xs transition-all flex items-center gap-2 border border-rose-100 dark:border-rose-900/30 disabled:opacity-50"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
                {pruning ? 'Pruning...' : 'Cleanup Repository'}
            </button>
            <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-xs font-bold text-slate-600 dark:text-slate-400 uppercase">
                {images.length} Total
            </span>
        </div>
    </div>

    <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden">
        <table class="w-full text-left border-collapse">
            <thead>
                <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 text-xs font-bold uppercase tracking-wider">
                    <th class="px-6 py-4">Image ID</th>
                    <th class="px-6 py-4">Repository Tag</th>
                    <th class="px-6 py-4">Size</th>
                    <th class="px-6 py-4 text-right">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                {#each images as i}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors group">
                        <td class="px-6 py-4 font-mono text-xs text-slate-400">{formatId(i.id)}</td>
                        <td class="px-6 py-4">
                            <div class="flex flex-col">
                                <span class="font-bold text-slate-900 dark:text-white truncate max-w-md">
                                    {i.repoTags?.[0] ?? '<none>:<none>'}
                                </span>
                                {#if i.repoTags && i.repoTags.length > 1}
                                    <span class="text-[10px] text-slate-400">+{i.repoTags.length - 1} more tags</span>
                                {/if}
                            </div>
                        </td>
                        <td class="px-6 py-4">
                            <span class="text-sm font-medium text-slate-600 dark:text-slate-400">
                                {formatSize(i.size)}
                            </span>
                        </td>
                        <td class="px-6 py-4 text-right">
                            <div class="flex justify-end gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                                <button class="p-1.5 text-slate-400 hover:text-brand-600 hover:bg-brand-50 dark:hover:bg-brand-900/20 rounded-lg transition-all" title="Inspect">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                                    </svg>
                                </button>
                                <button class="p-1.5 text-slate-400 hover:text-rose-600 hover:bg-rose-50 dark:hover:bg-brand-900/20 rounded-lg transition-all" title="Delete">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                    </svg>
                                </button>
                            </div>
                        </td>
                    </tr>
                {:else}
                    <tr>
                        <td colspan="4" class="px-6 py-12 text-center text-slate-400 italic">
                            No local images found.
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
</div>
