<script lang="ts">
    import type { ImageSummary } from "../api-types";

    let { images } = $props<{
        images: ImageSummary[];
    }>();

    const formatId = (id: string) => (id.length > 12 ? id.replace('sha256:', '').slice(0, 12) : id);
    const formatSize = (size: number) => size > 1024 * 1024 * 1024 ? `${(size / 1073741824).toFixed(2)} GB` : size > 1024 * 1024 ? `${(size / 1048576).toFixed(2)} MB` : `${size} B`;
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Image Repository</h2>
        <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-xs font-bold text-slate-600 dark:text-slate-400 uppercase">
            {images.length} Local Images
        </span>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {#each images as i}
            <div class="bg-white dark:bg-slate-800 p-5 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm hover:border-brand-500/50 transition-all group">
                <div class="flex justify-between items-start mb-3">
                    <div class="flex flex-col overflow-hidden">
                        <span class="text-xs font-mono text-slate-400 mb-1">{formatId(i.id)}</span>
                        <h3 class="font-bold text-slate-900 dark:text-white truncate" title={i.repoTags?.[0] ?? '<none>:<none>'}>
                            {i.repoTags?.[0] ?? '<none>:<none>'}
                        </h3>
                    </div>
                    <span class="px-2 py-1 bg-slate-50 dark:bg-slate-900 rounded-lg text-[10px] font-bold text-slate-500">
                        {formatSize(i.size)}
                    </span>
                </div>
                
                <div class="flex justify-end gap-2 mt-4 pt-4 border-t border-slate-50 dark:border-slate-700/50">
                    <button class="text-[10px] font-black uppercase text-slate-400 hover:text-brand-600 dark:hover:text-brand-400 transition-colors">
                        Inspect
                    </button>
                    <button class="text-[10px] font-black uppercase text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 transition-colors">
                        Delete
                    </button>
                </div>
            </div>
        {:else}
            <div class="col-span-full py-12 text-center text-slate-400 italic bg-slate-50 dark:bg-slate-900/50 rounded-2xl border-2 border-dashed border-slate-200 dark:border-slate-800">
                No images detected
            </div>
        {/each}
    </div>
</div>
