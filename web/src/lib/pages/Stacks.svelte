<script lang="ts">
    import { onDestroy, onMount } from "svelte";
    import PaginationBar from "../components/PaginationBar.svelte";
    import PortainerLogo from "../components/PortainerLogo.svelte";
    import GitOps from "./GitOps.svelte";
    import StaticNoise from "../components/StaticNoise.svelte";
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

    let stacks = $state<any[]>([]);
    let composeProjects = $state<LocalComposeProject[]>([]);
    let loading = $state(true);
    let redeploying = $state<Record<number, boolean>>({});
    let redeployJobIds = $state<Record<number, string>>({});
    let redeployStateTimer: ReturnType<typeof setInterval> | null = null;
    let redeployStatePolling = false;
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

    function redeployTargetForStack(id: number): string {
        return `portainer-stack-${id}`;
    }

    function startRedeployStatePolling() {
        if (redeployStateTimer || Object.keys(redeployJobIds).length === 0) return;
        redeployStateTimer = setInterval(() => {
            void refreshRedeployStates();
        }, 3000);
    }

    function stopRedeployStatePolling() {
        if (redeployStateTimer) {
            clearInterval(redeployStateTimer);
            redeployStateTimer = null;
        }
    }

    async function refreshRedeployStates() {
        if (redeployStatePolling || Object.keys(redeployJobIds).length === 0) return;
        redeployStatePolling = true;
        try {
            const res = await fetch("/api/diagnostics/snapshot?includeFleet=false&logLimit=0&auditLimit=0");
            if (!res.ok) return;
            const data = await res.json().catch(() => ({}));
            const activeJobs = Array.isArray(data?.activeJobs) ? data.activeJobs : [];
            const activeById = new Set(activeJobs.filter((job: any) => String(job?.status || "").toLowerCase() === "queued" || String(job?.status || "").toLowerCase() === "running").map((job: any) => String(job?.id || "")));
            const activeByTarget = new Set(activeJobs.filter((job: any) => String(job?.status || "").toLowerCase() === "queued" || String(job?.status || "").toLowerCase() === "running").map((job: any) => String(job?.target || "")));
            let hasPending = false;
            for (const [stackIdText, jobId] of Object.entries(redeployJobIds)) {
                const stackId = Number(stackIdText);
                const target = redeployTargetForStack(stackId);
                const stillActive = activeById.has(jobId) || activeByTarget.has(target);
                redeploying[stackId] = stillActive;
                if (!stillActive) {
                    delete redeployJobIds[stackId];
                } else {
                    hasPending = true;
                }
            }
            if (!hasPending) {
                stopRedeployStatePolling();
            }
        } catch {
            // Keep the existing card state until the next poll succeeds.
        } finally {
            redeployStatePolling = false;
        }
    }

    async function redeployStack(id: number) {
        if (redeploying[id]) return;
        redeploying[id] = true;
        toasts.info(`Triggering redeploy for stack ${id}...`);
        try {
            const res = await fetch(`/api/portainer/stacks/${id}/redeploy`, {
                method: "POST"
            });
            const data = await res.json().catch(() => ({}));
            if (res.ok) {
                const jobId = String(data.jobId || "").trim();
                toasts.success(data.message || "Redeploy triggered successfully");
                if (jobId) {
                    redeployJobIds[id] = jobId;
                    startRedeployStatePolling();
                    void refreshRedeployStates();
                } else {
                    toasts.error("Redeploy started, but the job identifier was missing.");
                    redeploying[id] = false;
                }
                await loadStacks();
            } else {
                toasts.error(data.message || "Redeploy failed");
                redeploying[id] = false;
            }
        } catch (e) {
            toasts.error("Failed to trigger redeploy");
            redeploying[id] = false;
        }
    }

    onMount(() => {
        loadStacks();
    });

    onDestroy(() => {
        stopRedeployStatePolling();
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
        if (value === "changed") return "Snapshot Changed";
        if (value === "unchanged") return "Snapshot OK";
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
        if (project.envEditable) return ".env Editable";
        if (project.envCreatable) return ".env Creatable";
        if (project.envPath) return ".env Read-Only";
        return "No .env";
    }

    function envAuthorityClass(project: LocalComposeProject): string {
        if (project.envEditable || project.envCreatable) return "bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300";
        if (project.envPath) return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function stackTypeLabel(type: number): string {
        if (type === 1) return "Swarm";
        if (type === 2) return "Compose";
        return "Unknown";
    }

    function stackStatusLabel(status: number): string {
        if (status === 1) return "Active";
        if (status === 2) return "Inactive";
        return "Unknown";
    }

    function stackStatusClass(status: number, busy = false): string {
        if (busy) return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        if (status === 1) return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        if (status === 2) return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function stackStateSummary(stack: any, busy = false): string {
        if (busy) return "Redeploy in progress…";
        if (stack.Status === 1) return "Healthy orchestration target";
        if (stack.Status === 2) return "Stack is not currently active";
        return "Status unavailable";
    }

    function portainerCardAccent(stack: any): string {
        if (redeploying[stack.Id]) return "from-amber-400/60 via-amber-300/20 to-transparent";
        if (stack.Status === 1) return "from-cyan-500/20 via-transparent to-transparent";
        return "from-slate-400/10 via-transparent to-transparent";
    }

    function openComposeEditor(project: LocalComposeProject) {
        onNavigate("compose-project-detail", {
            projectKey: project.projectKey || project.projectName
        });
    }
</script>

<div class="space-y-6">

    <!-- ── Page Header ────────────────────────────────────────────── -->
    <div class="flex flex-wrap items-center justify-between gap-4 opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">Stacks</h2>
            <p class="text-xs text-slate-500 font-medium">Compose projects, Portainer stacks, and GitOps repositories.</p>
        </div>
        <div class="flex items-center gap-2">
            <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-[10px] font-black text-slate-600 dark:text-slate-400 uppercase tracking-widest border border-slate-200 dark:border-slate-700">
                {composeProjects.length} Compose
            </span>
            {#if configStore.portainerActive}
                <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-[10px] font-black text-slate-600 dark:text-slate-400 uppercase tracking-widest border border-slate-200 dark:border-slate-700">
                    {stacks.length} Portainer
                </span>
            {/if}
            <button
                onclick={loadStacks}
                class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 transition-colors hover:border-brand-400 hover:text-brand-600 dark:hover:text-brand-300"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Refresh
            </button>
        </div>
    </div>

    <!-- ── Source Tab Bar ─────────────────────────────────────────── -->
    <div class="flex flex-wrap items-center gap-2 opacity-0 animate-reveal stagger-1">
        <!-- Local Compose tab -->
        <button
            type="button"
            onclick={() => activeSourceTab = "compose"}
            class="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border text-[10px] font-black uppercase tracking-widest transition-colors {activeSourceTab === 'compose' ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900/40 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:border-brand-300'}"
        >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
            Local Compose
            <span class="opacity-75">{composeProjects.length}</span>
        </button>

        <!-- Portainer tab (conditional) -->
        {#if configStore.portainerActive}
            <button
                type="button"
                onclick={() => activeSourceTab = "portainer"}
                class="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border text-[10px] font-black uppercase tracking-widest transition-colors {activeSourceTab === 'portainer' ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900/40 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:border-brand-300'}"
            >
                <PortainerLogo class="h-3.5 w-3.5" />
                Portainer
                <span class="opacity-75">{stacks.length}</span>
            </button>
        {/if}

        <!-- GitOps tab -->
        <button
            type="button"
            onclick={() => activeSourceTab = "repositories"}
            class="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border text-[10px] font-black uppercase tracking-widest transition-colors {activeSourceTab === 'repositories' ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900/40 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:border-brand-300'}"
        >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
            </svg>
            GitOps
        </button>
    </div>

    <!-- ── Loading ────────────────────────────────────────────────── -->
    {#if loading}
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950/80 px-6 py-14 text-center">
            <div class="mx-auto h-10 w-10 rounded-full border-4 border-brand-500 border-t-transparent animate-spin"></div>
            <p class="mt-4 text-[10px] font-black uppercase tracking-[0.28em] text-slate-400">Discovering stacks</p>
        </div>
    {:else}

        <!-- ── Local Compose Tab ───────────────────────────────────── -->
        {#if activeSourceTab === "compose"}
            <section class="space-y-4 opacity-0 animate-reveal stagger-2">
                {#if composeError}
                    <div class="rounded-2xl border border-rose-200 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-950/30 px-5 py-4 text-sm text-rose-700 dark:text-rose-300">
                        {composeError}
                    </div>
                {:else if composeProjects.length === 0}
                    <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-800 bg-white/75 dark:bg-slate-950/50 px-8 py-14 text-center">
                        <svg xmlns="http://www.w3.org/2000/svg" class="mx-auto h-10 w-10 text-slate-300 dark:text-slate-700" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                        </svg>
                        <p class="mt-3 text-sm font-medium text-slate-500 dark:text-slate-400">No local Docker Compose projects discovered</p>
                        <p class="mt-1 text-xs text-slate-400 dark:text-slate-500">Projects are discovered from running containers with Docker Compose labels.</p>
                    </div>
                {:else}
                    {#if composeTotalPages > 1}
                        <PaginationBar
                            summaryText={`Showing ${composePageStart}–${composePageEnd} of ${composeProjects.length}`}
                            pageText={`Page ${composePageIndex + 1} / ${composeTotalPages}`}
                            canPrev={composePageIndex > 0}
                            canNext={composePageIndex < composeTotalPages - 1}
                            onPrev={() => composePageIndex = Math.max(0, composePageIndex - 1)}
                            onNext={() => composePageIndex = Math.min(composeTotalPages - 1, composePageIndex + 1)}
                        />
                    {/if}

                    <div class="grid grid-cols-1 gap-5 xl:grid-cols-2">
                        {#each pagedComposeProjects as p, i}
                            <article
                                style="animation-delay: {0.06 + i * 0.04}s"
                                class="opacity-0 animate-reveal group relative flex flex-col rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 shadow-sm transition-all hover:-translate-y-0.5 hover:border-brand-400 hover:shadow-md dark:hover:border-brand-500"
                            >
                                <!-- update flag accent bar -->
                                {#if p.updateCandidates && p.updateCandidates > 0}
                                    <div class="h-0.5 w-full rounded-t-2xl bg-gradient-to-r from-amber-400 via-amber-300 to-transparent"></div>
                                {/if}

                                <div class="flex flex-1 flex-col gap-4 p-5">
                                    <!-- Header: name + update badge -->
                                    <div class="flex items-start justify-between gap-4">
                                        <div class="min-w-0">
                                            <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">Compose Project</p>
                                            <h3 class="mt-1 truncate text-lg font-black tracking-tight text-slate-900 dark:text-white">
                                                <button
                                                    class="text-left transition-colors hover:text-brand-600 dark:hover:text-brand-300"
                                                    onclick={() => openComposeEditor(p)}
                                                    title={p.projectName}
                                                >
                                                    {p.projectName}
                                                </button>
                                            </h3>
                                            <p class="mt-0.5 truncate text-[10px] font-mono text-slate-500 dark:text-slate-400" title={p.workingDir || (p.configFiles && p.configFiles[0]) || ""}>
                                                {p.workingDir || (p.configFiles && p.configFiles[0]) || "Path unavailable"}
                                            </p>
                                        </div>
                                        <div class="shrink-0">
                                            {#if p.updateCandidates && p.updateCandidates > 0}
                                                <span class="rounded-full bg-amber-100 px-2.5 py-1 text-[10px] font-black uppercase tracking-widest text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                                                    {p.updateCandidates} Update{p.updateCandidates > 1 ? "s" : ""}
                                                </span>
                                            {:else}
                                                <span class="rounded-full bg-emerald-100 px-2.5 py-1 text-[10px] font-black uppercase tracking-widest text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                                                    Up to date
                                                </span>
                                            {/if}
                                        </div>
                                    </div>

                                    <!-- Capability pills -->
                                    <div class="flex flex-wrap gap-1.5">
                                        <span class="rounded-full px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest {composeSourceStatusClass(p.sourceStatus)}">
                                            {composeSourceStatusLabel(p.sourceStatus)}
                                        </span>
                                        <span class="rounded-full px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest {composeAuthorityClass(p)}">
                                            {composeAuthorityLabel(p)}
                                        </span>
                                        <span class="rounded-full px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest {envAuthorityClass(p)}">
                                            {envAuthorityLabel(p)}
                                        </span>
                                        <span class="rounded-full px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest {snapshotStatusClass(p.snapshotStatus)}">
                                            {snapshotStatusLabel(p.snapshotStatus)}
                                        </span>
                                    </div>

                                    <!-- Member / service list -->
                                    {#if p.members && p.members.length > 0}
                                        <div class="space-y-1.5">
                                            <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">
                                                Services ({p.containerCount || p.members.length})
                                            </p>
                                            {#each p.members as m}
                                                <button
                                                    onclick={(e) => { e.stopPropagation(); onNavigate("container-detail", { id: m.containerId, tab: "lifecycle" }); }}
                                                    class="group/svc flex w-full items-center justify-between gap-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 px-3 py-2.5 text-left transition-all hover:border-brand-400 hover:bg-white dark:hover:bg-slate-800/50"
                                                >
                                                    <div class="flex min-w-0 items-center gap-2.5">
                                                        <span class="h-2 w-2 shrink-0 rounded-full {m.state === 'running' ? 'bg-emerald-500' : 'bg-slate-400'}"></span>
                                                        <div class="min-w-0">
                                                            <p class="truncate text-xs font-bold text-slate-800 dark:text-slate-100">{m.serviceName || m.containerName || m.containerId}</p>
                                                            <p class="truncate text-[9px] font-mono text-slate-500 dark:text-slate-400">{m.image || "Unknown image"}</p>
                                                        </div>
                                                    </div>
                                                    <div class="flex shrink-0 items-center gap-2">
                                                        {#if m.updateAvailable}
                                                            <span class="h-2 w-2 rounded-full bg-brand-500" title="Update available"></span>
                                                        {/if}
                                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 text-slate-300 transition-colors group-hover/svc:text-brand-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                                                        </svg>
                                                    </div>
                                                </button>
                                            {/each}
                                        </div>
                                    {/if}

                                    <!-- Footer: actions -->
                                    <div class="mt-auto flex items-center justify-between border-t border-slate-100 dark:border-slate-800 pt-4">
                                        <span class="text-[10px] font-black uppercase tracking-widest text-slate-400">
                                            {p.containerCount || 0} container{(p.containerCount || 0) !== 1 ? "s" : ""}
                                        </span>
                                        <button
                                            onclick={() => openComposeEditor(p)}
                                            class="inline-flex items-center gap-1.5 rounded-xl bg-brand-600 px-4 py-2 text-[10px] font-black uppercase tracking-widest text-white shadow-sm shadow-brand-500/20 transition-all hover:bg-brand-700"
                                        >
                                            Open Editor
                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M9 5l7 7-7 7" />
                                            </svg>
                                        </button>
                                    </div>
                                </div>
                            </article>
                        {/each}
                    </div>
                {/if}
            </section>
        {/if}

        <!-- ── Portainer Tab ───────────────────────────────────────── -->
        {#if configStore.portainerActive && activeSourceTab === "portainer"}
            <section class="space-y-4 opacity-0 animate-reveal stagger-2">
                {#if portainerError}
                    <div class="rounded-2xl border border-rose-200 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-950/30 px-5 py-4 text-sm text-rose-700 dark:text-rose-300">
                        {portainerError}
                    </div>
                {:else if stacks.length === 0}
                    <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-800 bg-white/75 dark:bg-slate-950/50 px-8 py-14 text-center">
                        <PortainerLogo class="mx-auto h-10 w-10 opacity-20" />
                        <p class="mt-3 text-sm font-medium text-slate-500 dark:text-slate-400">No stacks discovered</p>
                        <p class="mt-1 text-xs text-slate-400 dark:text-slate-500">No stacks were found in the configured Portainer endpoint.</p>
                    </div>
                {:else}
                    {#if totalPages > 1}
                        <PaginationBar
                            summaryText={`Showing ${pageStart}–${pageEnd} of ${stacks.length}`}
                            pageText={`Page ${pageIndex + 1} / ${totalPages}`}
                            canPrev={pageIndex > 0}
                            canNext={pageIndex < totalPages - 1}
                            onPrev={() => pageIndex = Math.max(0, pageIndex - 1)}
                            onNext={() => pageIndex = Math.min(totalPages - 1, pageIndex + 1)}
                        />
                    {/if}

                    <div class="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3">
                        {#each pagedStacks as s, i}
                            <article
                                style="animation-delay: {0.06 + i * 0.04}s"
                                class="opacity-0 animate-reveal group relative overflow-hidden rounded-2xl border shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-md
                                    {redeploying[s.Id]
                                        ? 'border-amber-400 dark:border-amber-500 ring-2 ring-amber-400/30 dark:ring-amber-500/25'
                                        : 'border-slate-200 dark:border-slate-700 hover:border-brand-400 dark:hover:border-brand-500'}"
                            >
                                <!-- TV static noise during redeploy -->
                                <StaticNoise active={redeploying[s.Id]} />

                                <!-- Top accent bar -->
                                <div class="relative h-1 bg-gradient-to-r {portainerCardAccent(s)}"></div>

                                <div class="relative space-y-4 bg-white dark:bg-slate-900 p-5">
                                    <!-- Header: name + badges -->
                                    <div class="flex items-start justify-between gap-3">
                                        <div class="min-w-0">
                                            <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">Portainer Stack</p>
                                            <h3 class="mt-1 truncate text-lg font-black tracking-tight text-slate-900 dark:text-white" title={s.Name}>
                                                {s.Name}
                                            </h3>
                                            <p class="mt-0.5 text-[10px] font-mono text-slate-500 dark:text-slate-400">
                                                ID {s.Id} · EP-{s.EndpointId}
                                            </p>
                                        </div>
                                        <div class="flex shrink-0 flex-col items-end gap-1.5">
                                            <span class="rounded-full px-2.5 py-1 text-[10px] font-black uppercase tracking-widest {stackStatusClass(s.Status, redeploying[s.Id])}">
                                                {redeploying[s.Id] ? "Redeploying" : stackStatusLabel(s.Status)}
                                            </span>
                                            <span class="rounded-full bg-cyan-100 px-2.5 py-1 text-[10px] font-black uppercase tracking-widest text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300">
                                                {stackTypeLabel(s.Type)}
                                            </span>
                                        </div>
                                    </div>

                                    <!-- State row -->
                                    <div class="flex items-center gap-2 rounded-xl border border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 px-3 py-2.5">
                                        <span class="h-2 w-2 shrink-0 rounded-full {redeploying[s.Id] ? 'bg-amber-500 animate-pulse' : s.Status === 1 ? 'bg-emerald-500' : 'bg-slate-400'}"></span>
                                        <p class="text-xs font-medium text-slate-600 dark:text-slate-300">
                                            {stackStateSummary(s, redeploying[s.Id])}
                                        </p>
                                    </div>

                                    <!-- Action buttons -->
                                    <div class="grid grid-cols-2 gap-2">
                                        <button
                                            onclick={() => onNavigate("containers", { search: s.Name })}
                                            class="inline-flex items-center justify-center gap-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/50 px-3 py-2 text-[10px] font-black uppercase tracking-widest text-slate-700 dark:text-slate-200 transition-all hover:border-brand-400 hover:text-brand-600 dark:hover:text-brand-300"
                                        >
                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                                            </svg>
                                            Containers
                                        </button>

                                        {#if s.Type === 2}
                                            <button
                                                onclick={() => redeployStack(s.Id)}
                                                disabled={redeploying[s.Id]}
                                                class="inline-flex items-center justify-center gap-1.5 rounded-xl bg-brand-600 px-3 py-2 text-[10px] font-black uppercase tracking-widest text-white shadow-sm shadow-brand-500/20 transition-all hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
                                            >
                                                {#if redeploying[s.Id]}
                                                    <span class="h-3 w-3 rounded-full border-2 border-white border-t-transparent animate-spin"></span>
                                                    Redeploying
                                                {:else}
                                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                                    </svg>
                                                    Redeploy
                                                {/if}
                                            </button>
                                        {:else}
                                            <div class="inline-flex items-center justify-center rounded-xl border border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 px-3 py-2 text-[10px] font-black uppercase tracking-widest text-slate-400">
                                                Swarm only
                                            </div>
                                        {/if}
                                    </div>
                                </div>
                            </article>
                        {/each}
                    </div>
                {/if}
            </section>
        {/if}

        <!-- ── GitOps Tab ─────────────────────────────────────────── -->
        {#if activeSourceTab === "repositories"}
            <section class="opacity-0 animate-reveal stagger-2">
                <GitOps {onNavigate} embedded />
            </section>
        {/if}

    {/if}

</div>

<style>
</style>
