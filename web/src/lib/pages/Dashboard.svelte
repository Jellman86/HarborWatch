<script lang="ts">
    import { onMount } from "svelte";
    import type { ContainerSummary, ScanSummary, ImageSummary, DockerEvent } from "../api-types";
    import { configStore } from "../stores/config.svelte";

    let { containers, images, events, onRefresh, onNavigate } = $props<{ 
        containers: ContainerSummary[], 
        images?: ImageSummary[], 
        events?: DockerEvent[],
        onRefresh?: () => void,
        onNavigate: (route: string, params?: any) => void
    }>();
    let scanSummary = $state<ScanSummary | null>(null);
    let fleetAdvice = $state("");
    let fleetAdviceTs = $state<number | null>(null);
    let analyzingFleet = $state(false);
    let schedules = $state<any[]>([]);

    async function loadData() {
        try {
            const [scanRes, schedRes, adviceRes] = await Promise.all([
                fetch("/api/scans/summary"),
                fetch("/api/scheduler/schedules"),
                fetch("/api/ai/fleet-advice")
            ]);
            if (scanRes.ok) scanSummary = await scanRes.json();
            if (schedRes.ok) schedules = await schedRes.json();
            if (adviceRes.ok) {
                const adviceData = await adviceRes.json();
                fleetAdvice = adviceData.advice;
                fleetAdviceTs = adviceData.timestamp;
            }
        } catch (e) {
            console.error("Failed to load dashboard data", e);
        }
    }

    async function getFleetAdvice() {
        if (containers.length === 0) return;
        analyzingFleet = true;
        try {
            const res = await fetch("/api/ai/fleet-advice", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(containers)
            });
            if (res.ok) {
                const data = await res.json();
                fleetAdvice = data.advice;
                fleetAdviceTs = Math.floor(Date.now() / 1000);
            }
        } catch (e) {
            console.error("Fleet analysis failed", e);
        } finally {
            analyzingFleet = false;
        }
    }

    onMount(() => {
        loadData();
    });

    let safeContainers = $derived(containers || []);
    let runningCount = $derived(safeContainers.filter((c: ContainerSummary) => c.state === 'running').length);
    let updateCount = $derived(safeContainers.filter((c: ContainerSummary) => c.updateAvailable).length);

    // Automation Domain Helpers
    function getDomainStatus(taskIds: string[]) {
        const scoped = schedules.filter(s => taskIds.includes(s.id));
        if (scoped.length === 0) return 'idle';
        return scoped.some(s => s.enabled) ? 'active' : 'paused';
    }

    const domains = $derived([
        { label: 'Upgrades', status: getDomainStatus(['container_update_check', 'container_update_apply']), color: 'text-sky-500' },
        { label: 'Maintenance', status: getDomainStatus(['docker_system_prune', 'metrics_prune']), color: 'text-teal-500' },
        { label: 'Security', status: getDomainStatus(['security_sweep_trivy', 'malware_sweep_clamav']), color: 'text-orange-500' }
    ]);

    // Top Vulnerable Images
    async function loadTopVulnerabilities() {
        try {
            const res = await fetch("/api/docker/images/intelligence");
            if (res.ok) {
                const data = await res.json();
                return (Array.isArray(data) ? data : [])
                    .filter(img => (img.vulnerabilityCritical || 0) > 0 || (img.vulnerabilityHigh || 0) > 0)
                    .map(img => {
                        // Find a container using this image to provide a link
                        const container = safeContainers.find((c: ContainerSummary) => c.image === img.primaryRef || (img.repoTags && img.repoTags.includes(c.image)));
                        return { ...img, containerId: container?.id };
                    })
                    .sort((a, b) => (b.vulnerabilityCritical || 0) - (a.vulnerabilityCritical || 0))
                    .slice(0, 5);
            }
        } catch (e) { return []; }
        return [];
    }

    let topRisks = $state<any[]>([]);
    $effect(() => {
        loadTopVulnerabilities().then(data => topRisks = data);
    });
</script>

