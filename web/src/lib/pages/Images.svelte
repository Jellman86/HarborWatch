<script lang="ts">
    import { onMount } from "svelte";
    import type { ImageSummary } from "../api-types";

    let images = $state<ImageSummary[]>([]);
    let loading = $state(true);
    let error = $state("");
    let pruning = $state(false);

    async function loadImages() {
        loading = true;
        error = "";
        try {
            const res = await fetch("/api/docker/images");
            if (res.ok) {
                const data = await res.json();
                images = data || [];
            } else {
                error = "Failed to load images";
            }
        } catch (e) {
            error = "Could not connect to backend";
        } finally {
            loading = false;
        }
    }

    async function pruneImages() {
        if (!confirm("Are you sure you want to trigger a system-wide image prune? This will remove all unused images.")) return;
        
        pruning = true;
        try {
            const res = await fetch("/api/docker/prune", { method: "POST" });
            if (res.ok) {
                await loadImages();
            }
        } catch (e) {
            console.error("Prune failed", e);
        } finally {
            pruning = false;
        }
    }

    onMount(() => {
        loadImages();
    });

    const formatSize = (bytes: number) => {
        const mb = bytes / (1024 * 1024);
        return mb > 1024 ? `${(mb / 1024).toFixed(2)} GB` : `${mb.toFixed(1)} MB`;
    };

    const formatId = (id: string) => id.replace('sha256:', '').slice(0, 12);
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">Image Repository</h2>
            <p class="text-xs text-slate-500 font-medium">Local artifact storage and versioning history.</p>
        </div>
        <div class="flex items-center gap-3">
            <button 
                onclick={loadImages}
                class="p-2 text-slate-400 hover:text-brand-600 transition-colors"
                title="Refresh Registry"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
            </button>
            <button 
                onclick={pruneImages}
                disabled={pruning}
                class="px-4 py-2 bg-rose-600 hover:bg-rose-700 text-white rounded-xl text-[10px] font-black uppercase tracking-widest transition-all shadow-lg shadow-rose-500/20 disabled:opacity-50 flex items-center gap-2"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
                {pruning ? 'Pruning...' : 'Cleanup Repository'}
            </button>
        </div>
    </div>

    {#if loading}
        <div class="flex flex-col items-center justify-center py-20 gap-4 text-slate-400">
            <div class="w-8 h-8 border-4 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
            <span class="text-sm font-black uppercase tracking-widest">Scanning Registry...</span>
        </div>
    {:else if error}
        <div class="bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 rounded-3xl p-12 text-center animate-reveal">
            <p class="text-rose-600 font-bold uppercase tracking-widest text-xs">{error}</p>
        </div>
    {:else if images.length === 0}
        <div class="bg-white dark:bg-slate-800 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-700 p-16 text-center animate-reveal">
            <p class="text-slate-400 italic font-medium">No images found in local repository.</p>
        </div>
    {:else}
        <div class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl overflow-hidden opacity-0 animate-reveal stagger-1">
            <table class="w-full text-left border-collapse">
                <thead>
                    <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 text-[10px] font-black uppercase tracking-widest border-b border-slate-100 dark:border-slate-700">
                        <th class="px-8 py-4">Artifact ID</th>
                        <th class="px-8 py-4">Repository / Tag</th>
                        <th class="px-8 py-4">Storage Size</th>
                        <th class="px-8 py-4">Created</th>
                        <th class="px-8 py-4">Usage</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                    {#each images as img, i}
                        <tr 
                            class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors opacity-0 animate-reveal"
                            style="animation-delay: {0.1 + (i * 0.03)}s"
                        >
                            <td class="px-8 py-4 font-mono text-[10px] text-slate-400">{formatId(img.id)}</td>
                            <td class="px-8 py-4">
                                <div class="flex flex-col">
                                    <span class="font-bold text-slate-900 dark:text-white truncate max-w-[300px]">
                                        {img.repoTags?.[0]?.split(':')[0] || '<none>'}
                                    </span>
                                    <span class="text-[10px] font-black text-brand-600 uppercase tracking-tighter">
                                        {img.repoTags?.[0]?.split(':')[1] || 'latest'}
                                    </span>
                                </div>
                            </td>
                            <td class="px-8 py-4 font-mono text-[10px] text-slate-500">{formatSize(img.size)}</td>
                            <td class="px-8 py-4 text-[11px] text-slate-400">{new Date(img.created * 1000).toLocaleDateString()}</td>
                            <td class="px-8 py-4">
                                <span class="px-2 py-1 bg-slate-100 dark:bg-slate-800 rounded-md text-[9px] font-black uppercase text-slate-500 dark:text-slate-400">
                                    {img.containers > 0 ? `${img.containers} Instances` : 'Dangling'}
                                </span>
                            </td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    {/if}
</div>
