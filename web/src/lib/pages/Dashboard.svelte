<script lang="ts">
    import type { HealthResponse, ScanSummary, MalwareScanSummary, ReleaseRiskSummary, ContainerSummary, ImageSummary, DockerEvent } from "../api-types";

    let { health, summary, malwareSummaries, releaseSummary, containers, images, events, onRefresh } = $props<{
        health: HealthResponse | null;
        summary: ScanSummary | null;
        malwareSummaries: MalwareScanSummary[];
        releaseSummary: ReleaseRiskSummary | null;
        containers: ContainerSummary[];
        images: ImageSummary[];
        events: DockerEvent[];
        onRefresh: () => void;
    }>();

    const formatId = (id: string) => (id.length > 12 ? id.slice(0, 12) : id);
    const riskBand = (score: number) => score >= 80 ? "Critical" : score >= 60 ? "High" : score >= 30 ? "Medium" : score > 0 ? "Low" : "None";
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">System Overview</h2>
        <button class="px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-lg font-semibold transition-colors" onclick={onRefresh}>
            Refresh Data
        </button>
    </div>

    <!-- Stats Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
            <span class="text-sm font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">Containers</span>
            <div class="mt-2 flex items-baseline gap-2">
                <span class="text-3xl font-bold text-slate-900 dark:text-white">{containers.length}</span>
                <span class="text-xs font-semibold text-emerald-600 dark:text-emerald-400">Online</span>
            </div>
        </div>
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
            <span class="text-sm font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">Images</span>
            <div class="mt-2 flex items-baseline gap-2">
                <span class="text-3xl font-bold text-slate-900 dark:text-white">{images.length}</span>
                <span class="text-xs font-semibold text-slate-500 uppercase">Stored</span>
            </div>
        </div>
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
            <span class="text-sm font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">Risk Score</span>
            <div class="mt-2 flex items-baseline gap-2">
                <span class="text-3xl font-bold {summary && summary.riskScore > 50 ? 'text-rose-600' : 'text-emerald-600'}">
                    {summary ? summary.riskScore : '0'}
                </span>
                <span class="text-xs font-semibold text-slate-500 uppercase">{summary ? riskBand(summary.riskScore) : 'N/A'}</span>
            </div>
        </div>
        <div class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm">
            <span class="text-sm font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">System Health</span>
            <div class="mt-2 flex items-baseline gap-2">
                <span class="text-xl font-bold text-slate-900 dark:text-white uppercase">{health?.status ?? 'Unknown'}</span>
                <span class="text-xs font-medium text-slate-500">{health?.version}</span>
            </div>
        </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Live Events -->
        <div class="lg:col-span-2 bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden flex flex-col h-[400px]">
            <div class="p-4 border-b border-slate-100 dark:border-slate-700 flex items-center justify-between">
                <h3 class="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
                    Live Docker Stream
                </h3>
            </div>
            <div class="flex-1 overflow-y-auto p-4 space-y-2 font-mono text-xs">
                {#each events as e}
                    <div class="flex gap-3 text-slate-600 dark:text-slate-400 border-b border-slate-50 dark:border-slate-700/50 pb-2">
                        <span class="text-slate-400">{new Date(e.time * 1000).toLocaleTimeString()}</span>
                        <span class="font-bold text-brand-600 dark:text-brand-400 uppercase w-16">{e.action}</span>
                        <span class="text-slate-900 dark:text-slate-200 truncate">{formatId(e.id)}</span>
                        <span class="italic truncate opacity-60">({e.from})</span>
                    </div>
                {:else}
                    <div class="flex items-center justify-center h-full text-slate-400">Waiting for events...</div>
                {/each}
            </div>
        </div>

        <!-- Vulnerability Quick View -->
        <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm p-6">
            <h3 class="font-bold text-slate-900 dark:text-white mb-4">Security Hotspots</h3>
            {#if summary}
                <div class="space-y-4">
                    <div class="p-4 bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 rounded-xl">
                        <div class="text-xs font-bold text-rose-600 dark:text-rose-400 uppercase mb-1">Critical Vulnerabilities</div>
                        <div class="text-2xl font-black text-rose-700 dark:text-rose-300">{summary.critical}</div>
                    </div>
                    <div class="p-4 bg-orange-50 dark:bg-orange-900/10 border border-orange-100 dark:border-orange-900/30 rounded-xl">
                        <div class="text-xs font-bold text-orange-600 dark:text-orange-400 uppercase mb-1">High Severity</div>
                        <div class="text-2xl font-black text-orange-700 dark:text-orange-300">{summary.high}</div>
                    </div>
                    <div class="mt-4 text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
                        Latest scan for <code class="bg-slate-100 dark:bg-slate-700 px-1 rounded">{summary.target}</code> completed on {new Date(summary.scannedAt * 1000).toLocaleDateString()}.
                    </div>
                </div>
            {:else}
                <div class="flex items-center justify-center h-48 text-slate-400">No scan data available</div>
            {/if}
        </div>
    </div>
</div>
