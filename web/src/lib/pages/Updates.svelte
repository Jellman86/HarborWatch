<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import type { UpdateJobStatus, UpdateStepEvent, UpdateStartResponse } from "../api-types";

    // Component State
    let { params } = $props<{
        params?: { 
            containerId: string; 
            targetImage: string; 
            validateUrl: string;
            validateMode?: string;
            validateTimeoutSec?: number;
            validateIntervalSec?: number;
        };
    }>();

    let updateJob = $state<UpdateJobStatus | null>(null);
    let updateLive = $state<UpdateStepEvent[]>([]);
    let updateError = $state("");
    let updateContainerId = $state("");
    let updateTargetImage = $state("");
    let validateURL = $state("http://localhost:18080/health");
    let validateMode = $state("both");
    let validateTimeout = $state(45);
    let validateInterval = $state(2);
    let bypassAi = $state(false);
    let skipHealthCheck = $state(false);

    // Pre-fill form from params when they change
    $effect(() => {
        if (params) {
            updateContainerId = params.containerId;
            updateTargetImage = params.targetImage;
            validateURL = params.validateUrl;
            if (params.validateMode) validateMode = params.validateMode;
            if (params.validateTimeoutSec) validateTimeout = params.validateTimeoutSec;
            if (params.validateIntervalSec) validateInterval = params.validateIntervalSec;
            if (typeof params.bypassAi === "boolean") bypassAi = params.bypassAi;
            if (typeof params.skipHealthCheck === "boolean") skipHealthCheck = params.skipHealthCheck;
        }
    });

    let updateEventSource: EventSource | null = null;
    let updatePollTimer: number | null = null;

    async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
        const response = await fetch(url, init);
        if (!response.ok) throw new Error(`${url} failed (${response.status})`);
        return (await response.json()) as T;
    }

    function connectUpdateEvents(jobId: string) {
        updateEventSource?.close();
        updateEventSource = new EventSource(`/api/updates/events/${jobId}`);
        updateEventSource.addEventListener("update", (evt) => {
            try {
                const parsed = JSON.parse((evt as MessageEvent).data) as UpdateStepEvent;
                updateLive = [...updateLive, parsed].slice(-100);
            } catch (e) {
                console.error("Failed to parse update event", e);
            }
        });
    }

    async function pollUpdate(jobId: string) {
        if (updatePollTimer) window.clearInterval(updatePollTimer);
        updatePollTimer = window.setInterval(async () => {
            try {
                const job = await fetchJSON<UpdateJobStatus>(`/api/updates/jobs/${jobId}`);
                if (job.aiAnalysis) {
                    job.aiAnalysis.breakingChanges = job.aiAnalysis.breakingChanges || [];
                }
                updateJob = job;
                if (job.status === "completed" || job.status === "failed" || job.status === "rolled_back") {
                    if (updatePollTimer) window.clearInterval(updatePollTimer);
                    updatePollTimer = null;
                }
            } catch (e) {
                updateError = "Failed to poll update status";
            }
        }, 1500);
    }

    async function startUpdate() {
        updateError = "";
        updateLive = [];
        try {
            const response = await fetchJSON<UpdateStartResponse>("/api/updates/run", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ 
                    containerId: updateContainerId, 
                    targetImage: updateTargetImage, 
                    validateUrl: validateURL,
                    validateMode: validateMode,
                    validateTimeoutSec: validateTimeout,
                    validateIntervalSec: validateInterval,
                    bypassAi: bypassAi,
                    skipHealthCheck: skipHealthCheck
                })
            });
            connectUpdateEvents(response.jobId);
            await pollUpdate(response.jobId);
        } catch (e) {
            updateError = e instanceof Error ? e.message : "Update pipeline failed to start";
        }
    }

    const getStepStatus = (stepId: string) => {
        const liveStep = updateLive.find(s => s.step === stepId);
        if (liveStep) return liveStep.status;
        const persistedStep = updateJob?.steps.find(s => s.step === stepId);
        return persistedStep?.status ?? 'pending';
    };

    const steps = [
        { id: 'preflight', label: 'Preflight' },
        { id: 'release_analysis', label: 'AI Analysis' },
        { id: 'backup', label: 'Backup' },
        { id: 'pull', label: 'Pull Image' },
        { id: 'recreate', label: 'Recreate' },
        { id: 'validate', label: 'Validate' },
        { id: 'cleanup', label: 'Cleanup' },
    ];

    onDestroy(() => {
        updateEventSource?.close();
        if (updatePollTimer) window.clearInterval(updatePollTimer);
    });
