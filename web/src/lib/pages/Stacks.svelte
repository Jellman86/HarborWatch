<script lang="ts">
    import { onDestroy, onMount } from "svelte";
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
        if (busy) return "Redeploy in progress";
        if (stack.Status === 1) return "Healthy orchestration target";
        if (stack.Status === 2) return "Inactive stack";
        return "Status unavailable";
    }

    function composeCardTone(project: LocalComposeProject): string {
        if (project.updateCandidates && project.updateCandidates > 0) return "ring-amber-400/30 bg-amber-50/70 dark:bg-amber-950/20";
        if (project.sourceVerified) return "ring-emerald-400/20 bg-white/90 dark:bg-slate-800/90";
        return "ring-slate-200/80 bg-white/90 dark:bg-slate-800/90";
    }

    function composeCardStateTone(project: LocalComposeProject): string {
        if (project.updateCandidates && project.updateCandidates > 0) return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        if (project.composeEditable) return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        if (project.sourceVerified) return "bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function composeCardSummary(project: LocalComposeProject): string {
        const parts = [automationSummary(project)];
        parts.push(`${project.containerCount || 0} containers`);
        return parts.join(" | ");
    }

    function portainerCardTone(stack: any): string {
        if (redeploying[stack.Id]) return "ring-amber-400/40 bg-[linear-gradient(135deg,_rgba(146,64,14,0.24),_rgba(15,23,42,0.92))]";
        if (stack.Status === 1) return "ring-cyan-400/20 bg-white/90 dark:bg-slate-800/90";
        return "ring-slate-200/80 bg-white/90 dark:bg-slate-800/90";
    }

    function portainerCardAccent(stack: any): string {
        if (redeploying[stack.Id]) return "from-amber-400/25 via-transparent to-cyan-400/10";
        if (stack.Status === 1) return "from-cyan-500/10 via-transparent to-slate-50/0";
        return "from-slate-500/10 via-transparent to-slate-50/0";
    }

    function openComposeEditor(project: LocalComposeProject) {
        onNavigate("compose-project-detail", {
            projectKey: project.projectKey || project.projectName
        });
    }
</script>

