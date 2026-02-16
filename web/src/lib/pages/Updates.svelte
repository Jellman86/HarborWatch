<script lang="ts">
    import type { UpdateJobStatus, UpdateStepEvent } from "../api-types";

    let { 
        updateJob, 
        updateLive, 
        updateError, 
        updateContainerId = $bindable(), 
        updateTargetImage = $bindable(), 
        validateURL = $bindable(), 
        onStartUpdate 
    } = $props<{
        updateJob: UpdateJobStatus | null;
        updateLive: UpdateStepEvent[];
        updateError: string;
        updateContainerId: string;
        updateTargetImage: string;
        validateURL: string;
        onStartUpdate: () => void;
    }>();

    const getStepStatus = (stepName: string) => {
        const liveStep = updateLive.find(s => s.step === stepName);
        if (liveStep) return liveStep.status;
        const persistedStep = updateJob?.steps.find(s => s.step === stepName);
        return persistedStep?.status ?? 'pending';
    };

    const steps = [
        { id: 'preflight', label: 'Preflight' },
        { id: 'backup', label: 'Backup' },
        { id: 'pull', label: 'Pull Image' },
        { id: 'recreate', label: 'Recreate' },
        { id: 'validate', label: 'Validate' },
    ];
</script>

<div class="space-y-8">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Safe Update Pipeline</h2>
    </div>

    <div class="bg-white dark:bg-slate-800 p-8 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-6">
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div class="space-y-1">
                <label class="text-[10px] font-black uppercase text-slate-400 ml-1">Target Container</label>
                <input bind:value={updateContainerId} placeholder="container-id" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
            </div>
            <div class="space-y-1">
                <label class="text-[10px] font-black uppercase text-slate-400 ml-1">New Image</label>
                <input bind:value={updateTargetImage} placeholder="image:tag" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
            </div>
            <div class="space-y-1">
                <label class="text-[10px] font-black uppercase text-slate-400 ml-1">Health URL</label>
                <input bind:value={validateURL} placeholder="http://..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
            </div>
        </div>

        <div class="flex justify-end pt-2">
            <button 
                onclick={onStartUpdate}
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