</script>

<div class="space-y-8">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Safe Update Pipeline</h2>
    </div>

    <div class="bg-white dark:bg-slate-800 p-8 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-6">
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div class="space-y-1">
                <label for="container-id" class="text-[10px] font-black uppercase text-slate-400 ml-1">Target Container</label>
                <input id="container-id" bind:value={updateContainerId} placeholder="container-id" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
            </div>
            <div class="space-y-1">
                <label for="target-image" class="text-[10px] font-black uppercase text-slate-400 ml-1">New Image</label>
                <input id="target-image" bind:value={updateTargetImage} placeholder="image:tag" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
            </div>
            <div class="space-y-1">
                <label for="validate-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Health URL</label>
                <input id="validate-url" bind:value={validateURL} placeholder="http://..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
            </div>
            <div class="space-y-1">
                <label for="validate-mode" class="text-[10px] font-black uppercase text-slate-400 ml-1">Validation Mode</label>
                <select id="validate-mode" bind:value={validateMode} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all">
                    <option value="both">Both (HTTP + Docker)</option>
                    <option value="http">HTTP Only</option>
                    <option value="docker">Docker Only</option>
                </select>
            </div>
            <div class="space-y-1">
                <label for="validate-timeout" class="text-[10px] font-black uppercase text-slate-400 ml-1">Timeout (sec)</label>
                <input id="validate-timeout" type="number" bind:value={validateTimeout} min="1" max="600" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
            </div>
            <div class="space-y-1">
                <label for="validate-interval" class="text-[10px] font-black uppercase text-slate-400 ml-1">Initial Interval (sec)</label>
                <input id="validate-interval" type="number" bind:value={validateInterval} min="1" max="30" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
            </div>
        </div>

        <div class="flex justify-end pt-2">
            <button 
                onclick={startUpdate}
                disabled={!!updateJob && (updateJob.status === 'running')}
                class="px-10 py-3 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-xl font-black uppercase tracking-widest text-xs transition-all shadow-xl shadow-brand-500/30"
            >
                Execute Pipeline
            </button>
        </div>
    </div>

    {#if updateError}
        <div class="p-4 bg-rose-50 border border-rose-100 text-rose-700 rounded-xl text-sm font-bold flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
            </svg>
            {updateError}
        </div>
    {/if}

    {#if updateJob || updateLive.length > 0}
        <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden">
            <div class="p-6 border-b border-slate-50 dark:border-slate-700 bg-slate-50/50 dark:bg-slate-900/50 flex justify-between items-center">
                <div>
                    <h3 class="font-bold text-slate-900 dark:text-white uppercase tracking-wider text-sm">Pipeline Execution Status</h3>
                    <p class="text-xs text-slate-500 mt-1">JOB ID: {updateJob?.jobId ?? 'Initializing...'}</p>
                </div>
                <span class="px-3 py-1 rounded-full text-[10px] font-black uppercase tracking-tighter {updateJob?.status === 'completed' ? 'bg-emerald-100 text-emerald-700' : updateJob?.status === 'failed' || updateJob?.status === 'rolled_back' ? 'bg-rose-100 text-rose-700' : 'bg-brand-100 text-brand-700 animate-pulse'}">
                    {updateJob?.status ?? 'Running'}
                </span>
            </div>

            <!-- AI Analysis Result -->
            {#if updateJob?.aiAnalysis}
                <div class="mx-6 mb-6 p-5 bg-brand-50 dark:bg-brand-900/10 border border-brand-100 dark:border-brand-900/30 rounded-2xl">
                    <div class="flex items-center justify-between mb-3">
                        <div class="flex items-center gap-2">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                            </svg>
                            <span class="font-bold text-slate-900 dark:text-slate-100 text-sm uppercase tracking-wider">AI Intelligence Report</span>
                        </div>
                        <span class="px-2 py-1 rounded text-[10px] font-black uppercase {updateJob.aiAnalysis.riskLevel === 'Low' ? 'bg-emerald-100 text-emerald-700' : 'bg-rose-100 text-rose-700'}">
                            {updateJob.aiAnalysis.riskLevel} Risk
                        </span>
                    </div>
                    <p class="text-sm text-slate-600 dark:text-slate-300 leading-relaxed mb-4 italic">
                        "{updateJob.aiAnalysis.summary}"
                    </p>
                    {#if updateJob.aiAnalysis.breakingChanges.length > 0}
                        <div class="space-y-2">
                            <span class="text-[9px] font-black text-rose-600 dark:text-rose-400 uppercase tracking-widest">Potential Breaking Changes:</span>
                            <ul class="list-disc list-inside text-xs text-slate-500 dark:text-slate-400">
                                {#each updateJob.aiAnalysis.breakingChanges as change}
                                    <li>{change}</li>
                                {/each}
                            </ul>
                        </div>
                    {/if}
                </div>
            {/if}

            <!-- Pipeline Stepper -->
            <div class="p-10">
                <div class="flex items-center">
                    {#each steps as step, i}
                        <div class="flex-1 relative">
                            {#if i !== 0}
                                <div class="absolute left-[-50%] right-[50%] top-5 h-0.5 {getStepStatus(step.id) !== 'pending' ? 'bg-brand-500' : 'bg-slate-200 dark:bg-slate-700'}"></div>
                            {/if}
                            <div class="relative flex flex-col items-center group">
                                <div class="w-10 h-10 rounded-full flex items-center justify-center border-2 z-10 transition-all duration-500 {getStepStatus(step.id) === 'completed' ? 'bg-brand-600 border-brand-600 text-white' : getStepStatus(step.id) === 'failed' ? 'bg-rose-600 border-rose-600 text-white' : getStepStatus(step.id) === 'running' ? 'bg-white dark:bg-slate-800 border-brand-500 ring-4 ring-brand-500/20' : 'bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-400'}">
                                    {#if getStepStatus(step.id) === 'completed'}
                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" viewBox="0 0 20 20" fill="currentColor">
                                            <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                                        </svg>
                                    {:else if getStepStatus(step.id) === 'failed'}
                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" viewBox="0 0 20 20" fill="currentColor">
                                            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
                                        </svg>
                                    {:else if getStepStatus(step.id) === 'running'}
                                        <div class="w-2 h-2 rounded-full bg-brand-600 animate-ping"></div>
                                    {:else}
                                        <span class="text-xs font-bold">{i + 1}</span>
                                    {/if}
                                </div>
                                <span class="mt-3 text-[10px] font-black uppercase tracking-widest {getStepStatus(step.id) !== 'pending' ? 'text-brand-600 dark:text-brand-400' : 'text-slate-400'}">
                                    {step.label}
                                </span>
                            </div>
                        </div>
                    {/each}
                </div>
            </div>

            <!-- Terminal log -->
            <div class="bg-slate-900 m-6 rounded-xl p-4 font-mono text-[10px] text-slate-300 h-[200px] overflow-y-auto custom-scrollbar">
                {#each updateLive as e}
                    <div class="flex gap-2 mb-1">
                        <span class="text-slate-600">[{new Date(e.timestamp * 1000).toLocaleTimeString()}]</span>
                        <span class="font-bold text-brand-400 uppercase w-16">{e.step}</span>
                        <span class={e.status === 'failed' ? 'text-rose-400' : 'text-slate-300'}>{e.message}</span>
                    </div>
                {:else}
                    <div class="text-slate-600 italic">No events streaming...</div>
                {/each}
            </div>
        </div>
    {/if}
</div>

<style>
    .custom-scrollbar::-webkit-scrollbar { width: 4px; }
    .custom-scrollbar::-webkit-scrollbar-track { @apply bg-transparent; }
    .custom-scrollbar::-webkit-scrollbar-thumb { @apply bg-slate-700 rounded-full; }
</style>