<div class="relative overflow-hidden">
    <div class="absolute inset-x-0 top-0 h-64 bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.10),_transparent_38%),radial-gradient(circle_at_top_right,_rgba(16,185,129,0.08),_transparent_30%)] pointer-events-none"></div>
    <div class="absolute inset-x-0 bottom-0 h-40 bg-[linear-gradient(180deg,_transparent,_rgba(15,23,42,0.03))] dark:bg-[linear-gradient(180deg,_transparent,_rgba(2,6,23,0.28))] pointer-events-none"></div>

    <div class="relative mx-auto max-w-[96rem] px-4 py-5 md:px-8 md:py-8 space-y-6">
        <section class="rounded-[2rem] border border-slate-200/70 dark:border-slate-800/80 bg-white/90 dark:bg-slate-950/85 shadow-[0_24px_80px_-42px_rgba(15,23,42,0.55)] backdrop-blur-xl overflow-hidden opacity-0 animate-reveal">
            <div class="px-5 py-5 md:px-7 md:py-7 border-b border-slate-200/60 dark:border-slate-800/80 bg-[linear-gradient(180deg,_rgba(248,250,252,0.96),_rgba(255,255,255,0.72))] dark:bg-[linear-gradient(180deg,_rgba(15,23,42,0.96),_rgba(15,23,42,0.84))]">
                <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                    <div class="space-y-3">
                        <div class="inline-flex items-center gap-2 rounded-full border border-slate-200/70 dark:border-slate-700 bg-white/80 dark:bg-slate-900/70 px-3 py-1 text-[10px] font-black uppercase tracking-[0.24em] text-slate-500 dark:text-slate-300">
                            Orchestration Workspace
                        </div>
                        <div class="border-l-4 border-brand-600 pl-4">
                            <h2 class="text-3xl md:text-4xl font-black text-slate-900 dark:text-white tracking-tight">Stack Explorer</h2>
                            <p class="mt-2 max-w-3xl text-sm text-slate-600 dark:text-slate-400">
                                Portainer stacks, local Compose projects, and GitOps repositories in one consistent control surface.
                            </p>
                        </div>
                    </div>
                    <div class="flex flex-wrap items-center gap-3">
                        <button
                            onclick={loadStacks}
                            class="inline-flex items-center justify-center gap-2 rounded-2xl border border-slate-200/80 dark:border-slate-700 bg-white/80 dark:bg-slate-900/70 px-4 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-700 dark:text-slate-200 shadow-sm transition-all hover:border-brand-400 hover:text-brand-700 dark:hover:text-brand-300"
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                            </svg>
                            Refresh Sources
                        </button>
                    </div>
                </div>

                <div class="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 px-4 py-3">
                        <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">Compose Projects</p>
                        <p class="mt-1 text-lg font-black text-slate-900 dark:text-white">{composeProjects.length}</p>
                    </div>
                    <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 px-4 py-3">
                        <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">Portainer Stacks</p>
                        <p class="mt-1 text-lg font-black text-slate-900 dark:text-white">{stacks.length}</p>
                    </div>
                    <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 px-4 py-3">
                        <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">GitOps</p>
                        <p class="mt-1 text-lg font-black text-slate-900 dark:text-white">Repositories</p>
                    </div>
                </div>
            </div>

            <div class="px-4 pt-4 md:px-6 md:pt-6">
                <div class="grid grid-cols-1 gap-3 xl:grid-cols-3">
                    <button
                        onclick={() => activeSourceTab = "compose"}
                        class="group rounded-[1.5rem] border p-4 text-left transition-all {activeSourceTab === 'compose' ? 'border-brand-400 bg-brand-500/8 shadow-[0_18px_50px_-30px_rgba(37,99,235,0.8)]' : 'border-slate-200/70 dark:border-slate-800 bg-white/70 dark:bg-slate-900/50 hover:border-brand-300'}"
                    >
                        <div class="flex items-center justify-between gap-3">
                            <div>
                                <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">Compose Files</p>
                                <h3 class="mt-1 text-lg font-black text-slate-900 dark:text-white">Local Compose</h3>
                            </div>
                            <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">{composeProjects.length}</span>
                        </div>
                        <p class="mt-2 text-xs text-slate-500 dark:text-slate-400">Source-aware compose projects with member inspection and editor access.</p>
                    </button>

                    {#if configStore.portainerActive}
                        <button
                            onclick={() => activeSourceTab = "portainer"}
                            class="group rounded-[1.5rem] border p-4 text-left transition-all {activeSourceTab === 'portainer' ? 'border-brand-400 bg-brand-500/8 shadow-[0_18px_50px_-30px_rgba(37,99,235,0.8)]' : 'border-slate-200/70 dark:border-slate-800 bg-white/70 dark:bg-slate-900/50 hover:border-brand-300'}"
                        >
                            <div class="flex items-center justify-between gap-3">
                                <div>
                                    <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">Portainer Stacks</p>
                                    <h3 class="mt-1 text-lg font-black text-slate-900 dark:text-white">Remote Control</h3>
                                </div>
                                <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">{stacks.length}</span>
                            </div>
                            <p class="mt-2 text-xs text-slate-500 dark:text-slate-400">Stack discovery and redeploy with visible in-card job state.</p>
                        </button>
                    {/if}

                    <button
                        onclick={() => activeSourceTab = "repositories"}
                        class="group rounded-[1.5rem] border p-4 text-left transition-all {activeSourceTab === 'repositories' ? 'border-brand-400 bg-brand-500/8 shadow-[0_18px_50px_-30px_rgba(37,99,235,0.8)]' : 'border-slate-200/70 dark:border-slate-800 bg-white/70 dark:bg-slate-900/50 hover:border-brand-300'}"
                    >
                        <div class="flex items-center justify-between gap-3">
                            <div>
                                <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">GitOps</p>
                                <h3 class="mt-1 text-lg font-black text-slate-900 dark:text-white">Repositories</h3>
                            </div>
                            <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">Open</span>
                        </div>
                        <p class="mt-2 text-xs text-slate-500 dark:text-slate-400">Repository-backed deployment rules with stack-level orchestration.</p>
                    </button>
                </div>
            </div>
        </section>

        {#if loading}
            <div class="rounded-[1.75rem] border border-slate-200/70 dark:border-slate-800 bg-white/85 dark:bg-slate-950/80 px-6 py-16 text-center shadow-sm">
                <div class="mx-auto w-10 h-10 border-4 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
                <p class="mt-4 text-[10px] font-black uppercase tracking-[0.28em] text-slate-400">Discovering orchestration sources</p>
            </div>
        {:else}
            {#if activeSourceTab === "compose"}
                <section class="space-y-4 opacity-0 animate-reveal stagger-1">
                    <div class="flex flex-col gap-3 rounded-[1.5rem] border border-slate-200/70 dark:border-slate-800 bg-white/85 dark:bg-slate-950/75 px-5 py-4 md:flex-row md:items-end md:justify-between">
                        <div>
                            <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">Local Compose Projects</p>
                            <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">Discovered from running containers with Docker Compose labels (non-Portainer).</p>
                        </div>
                        <span class="inline-flex items-center rounded-full bg-slate-100 dark:bg-slate-800 px-3 py-1.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">
                            {composeProjects.length} Projects
                        </span>
                    </div>

                    {#if composeError}
                        <div class="rounded-[1.5rem] border border-rose-200 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-950/30 px-5 py-4 text-sm text-rose-700 dark:text-rose-300">
                            {composeError}
                        </div>
                    {:else if composeProjects.length === 0}
                        <div class="rounded-[1.75rem] border border-dashed border-slate-200/80 dark:border-slate-800 bg-white/75 dark:bg-slate-950/50 px-8 py-14 text-center">
                            <p class="text-sm text-slate-500 dark:text-slate-400">No local Docker Compose projects discovered from running containers.</p>
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
                        <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
                            {#each pagedComposeProjects as p, i}
                                <article
                                    style="animation-delay: {0.08 + (i * 0.04)}s"
                                    class="opacity-0 animate-reveal relative overflow-hidden rounded-[1.75rem] border ring-1 shadow-[0_22px_60px_-42px_rgba(15,23,42,0.55)] transition-all hover:-translate-y-0.5 hover:shadow-[0_28px_70px_-38px_rgba(15,23,42,0.72)] {composeCardTone(p)}"
                                >
                                    <div class="absolute inset-0 bg-[radial-gradient(circle_at_top_right,_rgba(59,130,246,0.08),_transparent_34%)] pointer-events-none"></div>
                                    <div class="relative space-y-5 p-5 md:p-6">
                                        <div class="flex items-start justify-between gap-4">
                                            <div class="min-w-0">
                                                <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">Compose Project</p>
                                                <h3 class="mt-1 text-xl font-black tracking-tight text-slate-900 dark:text-white truncate" title={p.projectName}>
                                                    <button class="text-left transition-colors hover:text-brand-600 dark:hover:text-brand-300" onclick={() => openComposeEditor(p)}>{p.projectName}</button>
                                                </h3>
                                                <p class="mt-1 text-[10px] font-mono text-slate-500 dark:text-slate-400 truncate" title={p.workingDir || (p.configFiles && p.configFiles[0]) || "Path unavailable"}>
                                                    {p.workingDir || (p.configFiles && p.configFiles[0]) || "Path unavailable"}
                                                </p>
                                            </div>
                                            <div class="flex flex-col items-end gap-2">
                                                <span class="rounded-full px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] {composeCardStateTone(p)}">
                                                    {p.updateCandidates && p.updateCandidates > 0 ? `${p.updateCandidates} Update${p.updateCandidates > 1 ? 's' : ''}` : 'Up to date'}
                                                </span>
                                                <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">
                                                    {composeProjects.length} total
                                                </span>
                                            </div>
                                        </div>

                                        <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Source</p>
                                                <p class="mt-1 inline-flex rounded-full px-2 py-1 text-[10px] font-black uppercase tracking-[0.18em] {composeSourceStatusClass(p.sourceStatus)}">{composeSourceStatusLabel(p.sourceStatus)}</p>
                                            </div>
                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Compose</p>
                                                <p class="mt-1 inline-flex rounded-full px-2 py-1 text-[10px] font-black uppercase tracking-[0.18em] {composeAuthorityClass(p)}">{composeAuthorityLabel(p)}</p>
                                            </div>
                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Env</p>
                                                <p class="mt-1 inline-flex rounded-full px-2 py-1 text-[10px] font-black uppercase tracking-[0.18em] {envAuthorityClass(p)}">{envAuthorityLabel(p)}</p>
                                            </div>
                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Snapshot</p>
                                                <p class="mt-1 inline-flex rounded-full px-2 py-1 text-[10px] font-black uppercase tracking-[0.18em] {snapshotStatusClass(p.snapshotStatus)}">{snapshotStatusLabel(p.snapshotStatus)}</p>
                                            </div>
                                        </div>

                                        <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/70 dark:bg-slate-950/35 px-4 py-4">
                                            <div class="flex items-center justify-between gap-3">
                                                <div>
                                                    <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Context</p>
                                                    <p class="mt-1 text-sm font-semibold text-slate-700 dark:text-slate-300">{composeCardSummary(p)}</p>
                                                </div>
                                                <div class="text-right">
                                                    <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Services</p>
                                                    <p class="mt-1 text-xl font-black text-slate-900 dark:text-white">{p.containerCount || 0}</p>
                                                </div>
                                            </div>
                                        </div>

                                        <div class="space-y-2">
                                            <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">Containers / Services</p>
                                            <div class="space-y-2">
                                                {#each p.members || [] as m}
                                                    <button
                                                        onclick={(event) => {
                                                            event.stopPropagation();
                                                            onNavigate('container-detail', { id: m.containerId, tab: 'lifecycle' });
                                                        }}
                                                        class="group/svc flex w-full items-center justify-between gap-4 rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/85 dark:bg-slate-950/50 px-4 py-3 text-left transition-all hover:border-brand-400 hover:shadow-sm"
                                                    >
                                                        <div class="flex min-w-0 items-center gap-3">
                                                            <div class="h-2.5 w-2.5 rounded-full {m.state === 'running' ? 'bg-emerald-500' : 'bg-slate-400'}"></div>
                                                            <div class="min-w-0">
                                                                <p class="truncate text-xs font-bold text-slate-800 dark:text-slate-100">{m.serviceName || m.containerName || m.containerId}</p>
                                                                <p class="truncate text-[9px] font-mono text-slate-500 dark:text-slate-400">{m.image || 'Unknown image'}</p>
                                                            </div>
                                                        </div>
                                                        <div class="flex shrink-0 items-center gap-3">
                                                            {#if m.updateAvailable}
                                                                <span class="h-2.5 w-2.5 rounded-full bg-brand-500" title="Update Available"></span>
                                                            {/if}
                                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-slate-300 transition-colors group-hover/svc:text-brand-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                                                            </svg>
                                                        </div>
                                                    </button>
                                                {/each}
                                            </div>
                                        </div>

                                        <div class="flex items-center justify-between gap-3 border-t border-slate-200/70 dark:border-slate-800 pt-4">
                                            <div class="min-w-0">
                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Primary Action</p>
                                                <p class="mt-1 text-[11px] text-slate-500 dark:text-slate-400">Open compose source and manage stack definition.</p>
                                            </div>
                                            <button
                                                onclick={(event) => {
                                                    event.stopPropagation();
                                                    openComposeEditor(p);
                                                }}
                                                class="inline-flex shrink-0 items-center justify-center rounded-2xl bg-brand-600 px-4 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-white shadow-lg shadow-brand-500/20 transition-all hover:bg-brand-700"
                                            >
                                                Open Editor
                                            </button>
                                        </div>
                                    </div>
                                </article>
                            {/each}
                        </div>
                    {/if}
                </section>
            {/if}

            {#if configStore.portainerActive && activeSourceTab === "portainer"}
                <section class="space-y-4 opacity-0 animate-reveal stagger-1">
                    <div class="flex flex-col gap-3 rounded-[1.5rem] border border-slate-200/70 dark:border-slate-800 bg-white/85 dark:bg-slate-950/75 px-5 py-4 md:flex-row md:items-end md:justify-between">
                        <div>
                            <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">Portainer Stacks</p>
                            <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">Remote orchestration discovery and redeploy via Portainer API.</p>
                        </div>
                        <span class="inline-flex items-center rounded-full bg-slate-100 dark:bg-slate-800 px-3 py-1.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">
                            {stacks.length} Stacks
                        </span>
                    </div>

                    {#if portainerError}
                        <div class="rounded-[1.5rem] border border-rose-200 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-950/30 px-5 py-4 text-sm text-rose-700 dark:text-rose-300">
                            {portainerError}
                        </div>
                    {:else if stacks.length === 0}
                        <div class="rounded-[1.75rem] border border-dashed border-slate-200/80 dark:border-slate-800 bg-white/75 dark:bg-slate-950/50 px-8 py-14 text-center">
                            <p class="text-sm text-slate-500 dark:text-slate-400">No stacks discovered in the configured Portainer endpoint.</p>
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
                        <div class="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3">
                            {#each pagedStacks as s, i}
                                <article
                                    style="animation-delay: {0.08 + (i * 0.04)}s"
                                    class="group relative overflow-hidden rounded-[1.75rem] border ring-1 shadow-[0_22px_60px_-42px_rgba(15,23,42,0.55)] transition-all hover:-translate-y-0.5 hover:shadow-[0_28px_70px_-38px_rgba(15,23,42,0.72)] {portainerCardTone(s)}"
                                >
                                    <div class="absolute inset-0 pointer-events-none {redeploying[s.Id] ? 'stack-noise animate-stack-glow' : ''}"></div>
                                    <div class="relative h-1 bg-gradient-to-r {portainerCardAccent(s)}"></div>
                                    <div class="relative space-y-5 p-5 md:p-6">
                                        <div class="flex items-start justify-between gap-4">
                                            <div class="min-w-0">
                                                <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">Portainer Stack</p>
                                                <h3 class="mt-1 truncate text-xl font-black tracking-tight text-slate-900 dark:text-white" title={s.Name}>
                                                    {s.Name}
                                                </h3>
                                                <p class="mt-1 truncate text-[10px] font-mono uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
                                                    ID: {s.Id} | EP-{s.EndpointId}
                                                </p>
                                            </div>
                                            <div class="flex flex-col items-end gap-2">
                                                <span class="rounded-full px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] {stackStatusClass(s.Status, redeploying[s.Id])}">
                                                    {redeploying[s.Id] ? 'Redeploying' : stackStatusLabel(s.Status)}
                                                </span>
                                                <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">
                                                    {stackTypeLabel(s.Type)}
                                                </span>
                                            </div>
                                        </div>

                                        <div class="grid grid-cols-2 gap-3">
                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/80 dark:bg-slate-950/40 px-3 py-3">
                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">State</p>
                                                <p class="mt-1 text-sm font-black text-slate-900 dark:text-white">{stackStateSummary(s, redeploying[s.Id])}</p>
                                            </div>
                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/80 dark:bg-slate-950/40 px-3 py-3">
                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Engine</p>
                                                <p class="mt-1 text-sm font-black text-slate-900 dark:text-white">{stackTypeLabel(s.Type)}</p>
                                            </div>
                                        </div>

                                        <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/35 px-4 py-4">
                                            <div class="flex items-center gap-2">
                                                <span class="h-2.5 w-2.5 rounded-full {redeploying[s.Id] ? 'bg-amber-500 animate-pulse' : s.Status === 1 ? 'bg-emerald-500' : 'bg-slate-400'}"></span>
                                                <p class="text-sm font-semibold text-slate-700 dark:text-slate-300">
                                                    {redeploying[s.Id] ? 'Stack card remains active while the background redeploy job is queued or running.' : s.Status === 1 ? 'Ready for operator actions.' : 'Stack is not currently active.'}
                                                </p>
                                            </div>
                                        </div>

                                        <div class="flex items-center gap-2">
                                            <span class="rounded-full px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] {stackStatusClass(s.Status, redeploying[s.Id])}">
                                                {redeploying[s.Id] ? 'Busy' : s.Status === 1 ? 'Active' : 'Inactive'}
                                            </span>
                                            <span class="rounded-full bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300 px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em]">
                                                Portainer API
                                            </span>
                                        </div>

                                        <div class="grid gap-2 sm:grid-cols-2">
                                            <button
                                                onclick={() => onNavigate('containers', { search: s.Name })}
                                                class="inline-flex items-center justify-center rounded-2xl border border-slate-200/80 dark:border-slate-800 bg-white/85 dark:bg-slate-950/50 px-4 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-700 dark:text-slate-200 transition-all hover:border-brand-400 hover:text-brand-700 dark:hover:text-brand-300"
                                            >
                                                View Containers
                                            </button>
                                            {#if s.Type === 2}
                                                <button
                                                    onclick={() => redeployStack(s.Id)}
                                                    disabled={redeploying[s.Id]}
                                                    class="inline-flex items-center justify-center gap-2 rounded-2xl bg-brand-600 px-4 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-white shadow-lg shadow-brand-500/20 transition-all hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-70"
                                                >
                                                    {#if redeploying[s.Id]}
                                                        <div class="h-3 w-3 rounded-full border-2 border-white border-t-transparent animate-spin"></div>
                                                        Redeploying
                                                    {:else}
                                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                                        </svg>
                                                        Redeploy
                                                    {/if}
                                                </button>
                                            {:else}
                                                <div class="inline-flex items-center justify-center rounded-2xl border border-slate-200/80 dark:border-slate-800 bg-slate-50/80 dark:bg-slate-950/40 px-4 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">
                                                    Redeploy unavailable
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

            {#if activeSourceTab === "repositories"}
                <section class="space-y-4 opacity-0 animate-reveal stagger-1">
                    <GitOps {onNavigate} embedded />
                </section>
            {/if}
        {/if}
    </div>
</div>

<style>
    :global(.stack-noise) {
        background-image:
            linear-gradient(rgba(255, 255, 255, 0.06) 1px, transparent 1px),
            linear-gradient(90deg, rgba(255, 255, 255, 0.06) 1px, transparent 1px);
        background-size: 12px 12px;
        opacity: 0.55;
        mix-blend-mode: soft-light;
    }

    :global(.animate-stack-glow) {
        animation: stackGlow 1.8s ease-in-out infinite alternate;
    }

    @keyframes stackGlow {
        0% {
            filter: saturate(1) brightness(1);
        }
        100% {
            filter: saturate(1.08) brightness(1.03);
        }
    }
</style>
