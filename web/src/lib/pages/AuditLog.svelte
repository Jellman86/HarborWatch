<script lang="ts">
    import { onMount } from "svelte";
    import type { AuditJobSummary, UpdateStepEvent } from "../api-types";

    let jobs = $state<AuditJobSummary[]>([]);
    let loading = $state(false);
    let error = $state("");

    let expandedJobId = $state<string | null>(null);
    let jobSteps = $state<UpdateStepEvent[]>([]);
    let loadingSteps = $state(false);

    async function loadAudit() {
        loading = true;
        error = "";
        try {
            const response = await fetch("/api/audit/jobs");
            if (!response.ok) throw new Error("Failed to fetch audit log");
            const data = await response.json();
            jobs = data || [];
        } catch (e) {
            error = e instanceof Error ? e.message : "Unknown error";
        } finally {
            loading = false;
        }
    }

    async function toggleExpand(id: string) {
        if (expandedJobId === id) {
            expandedJobId = null;
            jobSteps = [];
            return;
        }

        expandedJobId = id;
        loadingSteps = true;
        try {
            const res = await fetch(`/api/audit/jobs/${id}/steps`);
            if (res.ok) {
                const data = await res.json();
                jobSteps = data || [];
            }
        } catch (e) {
            console.error("Failed to load job steps", e);
        } finally {
            loadingSteps = false;
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
            aria-label="Refresh log"
            title="Refresh log"
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
                    <th class="px-6 py-4 w-10"></th>
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
                    <tr 
                        class="hover:bg-slate-50 dark:hover:bg-slate-700/30 transition-colors cursor-pointer {expandedJobId === job.id ? 'bg-slate-50/50 dark:bg-slate-900/30' : ''}"
                        onclick={() => toggleExpand(job.id)}
                    >
                        <td class="px-6 py-4">
                            <button 
                                class="text-slate-400"
                                aria-label={expandedJobId === job.id ? 'Collapse' : 'Expand'}
                            >
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 transition-transform {expandedJobId === job.id ? 'rotate-90' : ''}" viewBox="0 0 20 20" fill="currentColor">
                                    <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd" />
                                </svg>
                            </button>
                        </td>
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
                    {#if expandedJobId === job.id}
                        <tr class="bg-slate-50/20 dark:bg-slate-900/10">
                            <td colspan="7" class="px-12 py-6">
                                {#if loadingSteps}
                                    <div class="flex items-center gap-2 text-slate-400 text-xs font-bold uppercase tracking-widest animate-pulse">
                                        <div class="w-3 h-3 border-2 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
                                        Retrieving logs...
                                    </div>
                                {:else if jobSteps.length > 0}
                                    <div class="space-y-2 font-mono text-[10px]">
                                        {#each jobSteps as step}
                                            <div class="flex gap-4 border-l-2 border-slate-200 dark:border-slate-700 pl-4 py-1">
                                                <span class="text-slate-400 w-24">{new Date(step.timestamp * 1000).toLocaleTimeString()}</span>
                                                <span class="font-bold text-brand-600 dark:text-brand-400 uppercase w-20">{step.step}</span>
                                                <span class={step.status === 'failed' ? 'text-rose-500' : 'text-slate-600 dark:text-slate-300'}>
                                                    {step.message}
                                                </span>
                                            </div>
                                        {/each}
                                    </div>
                                {:else}
                                    <p class="text-[10px] text-slate-400 italic">No detailed step logs found for this job type.</p>
                                {/if}
                            </td>
                        </tr>
                    {/if}
                {:else}
                    <tr>
                        <td colspan="7" class="px-6 py-12 text-center text-slate-400 italic text-sm">
                            {loading ? 'Fetching records...' : 'No historical jobs found'}
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
</div>
