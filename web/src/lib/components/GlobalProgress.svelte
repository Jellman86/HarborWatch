<script lang="ts">
    import type { JobProgress } from "../api-types";
    import { fly, slide } from "svelte/transition";

    let { jobs = [] } = $props<{
        jobs: JobProgress[];
    }>();

    const jobIcon = (type: string) => {
        if (type.startsWith("scan:")) return "shield-search";
        if (type === "update") return "refresh";
        return "activity";
    };

    const jobLabel = (job: JobProgress) => {
        if (job.type === "update") return `Updating ${job.target}`;
        if (job.type === "scan:trivy") return `Vulnerability scan: ${job.target}`;
        if (job.type === "scan:clamav") return `Malware scan: ${job.target}`;
        return `${job.type}: ${job.target}`;
    };
</script>

{#if jobs.length > 0}
    <div 
        class="fixed top-0 left-0 right-0 z-[60] flex flex-col items-center pointer-events-none"
        transition:slide
    >
        <div class="w-full max-w-2xl mt-4 px-4 space-y-2 pointer-events-auto">
            {#each jobs as job (job.id)}
                <div 
                    class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl shadow-brand-500/10 p-3 flex flex-col gap-2 overflow-hidden relative group"
                    in:fly={{ y: -20, duration: 300 }}
                    out:fly={{ y: -20, duration: 200 }}
                >
                    <!-- Background subtle pulse for running jobs -->
                    {#if job.status === 'running'}
                        <div class="absolute inset-0 bg-brand-500/5 animate-pulse pointer-events-none"></div>
                    {/if}

                    <div class="flex items-center justify-between gap-4 relative z-10">
                        <div class="flex items-center gap-3 min-w-0">
                            <div class="w-8 h-8 rounded-xl bg-brand-100 dark:bg-brand-900/30 flex items-center justify-center text-brand-600 dark:text-brand-400 flex-shrink-0">
                                <span class="text-xs font-black uppercase">{job.type.split(':')[0]}</span>
                            </div>
                            <div class="min-w-0">
                                <p class="text-[11px] font-black text-slate-900 dark:text-white uppercase tracking-tight truncate">
                                    {jobLabel(job)}
                                </p>
                                <p class="text-[9px] font-bold text-slate-400 uppercase tracking-widest">
                                    {job.status} • {job.progress}%
                                </p>
                            </div>
                        </div>
                    </div>

                    <!-- Progress Bar Container -->
                    <div class="h-1.5 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden relative z-10">
                        <div 
                            class="h-full bg-brand-500 transition-all duration-500 ease-out rounded-full"
                            style="width: {job.progress}%"
                        ></div>
                    </div>
                </div>
            {/each}
        </div>
    </div>
{/if}

<style>
    /* Prevent interference with sidebar on mobile */
    @media (max-width: 768px) {
        .fixed {
            padding-left: 0;
        }
    }
</style>
