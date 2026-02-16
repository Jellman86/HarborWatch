<script lang="ts">
    import { onMount } from "svelte";
    import type { ScheduleEntry } from "../api-types";

    // Since ScheduleEntry is newly added to the backend, 
    // I will use a local type for safety until the next type generation run
    interface Schedule {
        id: string;
        cronSpec: string;
        enabled: boolean;
        lastRun?: number;
    }

    let schedules = $state<Schedule[]>([]);
    let loading = $state(false);
    let error = $state("");

    async function loadSchedules() {
        loading = true;
        try {
            const res = await fetch("/api/scheduler/schedules");
            if (res.ok) schedules = await res.json();
        } catch (e) {
            error = "Failed to load automation schedules";
        } finally {
            loading = false;
        }
    }

    onMount(loadSchedules);

    const formatTime = (ts?: number) => ts && ts > 0 ? new Date(ts * 1000).toLocaleString() : "Never";
    
    // Human readable cron mapping
    const cronLabel = (spec: string) => {
        if (spec === "0 0 3 * * 0") return "Weekly (Sun 3 AM)";
        if (spec.startsWith("0 0")) return "Daily at Midnight";
        return spec;
    };

    const taskLabel = (id: string) => {
        switch(id) {
            case 'docker_system_prune': return "Docker System Garbage Collection";
            case 'security_sweep_trivy': return "Nightly Vulnerability Scan";
            case 'malware_sweep_clamav': return "Weekly Malware Sweep";
            default: return id.replace(/_/g, ' ').toUpperCase();
        }
    };
</script>

<div class="space-y-6">
    <div>
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Automation & Maintenance</h2>
        <p class="text-sm text-slate-500 mt-1">Configure automated background tasks to keep your system clean and secure.</p>
    </div>

    {#if error}
        <div class="p-4 bg-rose-50 border border-rose-100 text-rose-700 rounded-xl text-sm font-medium">{error}</div>
    {/if}

    <div class="grid grid-cols-1 gap-4">
        {#each schedules as s}
            <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm flex items-center justify-between group hover:border-brand-500/50 transition-all">
                <div class="flex items-center gap-5">
                    <div class="w-12 h-12 rounded-xl bg-slate-50 dark:bg-slate-900 flex items-center justify-center text-slate-400 group-hover:text-brand-600 transition-colors">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                    </div>
                    <div>
                        <h3 class="font-bold text-slate-900 dark:text-white">{taskLabel(s.id)}</h3>
                        <div class="flex items-center gap-3 mt-1">
                            <span class="text-[10px] font-black uppercase text-brand-600 dark:text-brand-400 tracking-widest">{cronLabel(s.cronSpec)}</span>
                            <span class="w-1 h-1 rounded-full bg-slate-300"></span>
                            <span class="text-[10px] font-medium text-slate-400">Last run: {formatTime(s.lastRun)}</span>
                        </div>
                    </div>
                </div>

                <div class="flex items-center gap-4">
                    <div class="flex flex-col items-end">
                        <span class="text-[9px] font-bold uppercase tracking-tighter mb-1 {s.enabled ? 'text-emerald-600' : 'text-slate-400'}">
                            {s.enabled ? 'Active' : 'Paused'}
                        </span>
                        <button 
                            class="w-10 h-5 rounded-full relative transition-colors {s.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                        >
                            <div class="absolute top-1 w-3 h-3 bg-white rounded-full transition-all {s.enabled ? 'right-1' : 'left-1'}"></div>
                        </button>
                    </div>
                    
                    <button class="px-4 py-2 bg-slate-50 dark:bg-slate-900 hover:bg-brand-600 hover:text-white dark:hover:bg-brand-600 rounded-lg text-xs font-bold text-slate-600 dark:text-slate-400 transition-all border border-slate-100 dark:border-slate-700 uppercase tracking-widest">
                        Run Now
                    </button>
                </div>
            </div>
        {:else}
            <div class="py-20 text-center text-slate-400 italic bg-slate-50 dark:bg-slate-900/50 rounded-2xl border-2 border-dashed border-slate-200 dark:border-slate-800">
                {loading ? 'Discovering automation tasks...' : 'No background tasks registered'}
            </div>
        {/each}
    </div>

    <div class="mt-8 p-6 bg-brand-50 dark:bg-brand-900/10 border border-brand-100 dark:border-brand-900/30 rounded-2xl flex gap-4">
        <div class="p-2 bg-white dark:bg-slate-800 rounded-lg shadow-sm self-start">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
        </div>
        <div>
            <h4 class="text-sm font-bold text-brand-900 dark:text-brand-300">Pro-Tip: Label-Driven Automation</h4>
            <p class="text-xs text-brand-700 dark:text-brand-400 mt-1 leading-relaxed">
                Add <code>harborwatch.enable=true</code> to any container to include it in the global automation scope. 
                Use <code>harborwatch.update.policy=ai-only</code> for smart, risk-aware updates.
            </p>
        </div>
    </div>
</div>
