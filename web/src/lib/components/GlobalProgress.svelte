<script lang="ts">
    import type { JobProgress } from "../api-types";
    import { slide } from "svelte/transition";

    let { jobs = [] } = $props<{
        jobs: JobProgress[];
    }>();

    // Calculate aggregated progress (average of all active jobs)
    // Show progress of the ENTIRE set (queued jobs count as 0%)
    let aggregateProgress = $derived.by(() => {
        if (jobs.length === 0) return 0;
        // Map indeterminate (-1) or queued to 0 for the purpose of the bar
        const total = jobs.reduce((acc: number, j: JobProgress) => acc + Math.max(0, j.progress), 0);
        return Math.round(total / jobs.length);
    });

    let queuedCount = $derived(jobs.filter((j: JobProgress) => j.status === 'queued').length);
    let activeCount = $derived(jobs.filter((j: JobProgress) => j.status === 'running').length);

    const formatTarget = (t: string) => t.replace(/^\//, '').replace(/^container:/, '').replace(/^image:/, '');

    const jobVerb = (type: string) => {
        if (type === "update") return "Updating";
        if (type === "gitops_deploy") return "Deploying Stack";
        if (type.startsWith("scan:")) {
            if (type === "scan:trivy") return "Scanning (Vulnerability)";
            if (type === "scan:clamav") return "Scanning (Malware)";
            return "Scanning";
        }
        if (type === "redeploy") return "Redeploying";
        return "Processing";
    };

    const jobTag = (type: string) => {
        if (type === "gitops_deploy") return { label: "GitOps Deploy", tone: "emerald" };
        if (type === "scan:trivy") return { label: "Vulnerability (Trivy)", tone: "amber" };
        if (type === "scan:clamav") return { label: "AV (ClamAV)", tone: "rose" };
        if (type === "update") return { label: "Upgrade", tone: "brand" };
        if (type === "redeploy") return { label: "Redeploy", tone: "sky" };
        if (type.startsWith("remediation")) return { label: "Self-Heal", tone: "fuchsia" };
        return { label: "Task", tone: "slate" };
    };

    const jobTagClass = (type: string) => {
        const tone = jobTag(type).tone;
        if (tone === "emerald") return "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300";
        if (tone === "amber") return "bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300";
        if (tone === "rose") return "bg-rose-100 text-rose-800 dark:bg-rose-900/30 dark:text-rose-300";
        if (tone === "brand") return "bg-brand-100 text-brand-800 dark:bg-brand-900/30 dark:text-brand-300";
        if (tone === "sky") return "bg-sky-100 text-sky-800 dark:bg-sky-900/30 dark:text-sky-300";
        if (tone === "fuchsia") return "bg-fuchsia-100 text-fuchsia-800 dark:bg-fuchsia-900/30 dark:text-fuchsia-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    };

    let summaryLabel = $derived.by(() => {
        if (jobs.length === 0) return "";
        if (jobs.length === 1) {
            return `${jobVerb(jobs[0].type)} ${formatTarget(jobs[0].target)}`;
        }
        
        const types = [...new Set(jobs.map((j: JobProgress) => j.type))];
        const targets = jobs.map((j: JobProgress) => formatTarget(j.target));
        
        if (types.length === 1) {
            const verb = jobVerb(types[0] as string);
            if (targets.length === 2) return `${verb} ${targets[0]} & ${targets[1]}`;
            return `${verb} ${targets[0]} and ${targets.length - 1} others`;
        }

        return `${jobs.length} background tasks in progress`;
    });

    let currentMessage = $derived.by(() => {
        if (jobs.length === 0) return "";
        
        // Prefer showing the most 'interesting' message (e.g. not just 'queued')
        const active = jobs.find((j: JobProgress) => j.status === 'running' && j.message);
        if (active) return active.message;
        
        const last = jobs[jobs.length - 1];
        return last.message || last.status;
    });

    let hasEstimatedProgress = $derived.by(() => jobs.some((j: JobProgress) => j.progressMode === "estimated"));

    let showDetails = $state(false);
    let cancellingAll = $state(false);

    async function cancelAllJobs() {
        if (cancellingAll) return;
        if (!confirm("Are you sure you want to cancel all active background tasks?")) return;
        
        cancellingAll = true;
        try {
            const res = await fetch("/api/system/jobs/cancel-all", { method: "POST" });
            if (res.ok) {
                // Background polling will clear the UI
            }
        } catch (e) {
            console.error("Failed to cancel jobs", e);
        } finally {
            cancellingAll = false;
        }
    }
</script>

{#if jobs.length > 0}
    <div 
        class="sticky top-[var(--global-progress-sticky-top)] z-30 w-full bg-white/95 dark:bg-slate-900/95 backdrop-blur-md border-b border-slate-200 dark:border-slate-800 overflow-hidden relative group"
        transition:slide={{ duration: 300 }}
        role="status"
        aria-live="polite"
    >
        <!-- Subtle pulse background -->
        <div class="absolute inset-0 bg-brand-500/5 animate-pulse pointer-events-none"></div>

        <div class="max-w-[120rem] mx-auto px-4 md:px-8 py-3 relative z-10">
            <div class="flex flex-col gap-2">
                <div class="flex items-center justify-between gap-4">
                    <div 
                        class="flex items-center gap-3 min-w-0 flex-1 cursor-help focus:outline-none focus:ring-2 focus:ring-brand-500 rounded-lg" 
                        onmouseenter={() => showDetails = true} 
                        onmouseleave={() => showDetails = false}
                        onclick={() => showDetails = !showDetails}
                        role="button"
                        tabindex="0"
                        onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') showDetails = !showDetails }}
                    >
                        <div class="w-6 h-6 rounded-lg bg-brand-100 dark:bg-brand-900/30 flex items-center justify-center text-brand-600 dark:text-brand-400 flex-shrink-0">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                            </svg>
                        </div>
                        <div class="min-w-0 flex flex-col md:flex-row md:items-baseline md:gap-3 cursor-help">
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
                    
                    <div class="flex items-center gap-4">
                        <div class="flex items-center gap-2">
                            {#if queuedCount > 0}
                                <span class="px-2 py-0.5 bg-brand-100 dark:bg-brand-900/40 text-brand-700 dark:text-brand-300 text-[8px] font-black uppercase rounded-md animate-pulse">
                                    {queuedCount} in Queue
                                </span>
                            {/if}
                            <p class="text-[9px] font-bold text-slate-400 uppercase tracking-widest whitespace-nowrap">
                                {aggregateProgress}% Total
                            </p>
                            {#if hasEstimatedProgress}
                                <p class="text-[8px] font-bold text-amber-600 dark:text-amber-400 uppercase tracking-widest whitespace-nowrap">
                                    Estimated
                                </p>
                            {/if}
                        </div>

                        <button 
                            onclick={cancelAllJobs}
                            disabled={cancellingAll}
                            class="px-2 py-1 bg-rose-50 hover:bg-rose-100 dark:bg-rose-900/20 dark:hover:bg-rose-900/40 text-rose-600 dark:text-rose-400 text-[9px] font-black uppercase tracking-widest rounded transition-all border border-rose-200 dark:border-rose-800 disabled:opacity-50"
                            title="Cancel all background tasks"
                        >
                            {cancellingAll ? '...' : 'Cancel All'}
                        </button>
                    </div>
                </div>

                <!-- Aggregated Progress Bar -->
                <div class="h-1.5 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden relative">
                    <div class="absolute inset-0 opacity-60 bg-[linear-gradient(90deg,transparent_0,transparent_6px,rgba(255,255,255,0.45)_6px,rgba(255,255,255,0.45)_8px)] dark:bg-[linear-gradient(90deg,transparent_0,transparent_6px,rgba(255,255,255,0.08)_6px,rgba(255,255,255,0.08)_8px)] bg-[length:12px_100%]"></div>
                    <div 
                        class="h-full bg-gradient-to-r from-brand-500 via-brand-400 to-emerald-400 transition-all duration-700 ease-out rounded-full shadow-[0_0_10px_rgba(var(--color-brand-500),0.35)] relative"
                        style="width: {aggregateProgress}%"
                    >
                        <div class="absolute inset-0 bg-white/20"></div>
                    </div>
                </div>

                <!-- Detailed View (Expanded on Hover or if multiple) -->
                {#if (showDetails && jobs.length > 1) || (showDetails && jobs[0]?.message)}
                    <div class="pt-2 border-t border-slate-100 dark:border-slate-800/50 mt-1 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-2" in:slide>
                        {#each jobs as job (job.id)}
                            <div class="flex items-center justify-between gap-3 text-[9px] min-w-0">
                                <div class="flex items-center gap-2 min-w-0 flex-1">
                                    <span class="px-1.5 py-0.5 rounded-md font-black uppercase tracking-wide whitespace-nowrap {jobTagClass(job.type)}">
                                        {jobTag(job.type).label}
                                    </span>
                                    <span class="text-slate-500 font-bold uppercase truncate min-w-0">
                                        {formatTarget(job.target)}
                                    </span>
                                </div>
                                <span class="text-slate-400 truncate flex-1 text-right">
                                    {job.message || job.status}
                                </span>
                                <span class="font-black text-brand-600 dark:text-brand-400 w-8 text-right">
                                    {job.progress >= 0 ? job.progress + '%' : '...'}
                                </span>
                            </div>
                        {/each}
                    </div>
                {/if}
            </div>
        </div>
    </div>
{/if}
