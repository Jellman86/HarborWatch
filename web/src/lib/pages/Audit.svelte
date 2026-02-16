<script lang="ts">
    import { onMount } from "svelte";
    import type { AuditJobSummary } from "../api-types";

    let jobs = $state<AuditJobSummary[]>([]);
    let loading = $state(true);
    let error = $state("");

    async function loadAudit() {
        loading = true;
        error = "";
        try {
            const res = await fetch("/api/audit/jobs");
            if (res.ok) {
                const data = await res.json();
                jobs = data || [];
            } else {
                error = "Failed to load audit trail";
            }
        } catch (e) {
            error = "Could not connect to backend";
        } finally {
            loading = false;
        }
    }

    onMount(() => {
        loadAudit();
    });

    const statusColor = (status: string) => {
        switch(status.toLowerCase()) {
            case 'completed': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400';
            case 'failed': return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400';
            case 'running': return 'bg-brand-100 text-brand-700 dark:bg-brand-900/30 dark:text-brand-400 animate-pulse';
            default: return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-400';
        }
    };
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">Audit Trail</h2>
            <p class="text-xs text-slate-500 font-medium">Historical record of all system operations and automated tasks.</p>
        </div>
        <button 
            onclick={loadAudit}
            class="px-4 py-2 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-xl font-bold transition-all border border-slate-200 dark:border-slate-700 shadow-sm flex items-center gap-2"
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
            <span class="text-sm font-black uppercase tracking-widest">Querying History...</span>
        </div>
    {:else if error}
        <div class="bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 rounded-3xl p-12 text-center animate-reveal">
            <p class="text-rose-600 font-bold uppercase tracking-widest text-xs">{error}</p>
        </div>
    {:else if jobs.length === 0}
        <div class="bg-white dark:bg-slate-800 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-700 p-16 text-center animate-reveal">
            <p class="text-slate-400 italic font-medium">No system actions recorded yet.</p>
        </div>
    {:else}
        <div class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl overflow-hidden opacity-0 animate-reveal stagger-1">
            <table class="w-full text-left border-collapse">
                <thead>
                    <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 text-[10px] font-black uppercase tracking-widest border-b border-slate-100 dark:border-slate-700">
                        <th class="px-8 py-4">Timestamp</th>
                        <th class="px-8 py-4">Action Type</th>
                        <th class="px-8 py-4">Target Resource</th>
                        <th class="px-8 py-4">Result Status</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                    {#each jobs as job, i}
                        <tr 
                            class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors opacity-0 animate-reveal"
                            style="animation-delay: {0.1 + (i * 0.03)}s"
                        >
                            <td class="px-8 py-4 text-[11px] text-slate-400 font-medium">
                                {new Date(job.startedAt * 1000).toLocaleString()}
                            </td>
                            <td class="px-8 py-4">
                                <span class="font-bold text-slate-900 dark:text-white uppercase tracking-tight text-xs">
                                    {job.type}
                                </span>
                            </td>
                            <td class="px-8 py-4 font-mono text-[10px] text-slate-500 truncate max-w-[250px]" title={job.target}>
                                {job.target}
                            </td>
                            <td class="px-8 py-4">
                                <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase {statusColor(job.status)}">
                                    {job.status}
                                </span>
                            </td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    {/if}
</div>
