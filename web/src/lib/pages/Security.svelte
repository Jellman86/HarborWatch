<script lang="ts">
    import type { ScanSummary, MalwareScanSummary, ScanJobStatus } from "../api-types";

    let { 
        summary, 
        malwareSummaries, 
        activeJob, 
        scanError, 
        target = $bindable(), 
        malwareTarget = $bindable(),
        onStartScan, 
        onStartMalwareScan 
    } = $props<{
        summary: ScanSummary | null;
        malwareSummaries: MalwareScanSummary[];
        activeJob: ScanJobStatus | null;
        scanError: string;
        target: string;
        malwareTarget: string;
        onStartScan: () => void;
        onStartMalwareScan: () => void;
    }>();

    const riskBand = (score: number) => score >= 80 ? "Critical" : score >= 60 ? "High" : score >= 30 ? "Medium" : score > 0 ? "Low" : "None";
</script>

<div class="space-y-8">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Security Suite</h2>
    </div>

    {#if scanError}
        <div class="p-4 bg-rose-50 border border-rose-100 text-rose-700 rounded-xl text-sm flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
            </svg>
            {scanError}
        </div>
    {/if}

    {#if activeJob}
        <div class="p-4 bg-brand-50 border border-brand-100 dark:bg-brand-900/10 dark:border-brand-900/30 rounded-xl flex items-center justify-between">
            <div class="flex items-center gap-4">
                <div class="w-8 h-8 rounded-full border-2 border-brand-600 border-t-transparent animate-spin"></div>
                <div>
                    <div class="text-sm font-bold text-brand-900 dark:text-brand-300">Scanning {activeJob.target}...</div>
                    <div class="text-xs text-brand-600 uppercase font-black tracking-widest">{activeJob.source} in progress</div>
                </div>
            </div>
            <span class="text-xs font-mono text-slate-400">JOB: {activeJob.jobId}</span>
        </div>
    {/if}

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <!-- Vulnerability Scanner -->
        <section class="space-y-4">
            <div class="flex items-center gap-3 mb-2">
                <div class="p-2 bg-amber-100 dark:bg-amber-900/20 text-amber-600 dark:text-amber-400 rounded-lg">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                </div>
                <h3 class="text-lg font-bold text-slate-900 dark:text-white">Image Vulnerabilities (Trivy)</h3>
            </div>

            <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-6">
                <div class="flex gap-2">
                    <input 
                        bind:value={target} 
                        placeholder="image:tag" 
                        class="flex-1 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-2 text-sm focus:ring-2 focus:ring-brand-500 outline-none transition-all"
                    />
                    <button 
                        onclick={onStartScan}
                        disabled={!!activeJob}
                        class="px-6 py-2 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-xl font-bold text-sm transition-all shadow-lg shadow-brand-500/20"
                    >
                        Scan
                    </button>
                </div>

                {#if summary}
                    <div class="space-y-4">
                        <div class="flex items-center justify-between">
                            <span class="text-sm font-medium text-slate-500 italic truncate max-w-[250px]">{summary.target}</span>
                            <span class="px-2 py-1 bg-slate-100 dark:bg-slate-700 rounded text-[10px] font-bold uppercase">{riskBand(summary.riskScore)} Risk</span>
                        </div>
                        
                        <div class="grid grid-cols-3 gap-2">
                            <div class="p-3 bg-rose-50 dark:bg-rose-900/10 rounded-xl text-center">
                                <div class="text-xs font-bold text-rose-600 dark:text-rose-400 uppercase">Critical</div>
                                <div class="text-xl font-black text-rose-700 dark:text-rose-300">{summary.critical}</div>
                            </div>
                            <div class="p-3 bg-orange-50 dark:bg-orange-900/10 rounded-xl text-center">
                                <div class="text-xs font-bold text-orange-600 dark:text-orange-400 uppercase">High</div>
                                <div class="text-xl font-black text-orange-700 dark:text-orange-300">{summary.high}</div>
                            </div>
                            <div class="p-3 bg-amber-50 dark:bg-amber-900/10 rounded-xl text-center">
                                <div class="text-xs font-bold text-amber-600 dark:text-amber-400 uppercase">Medium</div>
                                <div class="text-xl font-black text-amber-700 dark:text-amber-300">{summary.medium}</div>
                            </div>
                        </div>

                        <div class="pt-4 border-t border-slate-50 dark:border-slate-700 flex justify-between items-center text-[10px] text-slate-400 font-bold uppercase tracking-widest">
                            <span>Last Scan: {new Date(summary.scannedAt * 1000).toLocaleString()}</span>
                            <span>Score: {summary.riskScore}/100</span>
                        </div>
                    </div>
                {:else}
                    <div class="py-8 text-center text-slate-400 italic text-sm">No vulnerability data for this target</div>
                {/if}
            </div>
        </section>

        <!-- Malware Scanner -->
        <section class="space-y-4">
            <div class="flex items-center gap-3 mb-2">
                <div class="p-2 bg-rose-100 dark:bg-rose-900/20 text-rose-600 dark:text-rose-400 rounded-lg">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                </div>
                <h3 class="text-lg font-bold text-slate-900 dark:text-white">Malware Shield (ClamAV)</h3>
            </div>

            <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-6">
                <div class="flex gap-2">
                    <input 
                        bind:value={malwareTarget} 
                        placeholder="/path/to/scan" 
                        class="flex-1 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-2 text-sm focus:ring-2 focus:ring-brand-500 outline-none transition-all"
                    />
                    <button 
                        onclick={onStartMalwareScan}
                        disabled={!!activeJob}
                        class="px-6 py-2 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-xl font-bold text-sm transition-all shadow-lg shadow-brand-500/20"
                    >
                        Scan
                    </button>
                </div>

                <div class="space-y-3 max-h-[300px] overflow-y-auto pr-2 custom-scrollbar">
                    {#each malwareSummaries as ms}
                        <div class="p-4 rounded-xl border transition-all {ms.infected ? 'bg-rose-50 border-rose-100 dark:bg-rose-900/10 dark:border-rose-900/30' : 'bg-emerald-50 border-emerald-100 dark:bg-emerald-900/10 dark:border-emerald-900/30'}">
                            <div class="flex items-center justify-between mb-2">
                                <span class="text-xs font-bold font-mono text-slate-500 truncate max-w-[200px]">{ms.target}</span>
                                <span class="text-[10px] font-black uppercase {ms.infected ? 'text-rose-600' : 'text-emerald-600'}">
                                    {ms.infected ? 'Infected' : 'Clean'}
                                </span>
                            </div>
                            {#if ms.infected && ms.threatsFound.length > 0}
                                <div class="mt-2 flex flex-wrap gap-1">
                                    {#each ms.threatsFound as threat}
                                        <span class="px-2 py-0.5 bg-rose-600 text-white text-[9px] font-bold rounded uppercase">{threat}</span>
                                    {/each}
                                </div>
                            {/if}
                            <div class="mt-2 text-[9px] text-slate-400 font-bold uppercase tracking-widest">
                                Scanned {new Date(ms.scannedAt * 1000).toLocaleDateString()}
                            </div>
                        </div>
                    {:else}
                        <div class="py-8 text-center text-slate-400 italic text-sm">No malware scan history</div>
                    {/each}
                </div>
            </div>
        </section>
    </div>
</div>

<style>
    .custom-scrollbar::-webkit-scrollbar { width: 4px; }
    .custom-scrollbar::-webkit-scrollbar-track { @apply bg-transparent; }
    .custom-scrollbar::-webkit-scrollbar-thumb { @apply bg-slate-200 dark:bg-slate-700 rounded-full; }
</style>
