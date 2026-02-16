<script lang="ts">
    import { onMount } from "svelte";
    import type { AuditJobSummary } from "../api-types";

    let jobs = $state<AuditJobSummary[]>([]);
    let loading = $state(false);
    let error = $state("");

    async function loadAudit() {
        loading = true;
        error = "";
        try {
            const response = await fetch("/api/audit/jobs");
            if (!response.ok) throw new Error("Failed to fetch audit log");
            jobs = await response.json();
        } catch (e) {
            error = e instanceof Error ? e.message : "Unknown error";
        } finally {
            loading = false;
        }
    }

    onMount(() => {
        loadAudit();
    });

    const formatTime = (ts: number) => new Date(ts * 1000).toLocaleString();
    const duration = (start: number, end?: number) => {
        if (!end) return "Running...";
        const diff = end - start;
        if (diff < 60) return `${diff}s`;
        return `${Math.floor(diff / 60)}m ${diff % 60}s`;
    };

    const statusColor = (status: string) => {
        switch(status.toLowerCase()) {
            case 'completed':
            case 'success': return 'text-emerald-600 bg-emerald-50 dark:bg-emerald-900/20';
            case 'failed':
            case 'error': return 'text-rose-600 bg-rose-50 dark:bg-rose-900/20';
            case 'running': return 'text-brand-600 bg-brand-50 dark:bg-brand-900/20 animate-pulse';
            case 'rolled_back': return 'text-amber-600 bg-amber-50 dark:bg-amber-900/20';
            default: return 'text-slate-500 bg-slate-100 dark:bg-slate-800';
        }
    };
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <div>
            <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Operations Log</h2>
            <p class="text-sm text-slate-500 mt-1">Audit trail of all system maintenance and security tasks.</p>
        </div>
        <button 
            onclick={loadAudit}
            disabled={loading}
            class="px-4 py-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-sm font-bold shadow-sm hover:bg-slate-50 transition-all flex items-center gap-2"
        >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 {loading ? 'animate-spin' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Refresh
        </button>
    </div>

    {#if error}
        <div class="p-4 bg-rose-50 border border-rose-100 text-rose-700 rounded-xl text-sm">{error}</div>
    {/if}

    <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden">
        <table class="w-full text-left border-collapse">
            <thead>
                <tr class="bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 text-[10px] font-black uppercase tracking-widest">
                    <th class="px-6 py-4">Job ID</th>
                    <th class="px-6 py-4">Type</th>
                    <th class="px-6 py-4">Target</th>
                    <th class="px-6 py-4">Status</th>
                    <th class="px-6 py-4">Started</th>
                    <th class="px-6 py-4">Duration</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-700">
                {#each jobs as job}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors">
                        <td class="px-6 py-4 font-mono text-[10px] text-slate-400">{job.id}</td>
                        <td class="px-6 py-4">
                            <span class="text-xs font-bold text-slate-700 dark:text-slate-200">{job.type}</span>
                        </td>
                        <td class="px-6 py-4">
                            <code class="text-[10px] bg-slate-100 dark:bg-slate-900 px-1.5 py-0.5 rounded text-slate-600 dark:text-slate-400">{job.target}</code>
                        </td>
                        <td class="px-6 py-4">
                            <span class="px-2 py-1 rounded text-[10px] font-black uppercase {statusColor(job.status)}">
                                {job.status}
                            </span>
                        </td>
                        <td class="px-6 py-4 text-xs text-slate-500">{formatTime(job.startedAt)}</td>
                        <td class="px-6 py-4 text-xs font-medium text-slate-500">{duration(job.startedAt, job.completedAt)}</td>
                    </tr>
                {:else}
                    <tr>
                        <td colspan="6" class="px-6 py-12 text-center text-slate-400 italic text-sm">
                            {loading ? 'Fetching records...' : 'No historical jobs found'}
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
</div>
