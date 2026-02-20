<script lang="ts">
    import type { JobProgress } from "../api-types";
    import { slide } from "svelte/transition";

    let { jobs = [] } = $props<{
        jobs: JobProgress[];
    }>();

    // Calculate aggregated progress (average of all active jobs)
    let aggregateProgress = $derived(
        jobs.length > 0 
            ? Math.round(jobs.reduce((acc, job) => acc + (job.progress || 0), 0) / jobs.length)
            : 0
    );

    let summaryLabel = $derived.by(() => {
        if (jobs.length === 0) return "";
        if (jobs.length === 1) {
            const job = jobs[0];
            if (job.type === "update") return `Updating ${job.target}`;
            if (job.type === "scan:trivy") return `Vulnerability scan: ${job.target}`;
            if (job.type === "scan:clamav") return `Malware scan: ${job.target}`;
            return `${job.type}: ${job.target}`;
        }
        
        // Group by type for summary
        const types = new Set(jobs.map(j => j.type.split(':')[0]));
        const typeStr = Array.from(types).join(" & ");
        return `${jobs.length} ${typeStr} tasks in progress`;
    });
</script>

{#if jobs.length > 0}
    <div 
        class="w-full bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800 overflow-hidden relative group"
        transition:slide={{ duration: 300 }}
    >
        <!-- Subtle pulse background -->
        <div class="absolute inset-0 bg-brand-500/5 animate-pulse pointer-events-none"></div>

        <div class="max-w-[120rem] mx-auto px-4 md:px-8 py-3 flex flex-col gap-2 relative z-10">
            <div class="flex items-center justify-between gap-4">
                <div class="flex items-center gap-3 min-w-0">
                    <div class="w-6 h-6 rounded-lg bg-brand-100 dark:bg-brand-900/30 flex items-center justify-center text-brand-600 dark:text-brand-400 flex-shrink-0">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                    </div>
                    <div class="min-w-0 flex items-baseline gap-2">
                        <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-tight truncate">
                            {summaryLabel}
                        </p>
                        <p class="text-[9px] font-bold text-slate-400 uppercase tracking-widest whitespace-nowrap">
                            {aggregateProgress}% Total
                        </p>
                    </div>
                </div>
                
                <div class="hidden md:flex items-center gap-1">
                    {#each jobs as job}
                        <div 
                            class="w-1.5 h-1.5 rounded-full bg-brand-500/40" 
                            title={`${job.type}: ${job.target} (${job.progress}%)`}
                        ></div>
                    {/each}
                </div>
            </div>

            <!-- Aggregated Progress Bar -->
            <div class="h-1 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                <div 
                    class="h-full bg-brand-500 transition-all duration-700 ease-out rounded-full shadow-[0_0_8px_rgba(var(--color-brand-500),0.5)]"
                    style="width: {aggregateProgress}%"
                ></div>
            </div>
        </div>
    </div>
{/if}
