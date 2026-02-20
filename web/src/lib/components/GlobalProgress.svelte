<script lang="ts">
    import type { JobProgress } from "../api-types";
    import { slide, fly } from "svelte/transition";

    let { jobs = [] } = $props<{
        jobs: JobProgress[];
    }>();

    // Calculate aggregated progress (average of all active jobs)
    let aggregateProgress = $derived(
        jobs.length > 0 
            ? Math.round(jobs.reduce((acc, job) => acc + (job.progress || 0), 0) / jobs.length)
            : 0
    );

    const formatTarget = (t: string) => t.replace(/^\//, '');

    const jobVerb = (type: string) => {
        if (type === "update") return "Updating";
        if (type === "scan:trivy") return "Scanning (Vulnerability)";
        if (type === "scan:clamav") return "Scanning (Malware)";
        return "Processing";
    };

    let summaryLabel = $derived.by(() => {
        if (jobs.length === 0) return "";
        if (jobs.length === 1) {
            return `${jobVerb(jobs[0].type)} ${formatTarget(jobs[0].target)}`;
        }
        
        const types = [...new Set(jobs.map(j => j.type))];
        const targets = jobs.map(j => formatTarget(j.target));
        
        if (types.length === 1) {
            const verb = jobVerb(types[0]);
            if (targets.length === 2) return `${verb} ${targets[0]} & ${targets[1]}`;
            return `${verb} ${targets[0]} and ${targets.length - 1} others`;
        }

        return `${jobs.length} background tasks in progress`;
    });

    let currentMessage = $derived.by(() => {
        if (jobs.length === 0) return "";
        if (jobs.length === 1) return jobs[0].message || jobs[0].status;
        
        // For multiple, show the message of the most recently updated or just a generic one
        // Let's just show the last job's message if it has one
        const withMsg = jobs.filter(j => j.message);
        if (withMsg.length > 0) return withMsg[0].message;
        return "Multiple tasks running...";
    });

    let showDetails = $state(false);
</script>

{#if jobs.length > 0}
    <div 
        class="w-full bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 overflow-hidden relative group"
        transition:slide={{ duration: 300 }}
        onmouseenter={() => showDetails = true}
        onmouseleave={() => showDetails = false}
        role="status"
        aria-live="polite"
    >
        <!-- Subtle pulse background -->
        <div class="absolute inset-0 bg-brand-500/5 animate-pulse pointer-events-none"></div>

        <div class="max-w-[120rem] mx-auto px-4 md:px-8 py-3 relative z-10">
            <div class="flex flex-col gap-2">
                <div class="flex items-center justify-between gap-4">
                    <div class="flex items-center gap-3 min-w-0">
                        <div class="w-6 h-6 rounded-lg bg-brand-100 dark:bg-brand-900/30 flex items-center justify-center text-brand-600 dark:text-brand-400 flex-shrink-0">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                            </svg>
                        </div>
                        <div class="min-w-0 flex flex-col md:flex-row md:items-baseline md:gap-3">
                            <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-tight truncate">
                                {summaryLabel}
                            </p>
                            {#if currentMessage}
                                <p class="text-[9px] font-bold text-brand-600 dark:text-brand-400 uppercase tracking-widest truncate max-w-[200px] md:max-w-md">
                                    {currentMessage}
                                </p>
                            {/if}
                        </div>
                    </div>
                    
                    <div class="flex items-center gap-2">
                        <p class="text-[9px] font-bold text-slate-400 uppercase tracking-widest whitespace-nowrap">
                            {aggregateProgress}% Total
                        </p>
                        <div class="hidden md:flex items-center gap-1">
                            {#each jobs as job}
                                <div 
                                    class="w-1.5 h-1.5 rounded-full bg-brand-500/40" 
                                    title={`${jobVerb(job.type)} ${formatTarget(job.target)}: ${job.message || job.status} (${job.progress}%)`}
                                ></div>
                            {/each}
                        </div>
                    </div>
                </div>

                <!-- Aggregated Progress Bar -->
                <div class="h-1 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                    <div 
                        class="h-full bg-brand-500 transition-all duration-700 ease-out rounded-full shadow-[0_0_8px_rgba(var(--color-brand-500),0.5)]"
                        style="width: {aggregateProgress}%"
                    ></div>
                </div>

                <!-- Detailed View (Expanded on Hover or if multiple) -->
                {#if showDetails && jobs.length > 1}
                    <div class="pt-2 border-t border-slate-100 dark:border-slate-800/50 mt-1 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-2" in:slide>
                        {#each jobs as job (job.id)}
                            <div class="flex items-center justify-between gap-3 text-[9px] min-w-0">
                                <span class="text-slate-500 font-bold uppercase truncate flex-1">
                                    {formatTarget(job.target)}
                                </span>
                                <span class="text-slate-400 truncate flex-1 text-right">
                                    {job.message || job.status}
                                </span>
                                <span class="font-black text-brand-600 dark:text-brand-400 w-6 text-right">
                                    {job.progress}%
                                </span>
                            </div>
                        {/each}
                    </div>
                {/if}
            </div>
        </div>
    </div>
{/if}
