<script lang="ts">
    import { onMount, onDestroy } from "svelte";

    interface SystemStatus {
        uptime: number;
        version: string;
        memoryAlloc: number;
        numGoroutine: number;
        dbSize: number;
    }

    interface LogEntry {
        id: number;
        timestamp: number;
        level: string;
        message: string;
        source: string;
    }

    let status = $state<SystemStatus | null>(null);
    let logs = $state<LogEntry[]>([]);
    let pollTimer: number | null = null;

    async function loadDiagnostics() {
        try {
            const [statusRes, logsRes] = await Promise.all([
                fetch("/api/system/status"),
                fetch("/api/system/logs")
            ]);
            
            if (statusRes.ok) status = await statusRes.json();
            if (logsRes.ok) logs = await logsRes.json() || [];
        } catch (e) {
            console.error("Failed to load diagnostics", e);
        }
    }

    const formatBytes = (bytes: number) => {
        if (bytes === 0) return "0 B";
        const k = 1024;
        const sizes = ["B", "KB", "MB", "GB"];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
    };

    const formatUptime = (seconds: number) => {
        const d = Math.floor(seconds / (3600*24));
        const h = Math.floor(seconds % (3600*24) / 3600);
        const m = Math.floor(seconds % 3600 / 60);
        return `${d}d ${h}h ${m}m`;
    };

    onMount(() => {
        loadDiagnostics();
        pollTimer = window.setInterval(loadDiagnostics, 5000);
    });

    onDestroy(() => {
        if (pollTimer) clearInterval(pollTimer);
    });
</script>

<div class="space-y-8">
    <div>
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Appliance Health</h2>
        <p class="text-sm text-slate-500 mt-1">Real-time telemetry and internal application logs.</p>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
            <span class="text-[10px] font-black uppercase text-slate-400 tracking-widest">Uptime</span>
            <div class="mt-2 text-2xl font-black text-slate-900 dark:text-white">
                {status ? formatUptime(status.uptime) : '---'}
            </div>
        </div>
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
            <span class="text-[10px] font-black uppercase text-slate-400 tracking-widest">Memory (Alloc)</span>
            <div class="mt-2 text-2xl font-black text-brand-600 dark:text-brand-400">
                {status ? formatBytes(status.memoryAlloc) : '---'}
            </div>
        </div>
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
            <span class="text-[10px] font-black uppercase text-slate-400 tracking-widest">Database Size</span>
            <div class="mt-2 text-2xl font-black text-slate-900 dark:text-white">
                {status ? formatBytes(status.dbSize) : '---'}
            </div>
        </div>
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
            <span class="text-[10px] font-black uppercase text-slate-400 tracking-widest">Active Routines</span>
            <div class="mt-2 text-2xl font-black text-slate-900 dark:text-white">
                {status ? status.numGoroutine : '---'}
            </div>
        </div>
    </div>

    <!-- Internal Logs -->
    <div class="bg-slate-950 rounded-2xl border border-slate-800 shadow-2xl overflow-hidden flex flex-col h-[600px]">
        <div class="p-4 border-b border-slate-800 flex items-center justify-between bg-slate-900/50">
            <h3 class="text-xs font-black uppercase text-slate-400 tracking-widest flex items-center gap-2">
                <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
                System Log Stream
            </h3>
            <span class="text-[10px] font-mono text-slate-600">v{status?.version}</span>
        </div>
        <div class="flex-1 overflow-y-auto p-6 font-mono text-[11px] space-y-1 custom-scrollbar">
            {#each logs as entry}
                <div class="flex gap-4">
                    <span class="text-slate-600 shrink-0 w-24">[{new Date(entry.timestamp * 1000).toLocaleTimeString()}]</span>
                    <span class="font-black w-12 shrink-0 {entry.level === 'ERROR' ? 'text-rose-500' : entry.level === 'WARN' ? 'text-amber-500' : 'text-brand-400'}">{entry.level}</span>
                    <span class="text-slate-500 shrink-0 w-20">[{entry.source}]</span>
                    <span class="text-slate-300 break-all">{entry.message}</span>
                </div>
            {:else}
                <div class="text-slate-700 italic">No logs available...</div>
            {/each}
        </div>
    </div>
</div>

<style>
    .custom-scrollbar::-webkit-scrollbar { width: 4px; }
    .custom-scrollbar::-webkit-scrollbar-track { @apply bg-transparent; }
    .custom-scrollbar::-webkit-scrollbar-thumb { @apply bg-slate-800 rounded-full; }
</style>
