<script lang="ts">
    import type { ReleaseRiskSummary } from "../api-types";

    let { releaseSummary, releaseError, repo = $bindable(), onAnalyze } = $props<{
        releaseSummary: ReleaseRiskSummary | null;
        releaseError: string;
        repo: string;
        onAnalyze: () => void;
    }>();

    const riskBand = (score: number) => score >= 80 ? "Critical" : score >= 60 ? "High" : score >= 30 ? "Medium" : score > 0 ? "Low" : "None";
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Release Intelligence</h2>
    </div>

    <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
        <div class="flex gap-2 max-w-xl">
            <input 
                bind:value={repo} 
                placeholder="owner/repo (e.g. Jellman86/HarborWatch)" 
                class="flex-1 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-2 text-sm focus:ring-2 focus:ring-brand-500 outline-none transition-all"
            />
            <button 
                onclick={onAnalyze}
                class="px-6 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-xl font-bold text-sm transition-all shadow-lg shadow-brand-500/20"
            >
                Analyze
            </button>
        </div>
        {#if releaseError}<p class="mt-2 text-xs text-rose-600 font-medium">{releaseError}</p>{/if}
    </div>

    {#if releaseSummary}
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <!-- Metrics Card -->
            <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-6">
                <div class="space-y-1">
                    <span class="text-xs font-bold text-slate-400 uppercase tracking-widest">Target Repository</span>
                    <h3 class="font-bold text-slate-900 dark:text-white truncate">{releaseSummary.repo}</h3>
                </div>

                <div class="p-4 rounded-2xl border {releaseSummary.breakingChangeLikely ? 'bg-rose-50 border-rose-100 dark:bg-rose-900/10 dark:border-rose-900/30' : 'bg-emerald-50 border-emerald-100 dark:bg-emerald-900/10 dark:border-emerald-900/30'}">
                    <div class="text-[10px] font-black uppercase mb-1 {releaseSummary.breakingChangeLikely ? 'text-rose-600' : 'text-emerald-600'}">Breaking Changes</div>
                    <div class="text-lg font-bold text-slate-900 dark:text-slate-100">
                        {releaseSummary.breakingChangeLikely ? 'High Probability Detected' : 'No Critical Flags'}
                    </div>
                </div>

                <div class="grid grid-cols-2 gap-4">
                    <div class="p-4 bg-slate-50 dark:bg-slate-900 rounded-2xl border border-slate-100 dark:border-slate-700">
                        <div class="text-[10px] font-black text-slate-400 uppercase mb-1">Risk Score</div>
                        <div class="text-2xl font-black text-slate-900 dark:text-white">{releaseSummary.totalRisk}</div>
                        <div class="text-[9px] font-bold text-slate-500 uppercase">{riskBand(releaseSummary.totalRisk)}</div>
                    </div>
                    <div class="p-4 bg-slate-50 dark:bg-slate-900 rounded-2xl border border-slate-100 dark:border-slate-700">
                        <div class="text-[10px] font-black text-slate-400 uppercase mb-1">Latest Tag</div>
                        <div class="text-2xl font-black text-slate-900 dark:text-white truncate">{releaseSummary.latestTag || 'N/A'}</div>
                        <div class="text-[9px] font-bold text-slate-500 uppercase">Production</div>
                    </div>
                </div>

                <div class="text-[10px] text-slate-400 font-medium">
                    Analysis based on {releaseSummary.releasesAnalyzed} releases. Generated on {new Date(releaseSummary.generatedAt * 1000).toLocaleString()}.
                </div>
            </div>

            <!-- Excerpts Panel -->
            <div class="lg:col-span-2 bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm flex flex-col h-[500px]">
                <div class="p-4 border-b border-slate-100 dark:border-slate-700 flex items-center justify-between">
                    <h3 class="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                        </svg>
                        Security & Change Excerpts
                    </h3>
                </div>
                <div class="flex-1 overflow-y-auto p-6 space-y-6 custom-scrollbar">
                    {#each releaseSummary.highlightedExcerpts as ex}
                        <div class="relative pl-6 border-l-2 border-brand-500/20 group">
                            <div class="absolute -left-[5px] top-0 w-2 h-2 rounded-full bg-brand-500 ring-4 ring-white dark:ring-slate-800"></div>
                            <div class="flex items-center gap-2 mb-2">
                                <span class="text-xs font-black text-brand-600 dark:text-brand-400 uppercase tracking-tighter">{ex.tag}</span>
                                <span class="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-700 rounded text-[8px] font-bold text-slate-500 uppercase">Weight: {ex.weight}</span>
                            </div>
                            <p class="text-sm text-slate-600 dark:text-slate-300 leading-relaxed bg-slate-50 dark:bg-slate-900/50 p-3 rounded-xl italic">
                                "{ex.text}"
                            </p>
                        </div>
                    {:else}
                        <div class="flex flex-col items-center justify-center h-full text-slate-400 gap-2">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 opacity-20" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                            </svg>
                            <span class="text-sm italic">No excerpts flagged for this repository</span>
                        </div>
                    {/each}
                </div>
            </div>
        </div>
    {/if}
</div>

<style>
    .custom-scrollbar::-webkit-scrollbar { width: 4px; }
    .custom-scrollbar::-webkit-scrollbar-track { @apply bg-transparent; }
    .custom-scrollbar::-webkit-scrollbar-thumb { @apply bg-slate-200 dark:bg-slate-700 rounded-full; }
</style>
