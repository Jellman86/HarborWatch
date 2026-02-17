<script lang="ts">
    import { onMount } from "svelte";

    interface SystemStatus {
        uptime: number;
        numGoroutine: number;
        memoryAlloc: number;
        dbSize: number;
    }

    interface LogEntry {
        timestamp: number;
        level: string;
        source: string;
        message: string;
    }

    let status = $state<SystemStatus | null>(null);
    let logs = $state<LogEntry[]>([]);
    let loading = $state(true);

    async function loadData() {
        try {
            const [statusRes, logsRes] = await Promise.all([
                fetch("/api/system/status"),
                fetch("/api/system/logs?limit=50")
            ]);
            if (statusRes.ok) status = await statusRes.json();
            if (logsRes.ok) logs = await logsRes.json();
        } catch (e) {
            console.error("Failed to load diagnostics", e);
        } finally {
            loading = false;
        }
    }

    onMount(() => {
        loadData();
        const interval = setInterval(loadData, 5000);
        return () => clearInterval(interval);
    });

    const formatBytes = (bytes: number) => (bytes / (1024 * 1024)).toFixed(2) + " MB";
    const formatUptime = (seconds: number) => {
        const h = Math.floor(seconds / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        return `${h}h ${m}m`;
    };

    const levelColor = (level: string) => {
        switch(level.toLowerCase()) {
            case 'error': return 'text-rose-500';
            case 'warn': return 'text-amber-500';
            default: return 'text-slate-400';
        }
    };
</script>

<div class="space-y-8">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">System Health</h2>
            <p class="text-xs text-slate-500 font-medium">Real-time appliance telemetry and internal execution logs.</p>
        </div>
        <div class="flex items-center gap-2">
            <div class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></div>
            <span class="text-[10px] font-black uppercase tracking-widest text-slate-400">Live Telemetry</span>
        </div>
    </div>

    {#if status}
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 opacity-0 animate-reveal stagger-1">
            <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl">
                <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-1">Uptime</p>
                <p class="text-2xl font-black text-slate-900 dark:text-white tracking-tight">{formatUptime(status.uptime)}</p>
            </div>
            <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl">
                <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-1">Memory (Alloc)</p>
                <p class="text-2xl font-black text-slate-900 dark:text-white tracking-tight">{formatBytes(status.memoryAlloc)}</p>
            </div>
            <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl">
                <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-1">Database Size</p>
                <p class="text-2xl font-black text-slate-900 dark:text-white tracking-tight">{formatBytes(status.dbSize)}</p>
            </div>
            <div class="bg-white dark:bg-slate-800 p-6 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl">
                <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-1">Active Routines</p>
                <p class="text-2xl font-black text-slate-900 dark:text-white tracking-tight">{status.numGoroutine}</p>
            </div>
        </div>
    {/if}

    <div class="space-y-4 opacity-0 animate-reveal stagger-2">
        <h3 class="text-xs font-black uppercase text-slate-400 tracking-widest flex items-center gap-2">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Internal Application Logs
        </h3>
        
        <div class="bg-slate-900 rounded-3xl border border-slate-800 shadow-2xl overflow-hidden">
            <div class="overflow-x-auto">
                <table class="w-full text-left border-collapse">
                    <thead>
                        <tr class="bg-slate-950/50 text-[10px] font-black uppercase tracking-widest text-slate-500 border-b border-slate-800">
                            <th class="px-6 py-3 w-48">Timestamp</th>
                            <th class="px-6 py-3 w-24">Level</th>
                            <th class="px-6 py-3 w-32">Source</th>
                            <th class="px-6 py-3">Message</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-800/50 font-mono text-[11px]">
                        {#each logs as log}
                            <tr class="hover:bg-slate-800/30 transition-colors">
                                <td class="px-6 py-2 text-slate-500">
                                    {new Date(log.timestamp * 1000).toISOString().replace('T', ' ').split('.')[0]}
                                </td>
                                <td class="px-6 py-2 font-black uppercase {levelColor(log.level)}">
                                    {log.level}
                                </td>
                                <td class="px-6 py-2 text-brand-500 font-bold">
                                    {log.source}
                                </td>
                                <td class="px-6 py-2 text-slate-300">
                                    {log.message}
                                </td>
                            </tr>
                        {:else}
                            <tr>
                                <td colspan="4" class="px-6 py-12 text-center text-slate-600 italic">No internal logs captured in this window.</td>
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</div>