<div class="space-y-8">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-5 py-1">
            <h2 class="text-3xl font-black text-slate-900 dark:text-white tracking-tighter uppercase">Command Oversight</h2>
            <p class="text-sm text-slate-500 font-medium mt-1">Global posture and intelligence summary for the managed fleet.</p>
        </div>
        <div class="flex gap-2">
            <div class="px-4 py-2 bg-slate-100 dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 flex items-center gap-3">
                <div class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></div>
                <span class="text-[10px] font-black uppercase tracking-widest text-slate-500">System Nominal</span>
            </div>
        </div>
    </div>

    <!-- Quick Stats -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 opacity-0 animate-reveal stagger-1">
        <button 
            onclick={() => onNavigate('containers')}
            class="bg-white dark:bg-slate-800 p-5 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl relative overflow-hidden group text-left transition-all hover:border-brand-500 hover:scale-[1.02]"
        >
            <div class="absolute top-0 right-0 p-4 opacity-5 group-hover:scale-110 transition-transform">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" /></svg>
            </div>
            <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-2">Active Assets</p>
            <p class="text-3xl font-black text-slate-900 dark:text-white tracking-tighter">{runningCount}<span class="text-base text-slate-400 ml-2 font-medium">/ {safeContainers.length}</span></p>
        </button>

        <button 
            onclick={() => onNavigate('containers', { filter: 'updates' })}
            class="bg-white dark:bg-slate-800 p-5 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl relative overflow-hidden group text-left transition-all hover:border-amber-500 hover:scale-[1.02]"
        >
            <div class="absolute top-0 right-0 p-4 opacity-5 group-hover:scale-110 transition-transform">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
            </div>
            <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-2">Updates Detected</p>
            <p class="text-3xl font-black {updateCount > 0 ? 'text-amber-500' : 'text-slate-900 dark:text-white'} tracking-tighter">{updateCount}</p>
        </button>

        <button 
            onclick={() => onNavigate('containers', { filter: 'high-risk' })}
            class="bg-white dark:bg-slate-800 p-5 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl relative overflow-hidden group text-left transition-all hover:border-rose-500 hover:scale-[1.02]"
        >
            <div class="absolute top-0 right-0 p-4 opacity-5 group-hover:scale-110 transition-transform text-rose-500">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" /></svg>
            </div>
            <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-2">Vulnerabilities</p>
            <p class="text-3xl font-black {(scanSummary?.critical || 0) > 0 ? 'text-rose-600' : 'text-slate-900 dark:text-white'} tracking-tighter">{scanSummary?.critical || 0}</p>
        </button>

        <button 
            onclick={() => onNavigate('audit')}
            class="bg-white dark:bg-slate-800 p-5 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-xl relative overflow-hidden group text-left transition-all hover:border-brand-500 hover:scale-[1.02]"
        >
            <div class="absolute top-0 right-0 p-4 opacity-5 group-hover:scale-110 transition-transform">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>
            </div>
            <p class="text-[10px] font-black text-slate-400 uppercase tracking-widest mb-2">Security Scans</p>
            <p class="text-3xl font-black text-slate-900 dark:text-white tracking-tighter">{scanSummary?.total || 0}</p>
        </button>
    </div>

    <div class="grid grid-cols-1 xl:grid-cols-2 gap-8 opacity-0 animate-reveal stagger-2">
        <!-- Automation & Compliance -->
        <div class="space-y-6">
            <div class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden h-full">
                <div class="p-6 border-b border-slate-100 dark:border-slate-700 flex justify-between items-center bg-slate-50/50 dark:bg-slate-900/20">
                    <h3 class="text-sm font-black uppercase tracking-widest text-slate-400">Fleet Compliance & Status</h3>
                    <button onclick={() => onNavigate('settings')} class="text-[10px] font-black uppercase text-brand-600 hover:underline">Manage Policy</button>
                </div>
                <div class="p-6 space-y-8">
                    <!-- Automation States -->
                    <div class="grid grid-cols-3 gap-4">
                        {#each domains as domain}
                            <div class="text-center space-y-2">
                                <p class="text-[9px] font-black uppercase text-slate-400 tracking-tighter">{domain.label}</p>
                                <div class="inline-flex items-center gap-1.5 px-2 py-1 rounded-lg {domain.status === 'active' ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-100 text-slate-500'}">
                                    <div class="w-1.5 h-1.5 rounded-full {domain.status === 'active' ? 'bg-emerald-500 animate-pulse' : 'bg-slate-400'}"></div>
                                    <span class="text-[10px] font-black uppercase">{domain.status}</span>
                                </div>
                            </div>
                        {/each}
                    </div>

                    <!-- Top Risks -->
                    <div class="space-y-4">
                        <div class="flex items-center justify-between">
                            <p class="text-[10px] font-black uppercase text-slate-400">Top Image Risks</p>
                            <span class="text-[9px] font-bold text-slate-400 italic">Critical finding sort</span>
                        </div>
                        <div class="space-y-2">
                            {#each topRisks as risk}
                                <div class="flex items-center justify-between p-3 rounded-2xl bg-slate-50 dark:bg-slate-900/40 border border-slate-100 dark:border-slate-700">
                                    <div class="min-w-0 flex-1">
                                        <p class="text-xs font-bold text-slate-700 dark:text-slate-200 truncate">{risk.repoTags?.[0] || risk.primaryRef}</p>
                                        <p class="text-[9px] text-slate-400 uppercase mt-0.5">{risk.inUse ? 'Active in Fleet' : 'Orphaned'}</p>
                                    </div>
                                    <div class="flex items-center gap-2 ml-4">
                                        <span class="px-2 py-0.5 rounded-lg bg-rose-100 text-rose-700 text-[10px] font-black">{risk.vulnerabilityCritical || 0} Critical</span>
                                        {#if risk.containerId}
                                            <button onclick={() => onNavigate('container-detail', { id: risk.containerId, tab: 'security' })} class="p-1.5 text-slate-400 hover:text-brand-600">
                                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" /></svg>
                                            </button>
                                        {/if}
                                    </div>
                                </div>
                            {:else}
                                <p class="text-xs text-slate-400 italic text-center py-4">No critical security risks detected.</p>
                            {/each}
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Recent Activity Feed -->
        <div class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden h-full">
            <div class="p-6 border-b border-slate-100 dark:border-slate-700 flex justify-between items-center bg-slate-50/50 dark:bg-slate-900/20">
                <h3 class="text-sm font-black uppercase tracking-widest text-slate-400">System Activity & Events</h3>
                <button onclick={() => onNavigate('diagnostics')} class="text-[10px] font-black uppercase text-brand-600 hover:underline">View Logs</button>
            </div>
            <div class="divide-y divide-slate-50 dark:divide-slate-700/50 overflow-y-auto max-h-[480px]">
                {#each (events || []).slice(0, 15) as event}
                    <div class="p-4 hover:bg-slate-50 dark:hover:bg-slate-900/20 transition-colors flex gap-4">
                        <div class="w-8 h-8 rounded-full bg-slate-100 dark:bg-slate-900 flex items-center justify-center flex-shrink-0">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                            </svg>
                        </div>
                        <div class="min-w-0 flex-1">
                            <div class="flex items-center justify-between gap-2">
                                <p class="text-[11px] font-bold text-slate-700 dark:text-slate-200 uppercase">{event.action}</p>
                                <span class="text-[9px] text-slate-400 font-medium">{new Date(event.time * 1000).toLocaleTimeString()}</span>
                            </div>
                            <p class="text-[10px] text-slate-500 mt-0.5 truncate">{event.attributes?.name || event.from || 'System'}</p>
                        </div>
                    </div>
                {:else}
                    <div class="py-20 text-center text-slate-400 italic">No recent activity recorded.</div>
                {/each}
            </div>
        </div>
    </div>

    <!-- AI Advisor -->
    {#if configStore.aiActive}
        <div class="bg-slate-900 rounded-2xl p-6 md:p-7 border border-slate-800 shadow-2xl relative overflow-hidden opacity-0 animate-reveal stagger-2">
            <div class="absolute top-0 right-0 w-1/3 h-full bg-gradient-to-l from-brand-600/10 to-transparent pointer-events-none"></div>
            
            <div class="relative z-10 space-y-6">
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-4">
                        <div class="w-11 h-11 rounded-xl bg-brand-600 flex items-center justify-center text-white shadow-xl shadow-brand-500/20">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
                        </div>
                        <div>
                            <h3 class="text-lg font-black text-white uppercase tracking-tight">Fleet Intelligence Advisor</h3>
                            <p class="text-xs text-slate-400 font-medium">Heuristic analysis of your current deployment state.</p>
                        </div>
                    </div>
                    <button 
                        onclick={getFleetAdvice}
                        disabled={analyzingFleet}
                        class="px-7 py-2.5 bg-white hover:bg-slate-100 text-slate-900 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all disabled:opacity-50 active:scale-95 shadow-xl"
                    >
                        {analyzingFleet ? 'Processing Fleet Data...' : 'Generate AI Advice'}
                    </button>
                            </div>
                
                            {#if fleetAdvice}
                                <div class="space-y-3">
                                    {#if fleetAdviceTs}
                                        <p class="text-[10px] font-black text-brand-500 uppercase tracking-widest ml-1">
                                            Last generated: {new Date(fleetAdviceTs * 1000).toLocaleString()}
                                        </p>
                                    {/if}
                                    <div class="bg-slate-950/50 border border-slate-800 rounded-2xl p-5">
                                        <div class="prose prose-invert prose-sm max-w-none text-slate-300 italic leading-relaxed whitespace-pre-wrap">
                                            {fleetAdvice}
                                        </div>
                                    </div>
                                </div>
                            {:else}
                
                    <div class="py-12 text-center">
                        <p class="text-slate-500 text-sm font-medium italic">Request a fresh analysis to see proactive security and maintenance recommendations.</p>
                    </div>
                {/if}
            </div>
        </div>
    {/if}
</div>
