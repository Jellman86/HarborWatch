<script lang="ts">
    import { onMount } from "svelte";
    import PaginationBar from "../components/PaginationBar.svelte";
    import GitOps from "./GitOps.svelte";
    import { toasts } from "../stores/ToastStore";
    import { configStore } from "../stores/config.svelte";

    let { onNavigate, params = null } = $props<{
        onNavigate: (route: string, params?: any) => void;
        params?: { tab?: string } | null;
    }>();

    interface LocalComposeProjectMember {
        containerId: string;
        containerName?: string;
        serviceName?: string;
        image?: string;
        state?: string;
        updateAvailable?: boolean;
    }

    interface LocalComposeProject {
        projectKey?: string;
        projectName: string;
        workingDir?: string;
        configFiles?: string[];
        sourceStatus: "unverified" | "verified_readonly" | "verified_writable" | string;
        sourceVerified?: boolean;
        sourceWritable?: boolean;
        composeEditable?: boolean;
        envEditable?: boolean;
        envCreatable?: boolean;
        envPath?: string;
        containerCount?: number;
        updateCandidates?: number;
        snapshotRootPath?: string;
        snapshotStatus?: "none" | "unchanged" | "changed" | string;
        snapshotCount?: number;
        changedFiles?: number;
        lastSnapshotAt?: number;
        lastSnapshotPath?: string;
        snapshotError?: string;
        members?: LocalComposeProjectMember[];
    }

    // Component State
    let stacks = $state<any[]>([]);
    let composeProjects = $state<LocalComposeProject[]>([]);
    let loading = $state(true);
    let redeploying = $state<Record<number, boolean>>({});
    let portainerError = $state("");
    let composeError = $state("");
    let pageIndex = $state(0);
    let composePageIndex = $state(0);
    const pageSize = 12;
    let activeSourceTab = $state<"compose" | "portainer" | "repositories">("compose");

    async function loadStacks() {
        loading = true;
        portainerError = "";
        composeError = "";
        const [portainerResult, composeResult] = await Promise.allSettled([
            fetch("/api/portainer/stacks"),
            fetch("/api/compose/projects")
        ]);

        if (portainerResult.status === "fulfilled") {
            const res = portainerResult.value;
            if (res.ok) {
                const data = await res.json().catch(() => []);
                stacks = Array.isArray(data) ? data : [];
            } else if (res.status === 503) {
                stacks = [];
                portainerError = "Portainer integration is not configured. Enable it in Settings.";
            } else {
                const data = await res.json().catch(() => ({}));
                stacks = [];
                portainerError = data.message || "Failed to load Portainer stacks";
            }
        } else {
            stacks = [];
            portainerError = "Could not connect to backend for Portainer stack discovery";
        }

        if (composeResult.status === "fulfilled") {
            const res = composeResult.value;
            if (res.ok) {
                const data = await res.json().catch(() => []);
                composeProjects = Array.isArray(data) ? data : [];
            } else {
                const data = await res.json().catch(() => ({}));
                composeProjects = [];
                composeError = data.message || "Failed to load local Compose projects";
            }
        } else {
            composeProjects = [];
            composeError = "Could not connect to backend for local Compose discovery";
        }

        loading = false;
    }

    async function redeployStack(id: number) {
        if (redeploying[id]) return;
        redeploying[id] = true;
        toasts.info(`Triggering redeploy for stack ${id}...`);
        try {
            const res = await fetch(`/api/portainer/stacks/${id}/redeploy`, {
                method: "POST"
            });
            const data = await res.json();
            if (res.ok) {
                toasts.success(data.message || "Redeploy triggered successfully");
                await loadStacks();
            } else {
                toasts.error(data.message || "Redeploy failed");
            }
        } catch (e) {
            toasts.error("Failed to trigger redeploy");
        } finally {
            redeploying[id] = false;
        }
    }

    onMount(() => {
        loadStacks();
    });

    $effect(() => {
        const requested = String(params?.tab || "").toLowerCase();
        if (requested === "repositories") {
            activeSourceTab = "repositories";
            return;
        }
        if (requested === "portainer") {
            activeSourceTab = configStore.portainerActive ? "portainer" : "compose";
            return;
        }
        if (requested === "compose") {
            activeSourceTab = "compose";
        }
    });

    const stackType = (type: number) => {
        switch(type) {
            case 1: return "Swarm";
            case 2: return "Compose";
            default: return "Unknown";
        }
    };

    const statusColor = (status: number) => {
        switch(status) {
            case 1: return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"; // Active
            case 2: return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400"; // Inactive
            default: return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-400";
        }
    };

    const totalPages = $derived(Math.max(1, Math.ceil(stacks.length / pageSize)));
    const pagedStacks = $derived(stacks.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));
    const pageStart = $derived(stacks.length === 0 ? 0 : (pageIndex * pageSize) + 1);
    const pageEnd = $derived(Math.min((pageIndex + 1) * pageSize, stacks.length));
    const composeTotalPages = $derived(Math.max(1, Math.ceil(composeProjects.length / pageSize)));
    const pagedComposeProjects = $derived(composeProjects.slice(composePageIndex * pageSize, (composePageIndex + 1) * pageSize));
    const composePageStart = $derived(composeProjects.length === 0 ? 0 : (composePageIndex * pageSize) + 1);
    const composePageEnd = $derived(Math.min((composePageIndex + 1) * pageSize, composeProjects.length));

    $effect(() => {
        stacks.length;
        if (pageIndex > totalPages - 1) {
            pageIndex = Math.max(0, totalPages - 1);
        }
    });

    $effect(() => {
        composeProjects.length;
        if (composePageIndex > composeTotalPages - 1) {
            composePageIndex = Math.max(0, composeTotalPages - 1);
        }
    });

    function composeSourceStatusLabel(status: string | undefined): string {
        const value = String(status || "").trim();
        if (value === "verified_writable") return "Verified / Writable";
        if (value === "verified_readonly") return "Verified / Read-Only";
        return "Unverified";
    }

    function composeSourceStatusClass(status: string | undefined): string {
        const value = String(status || "").trim();
        if (value === "verified_writable") return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        if (value === "verified_readonly") return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function snapshotStatusLabel(status: string | undefined): string {
        const value = String(status || "").trim();
        if (value === "changed") return "Changed Since Snapshot";
        if (value === "unchanged") return "Unchanged Since Snapshot";
        return "No Snapshot";
    }

    function snapshotStatusClass(status: string | undefined): string {
        const value = String(status || "").trim();
        if (value === "changed") return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300";
        if (value === "unchanged") return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function composeAuthorityLabel(project: LocalComposeProject): string {
        if (project.composeEditable) return "Compose Editable";
        if (project.sourceVerified) return "Compose Read-Only";
        return "Compose Unverified";
    }

    function composeAuthorityClass(project: LocalComposeProject): string {
        if (project.composeEditable) return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        if (project.sourceVerified) return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function envAuthorityLabel(project: LocalComposeProject): string {
        if (project.envEditable) return "Local .env Editable";
        if (project.envCreatable) return "Local .env Creatable";
        if (project.envPath) return "Local .env Read-Only";
        return "No Local .env";
    }

    function envAuthorityClass(project: LocalComposeProject): string {
        if (project.envEditable || project.envCreatable) return "bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300";
        if (project.envPath) return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function automationSummary(project: LocalComposeProject): string {
        if (project.composeEditable) return "Compose + .env managed locally";
        if (project.sourceVerified && (project.envEditable || project.envCreatable)) return "Git-managed compose, local .env override";
        if (project.sourceVerified) return "Compose-aware, read-only source";
        return "Needs source access";
    }

    function openComposeEditor(project: LocalComposeProject) {
        onNavigate("compose-project-detail", {
            projectKey: project.projectKey || project.projectName
        });
    }
</script>



<div class="space-y-6">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">Stack Explorer</h2>
            <p class="text-xs text-slate-500 font-medium">Portainer stacks and local Docker Compose project discovery.</p>
        </div>
        <button 
            onclick={loadStacks}
            class="w-full md:w-auto px-4 py-2 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-xl font-bold transition-all border border-slate-200 dark:border-slate-700 shadow-sm flex items-center justify-center gap-2"
        >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Refresh
        </button>
    </div>

    <div class="inline-flex flex-nowrap overflow-x-auto hide-scrollbar gap-6 w-full border-b border-slate-200/50 dark:border-slate-800/50 pb-2 mb-6">
        <button
            onclick={() => activeSourceTab = "compose"}
            class="py-2 text-[10px] font-black uppercase tracking-widest transition-all flex items-center gap-2 whitespace-nowrap border-b-2 {activeSourceTab === 'compose' ? 'text-brand-600 border-brand-600' : 'text-slate-500 border-transparent hover:text-slate-700 dark:hover:text-slate-300'}"
        >
            Compose Files
            <span class="px-1.5 py-0.5 rounded-md bg-slate-200/80 dark:bg-slate-800 text-[8px] font-black text-slate-600 dark:text-slate-300">{composeProjects.length}</span>
        </button>
        {#if configStore.portainerActive}
            <button
                onclick={() => activeSourceTab = "portainer"}
                class="py-2 text-[10px] font-black uppercase tracking-widest transition-all flex items-center gap-2 whitespace-nowrap border-b-2 {activeSourceTab === 'portainer' ? 'text-brand-600 border-brand-600' : 'text-slate-500 border-transparent hover:text-slate-700 dark:hover:text-slate-300'}"
            >
                Portainer Stacks
                <span class="px-1.5 py-0.5 rounded-md bg-slate-200/80 dark:bg-slate-800 text-[8px] font-black text-slate-600 dark:text-slate-300">{stacks.length}</span>
            </button>
        {/if}
        <button
            onclick={() => activeSourceTab = "repositories"}
            class="py-2 text-[10px] font-black uppercase tracking-widest transition-all flex items-center gap-2 whitespace-nowrap border-b-2 {activeSourceTab === 'repositories' ? 'text-brand-600 border-brand-600' : 'text-slate-500 border-transparent hover:text-slate-700 dark:hover:text-slate-300'}"
        >
            Repositories
        </button>
    </div>

    {#if loading}
        <div class="flex flex-col items-center justify-center py-20 gap-4 text-slate-400">
            <div class="w-8 h-8 border-4 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
            <span class="text-sm font-black uppercase tracking-widest">Discovering Orchestration Sources...</span>
        </div>
    {:else}
        {#if activeSourceTab === "compose"}
        <section class="space-y-4">
            <div class="flex items-center justify-between gap-3">
                <div>
                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Local Compose Projects</p>
                    <p class="text-xs text-slate-500">Discovered from running containers with Docker Compose labels (non-Portainer).</p>
                </div>
                <span class="px-2 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300">
                    {composeProjects.length} Projects
                </span>
            </div>

            {#if composeError}
                <div class="rounded-2xl border border-rose-200 dark:border-rose-900/40 bg-rose-50 dark:bg-rose-900/10 p-4 text-sm text-rose-700 dark:text-rose-300">
                    {composeError}
                </div>
            {:else if composeProjects.length === 0}
                <div class="bg-white dark:bg-slate-800 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-700 p-10 text-center">
                    <p class="text-slate-400 italic font-medium">No local Docker Compose projects discovered from running containers.</p>
                </div>
            {:else}
                <PaginationBar
                    summaryText={`Showing ${composePageStart}-${composePageEnd} of ${composeProjects.length}`}
                    pageText={`Page ${composePageIndex + 1}/${composeTotalPages}`}
                    canPrev={composePageIndex > 0}
                    canNext={composePageIndex < composeTotalPages - 1}
                    onPrev={() => composePageIndex = Math.max(0, composePageIndex - 1)}
                    onNext={() => composePageIndex = Math.min(composeTotalPages - 1, composePageIndex + 1)}
                />
                <div class="grid grid-cols-1 xl:grid-cols-2 gap-10 opacity-0 animate-reveal stagger-1">
                    {#each pagedComposeProjects as p, i}
                        <article style="animation-delay: {0.1 + (i * 0.05)}s" class="opacity-0 animate-reveal bg-white dark:bg-slate-800 rounded-3xl border border-slate-200/60 dark:border-slate-800/60 overflow-hidden flex flex-col h-full group hover:border-brand-500 hover:shadow-md transition-all">
                            <!-- Header Area -->
                            <div class="p-6 border-b border-slate-100 dark:border-slate-700/50 space-y-4">
                                <div class="flex items-start justify-between gap-3">
                                    <div class="min-w-0">
                                        <h3 class="text-xl font-black tracking-tight text-slate-900 dark:text-white truncate group-hover:text-brand-600 transition-colors" title={p.projectName}>
                                            <button class="hover:underline" onclick={() => openComposeEditor(p)}>{p.projectName}</button>
                                        </h3>
                                        <p class="text-[10px] font-mono text-slate-400 mt-1 truncate" title={p.workingDir || (p.configFiles && p.configFiles[0]) || "Path unavailable"}>
                                            {p.workingDir || (p.configFiles && p.configFiles[0]) || "Path unavailable"}
                                        </p>
                                    </div>
                                    {#if p.updateCandidates && p.updateCandidates > 0}
                                        <span class="px-2.5 py-1 rounded-full text-[10px] font-black uppercase tracking-widest bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 whitespace-nowrap">
                                            {p.updateCandidates} Update{p.updateCandidates > 1 ? 's' : ''}
                                        </span>
                                    {:else}
                                        <span class="px-2.5 py-1 rounded-full text-[10px] font-black uppercase tracking-widest bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 whitespace-nowrap">
                                            Up to date
                                        </span>
                                    {/if}
                                </div>

                                <!-- Badges Row -->
                                <div class="flex flex-wrap gap-2">
                                    <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase tracking-widest {composeSourceStatusClass(p.sourceStatus)}" title="Source Status">
                                        {composeSourceStatusLabel(p.sourceStatus)}
                                    </span>
                                    <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase tracking-widest {composeAuthorityClass(p)}" title="Compose Authority">
                                        {composeAuthorityLabel(p)}
                                    </span>
                                    <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase tracking-widest {envAuthorityClass(p)}" title="Env Authority">
                                        {envAuthorityLabel(p)}
                                    </span>
                                    <span class="px-2 py-1 rounded-md text-[9px] font-black uppercase tracking-widest {snapshotStatusClass(p.snapshotStatus)}" title="Protection Status">
                                        Snapshots: {snapshotStatusLabel(p.snapshotStatus)}
                                    </span>
                                </div>
                            </div>

                            <!-- Services List -->
                            <div class="flex-1 p-6 bg-slate-50/30 dark:bg-slate-900/20">
                                <p class="text-[10px] font-black uppercase tracking-widest text-slate-400 mb-3">Containers / Services ({p.containerCount || 0})</p>
                                <div class="space-y-2">
                                    {#each p.members || [] as m}
                                        <button
                                            onclick={(event) => {
                                                event.stopPropagation();
                                                onNavigate('container-detail', { id: m.containerId, tab: 'lifecycle' });
                                            }}
                                            class="w-full flex items-center justify-between p-3 rounded-xl border border-slate-200/60 dark:border-slate-700/60 bg-white dark:bg-slate-800 hover:border-brand-400 hover:shadow-sm transition-all group/svc"
                                        >
                                            <div class="flex items-center gap-3 min-w-0">
                                                <div class="w-2 h-2 rounded-full {m.state === 'running' ? 'bg-emerald-500' : 'bg-slate-400'}"></div>
                                                <div class="text-left min-w-0">
                                                    <p class="text-xs font-bold text-slate-800 dark:text-slate-100 truncate">{m.serviceName || m.containerName || m.containerId}</p>
                                                    <p class="text-[9px] font-mono text-slate-500 truncate">{m.image || 'Unknown image'}</p>
                                                </div>
                                            </div>
                                            <div class="flex items-center gap-3 shrink-0">
                                                {#if m.updateAvailable}
                                                    <span class="w-2 h-2 rounded-full bg-brand-500" title="Update Available"></span>
                                                {/if}
                                                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-slate-300 group-hover/svc:text-brand-500 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                                                </svg>
                                            </div>
                                        </button>
                                    {/each}
                                </div>
                            </div>

                            <!-- Actions -->
                            <div class="p-6 border-t border-slate-100 dark:border-slate-700/50 bg-white dark:bg-slate-800">
                                <button
                                    onclick={(event) => {
                                        event.stopPropagation();
                                        openComposeEditor(p);
                                    }}
                                    class="w-full px-4 py-2.5 rounded-xl bg-brand-600 hover:bg-brand-700 text-white text-[10px] font-black uppercase tracking-widest transition-all shadow-md shadow-brand-500/20"
                                >
                                    Open Compose Editor
                                </button>
                            </div>
                        </article>
                    {/each}
                </div>
            {/if}
        </section>
        {/if}

        {#if configStore.portainerActive && activeSourceTab === "portainer"}
        <section class="space-y-4 pt-2">
            <div class="flex items-center justify-between gap-3">
                <div>
                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Portainer Stacks</p>
                    <p class="text-xs text-slate-500">Remote orchestration discovery and redeploy via Portainer API.</p>
                </div>
                <span class="px-2 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300">
                    {stacks.length} Stacks
                </span>
            </div>

            {#if portainerError}
                <div class="bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30 rounded-3xl p-6 text-center space-y-3 animate-reveal">
                    <h3 class="text-base font-bold text-slate-900 dark:text-white">Portainer Integration Unavailable</h3>
                    <p class="text-sm text-slate-500 max-w-xl mx-auto">{portainerError}</p>
                </div>
            {:else if stacks.length === 0}
                <div class="bg-white dark:bg-slate-800 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-700 p-12 text-center animate-reveal">
                    <p class="text-slate-400 italic font-medium">No stacks discovered in the configured Portainer endpoint.</p>
                </div>
            {:else}
                <PaginationBar
                    summaryText={`Showing ${pageStart}-${pageEnd} of ${stacks.length}`}
                    pageText={`Page ${pageIndex + 1}/${totalPages}`}
                    canPrev={pageIndex > 0}
                    canNext={pageIndex < totalPages - 1}
                    onPrev={() => pageIndex = Math.max(0, pageIndex - 1)}
                    onNext={() => pageIndex = Math.min(totalPages - 1, pageIndex + 1)}
                />
                <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-10 opacity-0 animate-reveal stagger-1">
                    {#each pagedStacks as s, i}
                        <article style="animation-delay: {0.1 + (i * 0.05)}s" class="opacity-0 animate-reveal bg-white dark:bg-slate-800 rounded-3xl border border-slate-200/60 dark:border-slate-800/60 overflow-hidden flex flex-col h-full group hover:border-brand-500 hover:shadow-md transition-all">
                            <div class="p-6 border-b border-slate-100 dark:border-slate-700/50 space-y-4">
                                <div class="flex items-start justify-between gap-3">
                                    <div class="min-w-0 flex-1">
                                        <h3 class="font-black text-slate-900 dark:text-white truncate text-xl tracking-tight group-hover:text-brand-600 transition-colors" title={s.Name}>
                                            {s.Name}
                                        </h3>
                                        <p class="text-[10px] font-mono text-slate-400 mt-1 uppercase tracking-widest truncate">ID: {s.Id} | EP-{s.EndpointId}</p>
                                    </div>
                                    <span class="px-2.5 py-1 rounded-full text-[10px] font-black uppercase tracking-widest {statusColor(s.Status)} whitespace-nowrap">
                                        {s.Status === 1 ? 'Active' : 'Inactive'}
                                    </span>
                                </div>
                                <div class="flex flex-wrap gap-2">
                                    <span class="px-2 py-1 rounded-md bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 text-[9px] font-black uppercase tracking-widest border border-slate-200 dark:border-slate-700">
                                        Engine: {stackType(s.Type)}
                                    </span>
                                    <span class="px-2 py-1 rounded-md bg-cyan-50 dark:bg-cyan-900/20 text-[9px] font-black text-cyan-700 dark:text-cyan-300 uppercase tracking-widest border border-cyan-100 dark:border-cyan-900/30">
                                        Portainer API
                                    </span>
                                </div>
                            </div>
                            
                            <div class="flex-1 p-6 bg-slate-50/30 dark:bg-slate-900/20 flex flex-col justify-center items-center text-center">
                                <div class="w-12 h-12 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center mb-3 text-slate-400">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 002-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                                    </svg>
                                </div>
                                <p class="text-sm font-bold text-slate-700 dark:text-slate-300">Remote Stack</p>
                                <p class="text-[11px] text-slate-500 mt-1 max-w-[200px]">Managed through Portainer. Detailed service introspection is limited.</p>
                            </div>

                            <div class="p-6 border-t border-slate-100 dark:border-slate-700/50 bg-white dark:bg-slate-800 flex flex-col gap-2">
                                <button
                                    onclick={() => onNavigate('containers', { search: s.Name })}
                                    class="w-full px-4 py-2.5 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 text-[10px] font-black uppercase tracking-widest rounded-xl transition-all border border-slate-200 dark:border-slate-700"
                                >
                                    View Containers
                                </button>
                                {#if s.Type === 2}
                                    <button
                                        onclick={() => redeployStack(s.Id)}
                                        disabled={redeploying[s.Id]}
                                        class="w-full px-4 py-2.5 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest rounded-xl transition-all shadow-md shadow-brand-500/20 flex justify-center items-center gap-2"
                                    >
                                        {#if redeploying[s.Id]}
                                            <div class="w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                                        {:else}
                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                            </svg>
                                        {/if}
                                        Redeploy
                                    </button>
                                {/if}
                            </div>
                        </article>
                    {/each}
                </div>
            {/if}
        </section>
        {/if}
        {#if activeSourceTab === "repositories"}
        <section class="pt-2">
            <GitOps {onNavigate} />
        </section>
        {/if}
    {/if}
</div>
