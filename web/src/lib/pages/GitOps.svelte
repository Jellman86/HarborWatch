<script lang="ts">
    import { onDestroy, onMount } from "svelte";
    import { toasts } from "../stores/ToastStore";
    import { configStore } from "../stores/config.svelte";

    let { onNavigate, embedded = false } = $props<{
        onNavigate: (route: string, params?: any) => void;
        embedded?: boolean;
    }>();

    // Data types matching backend
    interface GitSource {
        id: string;
        name: string;
        url: string;
        branch: string;
        targetDir: string;
        authMethod: 'none' | 'http_token' | 'ssh_key';
        syncIntervalMins: number;
        lastCommitHash?: string;
        lastSyncError?: string;
        lastSyncAt: number;
        createdAt: number;
    }

    interface GitDeployment {
        id: string;
        gitSourceId: string;
        composePath: string;
        envVarsJson?: string;
        envFilePath?: string;
        envInlineContent?: string;
        envInlineEnabled?: boolean;
        pullOnDeploy?: boolean;
        autoCreated?: boolean;
        enabled?: boolean;
        lastDeployedHash?: string;
        lastDeployedAt: number;
        lastError?: string;
        lastJobId?: string;
        deployStatus?: string;
        deployStatusMessage?: string;
        deployStartedAt?: number;
        deployFinishedAt?: number;
        deployOutputSummary?: string;
    }

    interface GitSourceFiles {
        composeFiles: string[];
        envFiles: string[];
    }

    interface DeploymentFormState {
        composePath: string;
        envFilePath: string;
        enabled: boolean;
        envInlineEnabled: boolean;
        envInlineContent: string;
        pullOnDeploy: boolean;
    }

    // Component State
    let sources = $state<GitSource[]>([]);
    let deployments = $state<Record<string, GitDeployment[]>>({});
    let sourceFiles = $state<Record<string, GitSourceFiles>>({});
    let loadingSourceFiles = $state<Record<string, boolean>>({});
    let loading = $state(true);
    let syncing = $state<Record<string, boolean>>({});
    let deploying = $state<Record<string, boolean>>({});
    let showAddSourceModal = $state(false);
    let showAddDeploymentModal = $state(false);
    let showEditDeploymentModal = $state(false);
    let selectedSourceId = $state<string | null>(null);
    let editingSourceId = $state<string | null>(null);
    let editingDeploymentId = $state<string | null>(null);
    let expandedSourceId = $state<string | null>(null);
    let deploymentRefreshTimer: ReturnType<typeof setInterval> | null = null;

    // Form State
    let newSource = $state({
        name: "",
        url: "",
        branch: "main",
        targetDir: "",
        authMethod: "none" as 'none' | 'http_token' | 'ssh_key',
        authSecret: "",
        syncIntervalMins: 5
    });

    let newDeployment = $state<DeploymentFormState>({
        composePath: "docker-compose.yml",
        envFilePath: "",
        enabled: true,
        envInlineEnabled: false,
        envInlineContent: "",
        pullOnDeploy: false
    });

    let editDeployment = $state<DeploymentFormState>({
        composePath: "",
        envFilePath: "",
        enabled: true,
        envInlineEnabled: false,
        envInlineContent: "",
        pullOnDeploy: false
    });

    onMount(() => {
        loadSources();
        deploymentRefreshTimer = setInterval(() => {
            if (!hasActiveDeployments()) return;
            for (const src of sources) {
                void loadDeployments(src.id);
            }
        }, 5000);
    });

    onDestroy(() => {
        if (deploymentRefreshTimer) clearInterval(deploymentRefreshTimer);
    });

    function asArray<T>(value: unknown): T[] {
        return Array.isArray(value) ? (value as T[]) : [];
    }

    function resetNewDeployment() {
        newDeployment = {
            composePath: "docker-compose.yml",
            envFilePath: "",
            enabled: true,
            envInlineEnabled: false,
            envInlineContent: "",
            pullOnDeploy: false
        };
    }

    async function loadSourceFilesForPicker(sourceId: string, force = false) {
        if (!sourceId) return;
        if (loadingSourceFiles[sourceId]) return;
        if (!force && sourceFiles[sourceId]) return;

        loadingSourceFiles[sourceId] = true;
        try {
            const res = await fetch(`/api/gitops/sources/${sourceId}/files`);
            if (!res.ok) {
                throw new Error("request_failed");
            }
            const payload = await res.json();
            sourceFiles[sourceId] = {
                composeFiles: asArray<string>(payload?.composeFiles),
                envFiles: asArray<string>(payload?.envFiles)
            };
        } catch (e) {
            toasts.error("Unable to load compose/env files. Sync and try again.");
        } finally {
            loadingSourceFiles[sourceId] = false;
        }
    }

    function openAddDeploymentModal(sourceId: string) {
        selectedSourceId = sourceId;
        resetNewDeployment();
        showAddDeploymentModal = true;
        loadSourceFilesForPicker(sourceId);
    }

    function openEditDeploymentModal(sourceId: string, dep: GitDeployment) {
        editingSourceId = sourceId;
        editingDeploymentId = dep.id;
        editDeployment = {
            composePath: dep.composePath,
            envFilePath: dep.envFilePath || "",
            enabled: dep.enabled !== false,
            envInlineEnabled: dep.envInlineEnabled === true,
            envInlineContent: dep.envInlineContent || "",
            pullOnDeploy: dep.pullOnDeploy === true
        };
        showEditDeploymentModal = true;
        loadSourceFilesForPicker(sourceId);
    }

    async function loadSources() {
        loading = true;
        try {
            const res = await fetch("/api/gitops/sources");
            if (res.ok) {
                sources = asArray<GitSource>(await res.json());
                // Load deployments for all sources
                for (const src of sources) {
                    loadDeployments(src.id);
                }
            } else {
                toasts.error("Failed to load git sources");
            }
        } catch (e) {
            toasts.error("Connection error while loading sources");
        } finally {
            loading = false;
        }
    }

    async function loadDeployments(sourceId: string) {
        try {
            const res = await fetch(`/api/gitops/deployments?sourceId=${sourceId}`);
            if (res.ok) {
                deployments[sourceId] = asArray<GitDeployment>(await res.json());
            }
        } catch (e) {
            console.error(`Failed to load deployments for ${sourceId}`, e);
        }
    }

    async function addSource() {
        try {
            const res = await fetch("/api/gitops/sources", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(newSource)
            });
            if (res.ok) {
                toasts.success("Git source added successfully");
                showAddSourceModal = false;
                loadSources();
                // Reset form
                newSource = {
                    name: "",
                    url: "",
                    branch: "main",
                    targetDir: "",
                    authMethod: "none",
                    authSecret: "",
                    syncIntervalMins: 5
                };
            } else {
                const data = await res.json();
                toasts.error(data.message || "Failed to add git source");
            }
        } catch (e) {
            toasts.error("Connection error while adding source");
        }
    }

    async function addDeployment() {
        if (!selectedSourceId) return;

        try {
            const res = await fetch("/api/gitops/deployments", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    gitSourceId: selectedSourceId,
                    composePath: newDeployment.composePath,
                    envFilePath: newDeployment.envFilePath,
                    enabled: newDeployment.enabled,
                    envVarsJson: "",
                    envInlineEnabled: newDeployment.envInlineEnabled,
                    envInlineContent: newDeployment.envInlineContent,
                    pullOnDeploy: newDeployment.pullOnDeploy
                })
            });
            if (res.ok) {
                toasts.success("Deployment rule added");
                showAddDeploymentModal = false;
                loadDeployments(selectedSourceId);
                resetNewDeployment();
            } else {
                const data = await res.json();
                toasts.error(data.message || "Failed to add deployment");
            }
        } catch (e) {
            toasts.error("Connection error");
        }
    }

    async function saveDeploymentEdits() {
        if (!editingDeploymentId || !editingSourceId) return;
        const sourceId = editingSourceId;
        try {
            const res = await fetch(`/api/gitops/deployments/${editingDeploymentId}`, {
                method: "PATCH",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    composePath: editDeployment.composePath,
                    envFilePath: editDeployment.envFilePath,
                    enabled: editDeployment.enabled,
                    envVarsJson: "",
                    envInlineEnabled: editDeployment.envInlineEnabled,
                    envInlineContent: editDeployment.envInlineContent,
                    pullOnDeploy: editDeployment.pullOnDeploy
                })
            });
            if (res.ok) {
                toasts.success("Deployment rule updated");
                showEditDeploymentModal = false;
                editingDeploymentId = null;
                editingSourceId = null;
                loadDeployments(sourceId);
            } else {
                const data = await res.json().catch(() => ({}));
                toasts.error(data.message || "Failed to update deployment rule");
            }
        } catch (e) {
            toasts.error("Connection error");
        }
    }

    async function deleteDeployment(id: string, sourceId: string) {
        if (!confirm("Remove this deployment rule?")) return;
        try {
            const res = await fetch(`/api/gitops/deployments/${id}`, { method: "DELETE" });
            if (res.ok) {
                toasts.success("Deployment removed");
                loadDeployments(sourceId);
            }
        } catch (e) {
            toasts.error("Connection error");
        }
    }

    async function toggleDeployment(id: string, sourceId: string, enabled: boolean) {
        try {
            const res = await fetch(`/api/gitops/deployments/${id}/enabled`, {
                method: "PATCH",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ enabled })
            });
            if (res.ok) {
                toasts.success(enabled ? "Deployment enabled" : "Deployment disabled");
                loadDeployments(sourceId);
            } else {
                const data = await res.json().catch(() => ({}));
                toasts.error(data.message || "Failed to update deployment status");
            }
        } catch (e) {
            toasts.error("Connection error");
        }
    }

    async function deployNow(id: string) {
        if (deploying[id]) return;
        deploying[id] = true;
        try {
            const res = await fetch(`/api/gitops/deployments/${id}/deploy`, { method: "POST" });
            const data = await res.json().catch(() => ({}));
            if (res.ok) {
                if (data.duplicate) {
                    toasts.info("Deployment already queued or running");
                } else {
                    toasts.success("Deployment queued");
                }
                const sourceID = sources.find(s => (deployments[s.id] || []).some(d => d.id === id))?.id;
                if (sourceID) void loadDeployments(sourceID);
            } else {
                toasts.error(data.message || "Deployment failed");
            }
        } catch (e) {
            toasts.error("Connection error during deployment");
        } finally {
            deploying[id] = false;
        }
    }

    async function deleteSource(id: string) {
        if (!confirm("Are you sure you want to delete this repository? This will also delete all its deployment rules.")) return;
        try {
            const res = await fetch(`/api/gitops/sources/${id}`, { method: "DELETE" });
            if (res.ok) {
                toasts.success("Repository removed");
                loadSources();
            } else {
                toasts.error("Failed to remove repository");
            }
        } catch (e) {
            toasts.error("Connection error");
        }
    }

    async function syncSource(id: string) {
        if (syncing[id]) return;
        syncing[id] = true;
        toasts.info("Starting synchronization...");
        try {
            const res = await fetch(`/api/gitops/sources/${id}/sync`, { method: "POST" });
            const data = await res.json();
            if (res.ok) {
                if (data.changed) {
                    toasts.success(`Source updated to ${data.hash?.slice(0, 7)}`);
                } else {
                    toasts.info("Repository is already up to date");
                }
                if (Number(data.autoCreated || 0) > 0) {
                    const count = Number(data.autoCreated);
                    toasts.info(`Discovered ${count} compose file${count === 1 ? "" : "s"}. Rules were added disabled.`);
                }
                loadSourceFilesForPicker(id, true);
                loadSources();
            } else {
                toasts.error(data.message || "Sync failed");
            }
        } catch (e) {
            toasts.error("Connection error during sync");
        } finally {
            syncing[id] = false;
        }
    }

    function toggleExpand(id: string) {
        expandedSourceId = expandedSourceId === id ? null : id;
    }

    function formatRelativeTime(ts: number) {
        if (!ts) return "Never";
        const diff = Math.floor(Date.now() / 1000) - ts;
        if (diff < 60) return "Just now";
        if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
        return new Date(ts * 1000).toLocaleDateString();
    }

    function resolvedComposeMapping(targetDir: string, composePath: string): string {
        const baseDir = String(configStore.gitOpsMasterDirectory || "").replace(/\/+$/, "");
        const normalizedTargetDir = String(targetDir || "").replace(/^\/+|\/+$/g, "");
        const normalizedComposePath = String(composePath || "").replace(/^\/+/, "");
        return [baseDir, normalizedTargetDir, normalizedComposePath].filter(Boolean).join("/");
    }

    function activeEnvSourceLabel(dep: GitDeployment): string {
        if (dep.envInlineEnabled) return "HarborWatch Override";
        if (dep.envFilePath) return "Custom Env File";
        if (dep.envVarsJson) return "Legacy Overlay";
        return "Repo / Default Env";
    }

    function activeEnvSourceClass(dep: GitDeployment): string {
        if (dep.envInlineEnabled) return "bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300";
        if (dep.envFilePath) return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        if (dep.envVarsJson) return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function activeEnvSourceSummary(dep: GitDeployment): string {
        if (dep.envInlineEnabled) return "Deploy ignores repo/default env files and uses HarborWatch-managed raw .env content.";
        if (dep.envFilePath) return "Deploy uses the configured env file path.";
        if (dep.envVarsJson) return "Deploy keeps the legacy HarborWatch env overlay on top of the base env source.";
        return "Deploy relies on Docker Compose default env resolution next to the compose file.";
    }

    function deployImagePolicyLabel(dep: GitDeployment): string {
        return dep.pullOnDeploy ? "Pull Before Deploy" : "Use Local Images";
    }

    function deployImagePolicyClass(dep: GitDeployment): string {
        return dep.pullOnDeploy
            ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
            : "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function deployImagePolicySummary(dep: GitDeployment): string {
        return dep.pullOnDeploy
            ? "HarborWatch pulls newer images before applying this stack."
            : "HarborWatch applies this stack using currently available local images.";
    }

    function deploymentStateLabel(dep: GitDeployment): string {
        if (dep.deployStatus === "queued") return "Queued";
        if (dep.deployStatus === "running") return "Deploying";
        if (dep.deployStatus === "completed") return "Completed";
        if (dep.deployStatus === "failed") return "Failed";
        if (dep.enabled === false) return "Disabled";
        return "Ready";
    }

    function deploymentStateClass(dep: GitDeployment): string {
        if (dep.deployStatus === "queued") return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        if (dep.deployStatus === "running") return "bg-brand-100 text-brand-700 dark:bg-brand-900/30 dark:text-brand-300";
        if (dep.deployStatus === "completed") return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        if (dep.deployStatus === "failed") return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300";
        if (dep.enabled === false) return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
        return "bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300";
    }

    function deploymentStatusSummary(dep: GitDeployment): string {
        if (dep.deployStatusMessage) return dep.deployStatusMessage;
        if (dep.lastError) return dep.lastError;
        if (dep.lastDeployedAt) {
            return `Last successful deploy ${formatRelativeTime(dep.lastDeployedAt)}${dep.lastDeployedHash ? ` (${dep.lastDeployedHash.slice(0, 7)})` : ''}`;
        }
        return "Deployment has not run yet.";
    }

    function hasActiveDeployments(): boolean {
        return Object.values(deployments).some((items) => (items || []).some((dep) => dep.deployStatus === "queued" || dep.deployStatus === "running"));
    }

    function countDeployments(): number {
        return Object.values(deployments).reduce((total, items) => total + (items?.length || 0), 0);
    }

    function countBusyDeployments(): number {
        return Object.values(deployments).reduce((total, items) => {
            return total + (items || []).filter((dep) => dep.deployStatus === "queued" || dep.deployStatus === "running").length;
        }, 0);
    }

    function sourceBusy(source: GitSource): boolean {
        return syncing[source.id] === true;
    }

    function sourceCardTone(source: GitSource): string {
        if (sourceBusy(source)) return "ring-amber-400/40 bg-[linear-gradient(135deg,_rgba(161,98,7,0.14),_rgba(15,23,42,0.95))]";
        if (source.lastSyncError) return "ring-rose-400/30 bg-white/92 dark:bg-slate-900/92";
        return "ring-slate-200/80 bg-white/92 dark:bg-slate-900/90";
    }

    function sourceCardAccent(source: GitSource): string {
        if (sourceBusy(source)) return "from-amber-400/30 via-transparent to-cyan-400/10";
        if (source.lastSyncError) return "from-rose-400/30 via-transparent to-amber-400/10";
        if (source.lastCommitHash) return "from-cyan-500/12 via-transparent to-slate-50/0";
        return "from-slate-500/10 via-transparent to-slate-50/0";
    }

    function sourceStateChipClass(source: GitSource): string {
        if (sourceBusy(source)) return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        if (source.lastSyncError) return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300";
        return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
    }

    function sourceStateLabel(source: GitSource): string {
        if (sourceBusy(source)) return "Syncing";
        if (source.lastSyncError) return "Sync Issue";
        return "Ready";
    }

    function sourceStateSummary(source: GitSource): string {
        if (sourceBusy(source)) return "Refreshing repository contents and stack definitions.";
        if (source.lastSyncError) return source.lastSyncError;
        return `Branch ${source.branch} | ${source.syncIntervalMins} min sync`;
    }

    function deploymentBusy(dep: GitDeployment): boolean {
        return dep.deployStatus === "queued" || dep.deployStatus === "running" || deploying[dep.id] === true;
    }

    function deploymentCardTone(dep: GitDeployment): string {
        if (deploymentBusy(dep)) return "ring-amber-400/40 bg-[linear-gradient(135deg,_rgba(161,98,7,0.16),_rgba(15,23,42,0.95))]";
        if (dep.deployStatus === "failed") return "ring-rose-400/30 bg-white/92 dark:bg-slate-900/92";
        if (dep.enabled === false) return "ring-slate-200/80 bg-white/82 dark:bg-slate-900/82";
        return "ring-slate-200/80 bg-white/92 dark:bg-slate-900/90";
    }

    function deploymentCardAccent(dep: GitDeployment): string {
        if (deploymentBusy(dep)) return "from-amber-400/35 via-transparent to-cyan-400/10";
        if (dep.deployStatus === "failed") return "from-rose-400/30 via-transparent to-slate-50/0";
        if (dep.deployStatus === "completed") return "from-emerald-400/25 via-transparent to-cyan-400/10";
        return "from-slate-500/10 via-transparent to-slate-50/0";
    }

    function deploymentStateTone(dep: GitDeployment): string {
        if (deploymentBusy(dep)) return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        if (dep.deployStatus === "failed") return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300";
        if (dep.deployStatus === "completed") return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        if (dep.enabled === false) return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
        return "bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300";
    }

    function deploymentNoiseClass(dep: GitDeployment): string {
        return deploymentBusy(dep) ? "deployment-noise animate-deployment-pulse" : "";
    }
</script>

<div class={embedded ? "space-y-6" : "relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.10),_transparent_34%),radial-gradient(circle_at_top_right,_rgba(16,185,129,0.09),_transparent_28%),linear-gradient(180deg,_rgba(248,250,252,1),_rgba(241,245,249,0.72))] dark:bg-[radial-gradient(circle_at_top_left,_rgba(37,99,235,0.14),_transparent_34%),radial-gradient(circle_at_top_right,_rgba(16,185,129,0.12),_transparent_28%),linear-gradient(180deg,_rgba(2,6,23,1),_rgba(15,23,42,0.82))]"}>
    {#if !embedded}
        <div class="absolute inset-0 pointer-events-none opacity-40 bg-[linear-gradient(rgba(148,163,184,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(148,163,184,0.08)_1px,transparent_1px)] bg-[size:42px_42px]"></div>
        <div class="absolute inset-x-0 top-0 h-72 bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.10),_transparent_36%),radial-gradient(circle_at_top_right,_rgba(16,185,129,0.08),_transparent_28%)] pointer-events-none"></div>
        <div class="absolute inset-x-0 bottom-0 h-40 bg-[linear-gradient(180deg,_transparent,_rgba(15,23,42,0.03))] dark:bg-[linear-gradient(180deg,_transparent,_rgba(2,6,23,0.30))] pointer-events-none"></div>
    {/if}

    <div class={embedded ? "space-y-6" : "relative mx-auto max-w-[96rem] px-4 py-5 md:px-8 md:py-8 space-y-6"}>
        <section class="rounded-[2rem] border border-slate-200/70 dark:border-slate-800/80 bg-white/92 dark:bg-slate-950/88 shadow-[0_24px_80px_-42px_rgba(15,23,42,0.55)] backdrop-blur-xl overflow-hidden opacity-0 animate-reveal">
            <div class="px-5 py-5 md:px-7 md:py-7 border-b border-slate-200/60 dark:border-slate-800/80 bg-[linear-gradient(180deg,_rgba(248,250,252,0.98),_rgba(255,255,255,0.72))] dark:bg-[linear-gradient(180deg,_rgba(15,23,42,0.98),_rgba(15,23,42,0.84))]">
                <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                    <div class="space-y-3">
                        <div class="inline-flex items-center gap-2 rounded-full border border-slate-200/70 dark:border-slate-700 bg-white/80 dark:bg-slate-900/70 px-3 py-1 text-[10px] font-black uppercase tracking-[0.24em] text-slate-500 dark:text-slate-300">
                            Repository Orchestration
                        </div>
                        <div class="border-l-4 border-brand-600 pl-4">
                            <h2 class="flex items-center gap-3 text-3xl md:text-4xl font-black text-slate-900 dark:text-white tracking-tight">
                                <span class="inline-flex h-12 w-12 items-center justify-center rounded-2xl border border-brand-500/20 bg-brand-500/10 text-brand-600 dark:text-brand-300">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M7 7.5A2.5 2.5 0 019.5 5h5A2.5 2.5 0 0117 7.5v1A2.5 2.5 0 0114.5 11h-5A2.5 2.5 0 017 8.5v-1Z" />
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M6 17.5A2.5 2.5 0 018.5 15h7A2.5 2.5 0 0118 17.5v1A2.5 2.5 0 0115.5 21h-7A2.5 2.5 0 016 18.5v-1Z" />
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M9.5 11v4M14.5 11v4M12 11v4" />
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M9.5 12.5H6.5A2.5 2.5 0 004 15v.5M14.5 12.5h3A2.5 2.5 0 0120 15v.5" />
                                    </svg>
                                </span>
                                GitOps Repositories
                            </h2>
                            <p class="mt-2 max-w-3xl text-sm text-slate-600 dark:text-slate-400">
                                Manage repository-backed stacks with identity-first cards, explicit sync state, and nested deployment controls.
                            </p>
                        </div>
                    </div>
                    <div class="flex flex-wrap items-center gap-3">
                        <button
                            onclick={() => showAddSourceModal = true}
                            class="inline-flex items-center justify-center gap-2 rounded-2xl border border-slate-200/80 dark:border-slate-700 bg-white/80 dark:bg-slate-900/70 px-4 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-700 dark:text-slate-200 shadow-sm transition-all hover:border-brand-400 hover:text-brand-700 dark:hover:text-brand-300"
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M12 4v16m8-8H4" />
                            </svg>
                            Add Repository
                        </button>
                        <button
                            onclick={loadSources}
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
                        <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">Repositories</p>
                        <p class="mt-1 text-lg font-black text-slate-900 dark:text-white">{sources.length}</p>
                    </div>
                    <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 px-4 py-3">
                        <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">Deployment Rules</p>
                        <p class="mt-1 text-lg font-black text-slate-900 dark:text-white">{countDeployments()}</p>
                    </div>
                    <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 px-4 py-3">
                        <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">Active Jobs</p>
                        <p class="mt-1 text-lg font-black text-slate-900 dark:text-white">{countBusyDeployments()}</p>
                    </div>
                </div>
            </div>
        </section>

        {#if loading && sources.length === 0}
            <div class="rounded-[1.75rem] border border-slate-200/70 dark:border-slate-800 bg-white/85 dark:bg-slate-950/80 px-6 py-16 text-center shadow-sm">
                <div class="mx-auto w-10 h-10 border-4 border-brand-500 border-t-transparent rounded-full animate-spin"></div>
                <p class="mt-4 text-[10px] font-black uppercase tracking-[0.28em] text-slate-400">Discovering repositories</p>
            </div>
        {:else if sources.length === 0}
            <div class="rounded-[1.75rem] border border-dashed border-slate-200/80 dark:border-slate-800 bg-white/75 dark:bg-slate-950/60 px-8 py-14 text-center">
                <div class="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-slate-50 dark:bg-slate-900 text-slate-300 dark:text-slate-700">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M7 7.5A2.5 2.5 0 019.5 5h5A2.5 2.5 0 0117 7.5v1A2.5 2.5 0 0114.5 11h-5A2.5 2.5 0 017 8.5v-1Z" />
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M6 17.5A2.5 2.5 0 018.5 15h7A2.5 2.5 0 0118 17.5v1A2.5 2.5 0 0115.5 21h-7A2.5 2.5 0 016 18.5v-1Z" />
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M9.5 11v4M14.5 11v4M12 11v4" />
                    </svg>
                </div>
                <h3 class="text-xl font-black text-slate-900 dark:text-white uppercase tracking-tight">No Repositories Connected</h3>
                <p class="mt-2 max-w-lg mx-auto text-sm text-slate-500 dark:text-slate-400">Add a Git repository to start managing compose stacks with native GitOps workflows.</p>
                <button
                    onclick={() => showAddSourceModal = true}
                    class="mt-8 inline-flex items-center justify-center rounded-2xl bg-brand-600 px-8 py-3 text-[11px] font-black uppercase tracking-widest text-white transition-all hover:bg-brand-700 shadow-lg shadow-brand-500/20"
                >
                    Connect Your First Repository
                </button>
            </div>
        {:else}
            <div class="space-y-5 opacity-0 animate-reveal stagger-1">
                {#each sources as source, index}
                    <article
                        style="animation-delay: {0.08 + (index * 0.05)}s"
                        class="opacity-0 animate-reveal group relative overflow-hidden rounded-[1.75rem] border ring-1 shadow-[0_22px_60px_-42px_rgba(15,23,42,0.55)] transition-all hover:-translate-y-0.5 hover:shadow-[0_28px_70px_-38px_rgba(15,23,42,0.72)] {sourceCardTone(source)}"
                    >
                        <div class="absolute inset-0 pointer-events-none {sourceBusy(source) ? 'repo-noise animate-repo-pulse' : ''}"></div>
                        <div class="relative h-1 bg-gradient-to-r {sourceCardAccent(source)}"></div>
                        <div class="relative space-y-5 p-5 md:p-6">
                            <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                                <div class="min-w-0 flex-1">
                                    <div class="flex items-start gap-4">
                                        <button
                                            onclick={() => toggleExpand(source.id)}
                                            aria-label={expandedSourceId === source.id ? `Collapse repository ${source.name}` : `Expand repository ${source.name}`}
                                            class="mt-0.5 inline-flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-slate-200/70 dark:border-slate-700 bg-white/80 dark:bg-slate-900/70 transition-all hover:border-brand-400 hover:text-brand-600 dark:hover:text-brand-300 active:scale-95"
                                        >
                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-slate-400 transition-transform duration-300 {expandedSourceId === source.id ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                                            </svg>
                                        </button>
                                        <div class="min-w-0 space-y-2">
                                            <div class="flex flex-wrap items-center gap-2">
                                                <span class="rounded-full px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] {sourceStateChipClass(source)}">
                                                    {sourceStateLabel(source)}
                                                </span>
                                                <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">
                                                    Branch {source.branch}
                                                </span>
                                            </div>
                                            <h3 class="truncate text-2xl font-black tracking-tight text-slate-900 dark:text-white" title={source.name}>{source.name}</h3>
                                            <p class="truncate text-[10px] font-mono text-slate-500 dark:text-slate-400" title={source.url}>{source.url}</p>
                                            <div class="flex flex-wrap items-center gap-2 text-[10px] font-black uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">
                                                <span class="rounded-full bg-slate-100/90 dark:bg-slate-800 px-2.5 py-1">{source.authMethod === 'none' ? 'Public' : source.authMethod === 'http_token' ? 'Token Auth' : 'SSH Auth'}</span>
                                                <span class="rounded-full bg-slate-100/90 dark:bg-slate-800 px-2.5 py-1">{source.syncIntervalMins} min sync</span>
                                                <span class="rounded-full bg-slate-100/90 dark:bg-slate-800 px-2.5 py-1">Target {source.targetDir || '.'}</span>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <div class="flex flex-wrap items-center gap-2 xl:justify-end">
                                    <button
                                        onclick={() => openAddDeploymentModal(source.id)}
                                        class="inline-flex items-center justify-center rounded-2xl bg-brand-600 px-3 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-white shadow-lg shadow-brand-500/20 transition-all hover:bg-brand-700"
                                    >
                                        Add Stack
                                    </button>
                                    <button
                                        onclick={() => syncSource(source.id)}
                                        disabled={syncing[source.id]}
                                        class="inline-flex items-center justify-center rounded-2xl border border-slate-200/80 dark:border-slate-800 bg-white/85 dark:bg-slate-950/50 px-3 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-700 dark:text-slate-200 transition-all hover:border-brand-400 hover:text-brand-700 dark:hover:text-brand-300 disabled:opacity-50"
                                        title="Sync Repository"
                                    >
                                        {syncing[source.id] ? 'Syncing...' : 'Sync'}
                                    </button>
                                    <button
                                        onclick={() => toggleExpand(source.id)}
                                        class="inline-flex items-center justify-center rounded-2xl border border-slate-200/80 dark:border-slate-800 bg-white/85 dark:bg-slate-950/50 px-3 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-700 dark:text-slate-200 transition-all hover:border-brand-400 hover:text-brand-700 dark:hover:text-brand-300"
                                    >
                                        {expandedSourceId === source.id ? 'Collapse' : 'Expand'}
                                    </button>
                                    <button
                                        onclick={() => deleteSource(source.id)}
                                        class="inline-flex items-center justify-center rounded-2xl border border-slate-200/80 dark:border-slate-800 bg-white/85 dark:bg-slate-950/50 px-3 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-700 dark:text-slate-200 transition-all hover:border-rose-400 hover:text-rose-600 dark:hover:text-rose-300"
                                        title="Remove Repository"
                                    >
                                        Remove
                                    </button>
                                </div>
                            </div>

                            <div class="grid grid-cols-2 gap-3 xl:grid-cols-4">
                                <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                    <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Last Sync</p>
                                    <p class="mt-1 text-sm font-bold text-slate-700 dark:text-slate-200">{formatRelativeTime(source.lastSyncAt)}</p>
                                </div>
                                <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                    <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Commit</p>
                                    <p class="mt-1 text-sm font-mono font-bold text-slate-700 dark:text-slate-200">{source.lastCommitHash ? source.lastCommitHash.slice(0, 7) : "N/A"}</p>
                                </div>
                                <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                    <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Auth</p>
                                    <p class="mt-1 text-sm font-black uppercase tracking-tight text-slate-700 dark:text-slate-200">
                                        {source.authMethod === 'none' ? 'Public' : source.authMethod === 'http_token' ? 'Token' : 'SSH Key'}
                                    </p>
                                </div>
                                <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                    <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Deployment Rules</p>
                                    <p class="mt-1 text-sm font-black text-slate-900 dark:text-white">{(deployments[source.id] || []).length}</p>
                                </div>
                            </div>

                            <div class="grid gap-3 xl:grid-cols-[1.3fr_0.7fr]">
                                <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/35 px-4 py-4">
                                    <div class="flex items-start justify-between gap-4">
                                        <div class="min-w-0">
                                            <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Repository State</p>
                                            <p class="mt-1 text-sm font-semibold text-slate-700 dark:text-slate-300">{sourceStateSummary(source)}</p>
                                            <p class="mt-2 text-[10px] text-slate-500 dark:text-slate-400">
                                                Sync target: <span class="font-mono">{source.targetDir || '.'}</span>
                                            </p>
                                        </div>
                                        <div class="text-right">
                                            <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Sync Interval</p>
                                            <p class="mt-1 text-xl font-black text-slate-900 dark:text-white">{source.syncIntervalMins}m</p>
                                        </div>
                                    </div>
                                    {#if source.lastSyncError}
                                        <div class="mt-4 rounded-2xl border border-rose-200 dark:border-rose-900/40 bg-rose-50 dark:bg-rose-950/20 px-3 py-3 text-sm text-rose-700 dark:text-rose-300">
                                            {source.lastSyncError}
                                        </div>
                                    {/if}
                                </div>

                                <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/35 px-4 py-4">
                                    <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Top-Level Actions</p>
                                    <p class="mt-1 text-[11px] text-slate-500 dark:text-slate-400">Repository sync, stack creation, and lifecycle controls stay at the card edge.</p>
                                </div>
                            </div>

                            {#if expandedSourceId === source.id}
                                <div class="space-y-4 rounded-[1.5rem] border border-slate-200/70 dark:border-slate-800 bg-slate-50/80 dark:bg-slate-950/45 p-4 md:p-5">
                                    <div class="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
                                        <div>
                                            <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">Deployment Rules</p>
                                            <p class="mt-1 text-sm text-slate-600 dark:text-slate-400">Nested cards preserve identity and job state for each stack rule.</p>
                                        </div>
                                        <span class="inline-flex items-center rounded-full bg-slate-100 dark:bg-slate-800 px-3 py-1.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">
                                            {(deployments[source.id] || []).length} Stacks
                                        </span>
                                    </div>

                                    {#if (deployments[source.id] || []).length === 0}
                                        <div class="rounded-[1.5rem] border border-dashed border-slate-200/80 dark:border-slate-800 bg-white/75 dark:bg-slate-950/50 px-8 py-12 text-center">
                                            <p class="text-sm text-slate-500 font-medium italic">No compose files selected for deployment from this repository.</p>
                                            <button
                                                onclick={() => openAddDeploymentModal(source.id)}
                                                class="mt-4 text-[10px] font-black uppercase tracking-widest text-brand-600 dark:text-brand-400 hover:underline"
                                            >
                                                Configure first stack
                                            </button>
                                        </div>
                                    {:else}
                                        <div class="grid grid-cols-1 gap-3">
                                            {#each deployments[source.id] as dep}
                                                <article class="group/dep relative overflow-hidden rounded-[1.5rem] border ring-1 shadow-[0_18px_50px_-36px_rgba(15,23,42,0.45)] transition-all hover:-translate-y-0.5 hover:shadow-[0_22px_60px_-32px_rgba(15,23,42,0.58)] {deploymentCardTone(dep)}">
                                                    <div class="absolute inset-0 pointer-events-none {deploymentNoiseClass(dep)}"></div>
                                                    <div class="relative h-1 bg-gradient-to-r {deploymentCardAccent(dep)}"></div>
                                                    {#if deploymentBusy(dep)}
                                                        <div class="absolute inset-x-0 top-0 h-full bg-[linear-gradient(90deg,rgba(245,158,11,0.10),transparent_24%,transparent_76%,rgba(34,211,238,0.08))] pointer-events-none"></div>
                                                    {/if}
                                                    <div class="relative space-y-4 p-4 md:p-5">
                                                        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                                                            <div class="min-w-0 flex-1">
                                                                <div class="flex flex-wrap items-center gap-2">
                                                                    <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">Nested Stack</p>
                                                                    <span class="rounded-full px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] {deploymentStateTone(dep)}">
                                                                        {deploymentStateLabel(dep)}
                                                                    </span>
                                                                    {#if dep.autoCreated && dep.enabled === false}
                                                                        <span class="rounded-full bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300 px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em]">
                                                                            Newly Discovered
                                                                        </span>
                                                                    {/if}
                                                                    {#if dep.enabled === false}
                                                                        <span class="rounded-full bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em]">
                                                                            Disabled
                                                                        </span>
                                                                    {/if}
                                                                </div>
                                                                <h4 class="mt-2 truncate text-lg font-black tracking-tight text-slate-900 dark:text-white font-mono" title={dep.composePath}>{dep.composePath}</h4>
                                                                <p class="mt-1 text-[10px] font-mono text-slate-500 dark:text-slate-400 break-all">Mapped: {resolvedComposeMapping(source.targetDir, dep.composePath)}</p>
                                                                <p class="mt-1 text-[10px] text-slate-500 dark:text-slate-400">
                                                                    Last deployed: {formatRelativeTime(dep.lastDeployedAt)} {dep.lastDeployedHash ? `(${dep.lastDeployedHash.slice(0, 7)})` : ''}
                                                                </p>
                                                            </div>
                                                            <div class="flex flex-wrap items-center gap-2 lg:justify-end">
                                                                <span class="rounded-full px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] {deploymentStateTone(dep)}">
                                                                    {deploymentStateLabel(dep)}
                                                                </span>
                                                                <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] text-slate-600 dark:text-slate-300">
                                                                    {deploymentBusy(dep) ? 'Job Active' : 'Idle'}
                                                                </span>
                                                            </div>
                                                        </div>

                                                        <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
                                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Status</p>
                                                                <p class="mt-1 text-sm font-semibold text-slate-700 dark:text-slate-300">{deploymentStatusSummary(dep)}</p>
                                                            </div>
                                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Env Source</p>
                                                                <p class="mt-1 inline-flex rounded-full px-2 py-1 text-[10px] font-black uppercase tracking-[0.18em] {activeEnvSourceClass(dep)}">{activeEnvSourceLabel(dep)}</p>
                                                                <p class="mt-2 text-[10px] text-slate-500 dark:text-slate-400">{activeEnvSourceSummary(dep)}</p>
                                                            </div>
                                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Image Policy</p>
                                                                <p class="mt-1 inline-flex rounded-full px-2 py-1 text-[10px] font-black uppercase tracking-[0.18em] {deployImagePolicyClass(dep)}">{deployImagePolicyLabel(dep)}</p>
                                                                <p class="mt-2 text-[10px] text-slate-500 dark:text-slate-400">{deployImagePolicySummary(dep)}</p>
                                                            </div>
                                                            <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/75 dark:bg-slate-950/40 px-3 py-3">
                                                                <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Activity</p>
                                                                <p class="mt-1 text-sm font-semibold text-slate-700 dark:text-slate-300">{dep.deployStatusMessage || dep.lastError || (dep.lastDeployedAt ? `Last successful deploy ${formatRelativeTime(dep.lastDeployedAt)}` : 'Deployment has not run yet.')}</p>
                                                            </div>
                                                        </div>

                                                        <div class="rounded-2xl border border-slate-200/70 dark:border-slate-800 bg-white/70 dark:bg-slate-950/35 px-4 py-4">
                                                            <div class="flex items-center justify-between gap-3">
                                                                <div class="min-w-0">
                                                                    <p class="text-[9px] font-black uppercase tracking-[0.22em] text-slate-400">Deployment State</p>
                                                                    <p class="mt-1 text-[11px] text-slate-500 dark:text-slate-400">
                                                                        {deploymentBusy(dep) ? 'This card stays active while the deployment remains queued or running.' : 'No active job is attached to this rule.'}
                                                                    </p>
                                                                </div>
                                                                <div class="flex items-center gap-2">
                                                                    <span class="h-2.5 w-2.5 rounded-full {deploymentBusy(dep) ? 'bg-amber-500 animate-pulse' : dep.deployStatus === 'completed' ? 'bg-emerald-500' : dep.deployStatus === 'failed' ? 'bg-rose-500' : 'bg-slate-400'}"></span>
                                                                    <span class="rounded-full px-3 py-1 text-[10px] font-black uppercase tracking-[0.22em] {deploymentBusy(dep) ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300' : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'}">
                                                                        {deploymentBusy(dep) ? 'In Progress' : dep.deployStatus === 'failed' ? 'Failed' : dep.deployStatus === 'completed' ? 'Completed' : 'Idle'}
                                                                    </span>
                                                                </div>
                                                            </div>
                                                        </div>

                                                        {#if dep.envFilePath}
                                                            <p class="text-[10px] text-slate-500 mt-0.5 font-mono">Env file: {dep.envFilePath}</p>
                                                        {/if}

                                                        {#if dep.lastError}
                                                            <div class="rounded-2xl border border-rose-200 dark:border-rose-900/40 bg-rose-50 dark:bg-rose-950/20 px-3 py-3 text-[11px] text-rose-700 dark:text-rose-300">
                                                                {dep.lastError}
                                                            </div>
                                                        {/if}

                                                        <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
                                                            <button
                                                                onclick={() => deployNow(dep.id)}
                                                                disabled={deploying[dep.id] || dep.enabled === false}
                                                                class="inline-flex items-center justify-center rounded-2xl bg-brand-600 px-3 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-white shadow-lg shadow-brand-500/20 transition-all hover:bg-brand-700 disabled:opacity-50"
                                                            >
                                                                {deploying[dep.id] ? 'Queueing...' : dep.deployStatus === 'running' ? 'Deploying...' : dep.deployStatus === 'queued' ? 'Queued' : 'Deploy Now'}
                                                            </button>
                                                            <button
                                                                onclick={() => toggleDeployment(dep.id, source.id, dep.enabled === false)}
                                                                class="inline-flex items-center justify-center rounded-2xl border border-slate-200/80 dark:border-slate-800 bg-white/85 dark:bg-slate-950/50 px-3 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-700 dark:text-slate-200 transition-all hover:border-brand-400 hover:text-brand-700 dark:hover:text-brand-300"
                                                            >
                                                                {dep.enabled === false ? 'Enable' : 'Disable'}
                                                            </button>
                                                            <button
                                                                onclick={() => openEditDeploymentModal(source.id, dep)}
                                                                class="inline-flex items-center justify-center rounded-2xl border border-slate-200/80 dark:border-slate-800 bg-white/85 dark:bg-slate-950/50 px-3 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-700 dark:text-slate-200 transition-all hover:border-brand-400 hover:text-brand-700 dark:hover:text-brand-300"
                                                            >
                                                                Edit
                                                            </button>
                                                            <button
                                                                onclick={() => deleteDeployment(dep.id, source.id)}
                                                                aria-label={`Delete deployment ${dep.composePath}`}
                                                                class="inline-flex items-center justify-center rounded-2xl border border-slate-200/80 dark:border-slate-800 bg-white/85 dark:bg-slate-950/50 px-3 py-2.5 text-[10px] font-black uppercase tracking-[0.22em] text-slate-700 dark:text-slate-200 transition-all hover:border-rose-400 hover:text-rose-600 dark:hover:text-rose-300"
                                                            >
                                                                Delete
                                                            </button>
                                                        </div>
                                                    </div>
                                                </article>
                                            {/each}
                                        </div>
                                    {/if}
                                </div>
                            {/if}
                        </div>
                    </article>
                {/each}
            </div>
        {/if}
    </div>
</div>
<style>
    :global(.repo-noise), :global(.deployment-noise) {
        background-image:
            linear-gradient(rgba(255, 255, 255, 0.05) 1px, transparent 1px),
            linear-gradient(90deg, rgba(255, 255, 255, 0.05) 1px, transparent 1px);
        background-size: 12px 12px;
        opacity: 0.55;
        mix-blend-mode: soft-light;
    }

    :global(.animate-repo-pulse), :global(.animate-deployment-pulse) {
        animation: cardPulse 1.8s ease-in-out infinite alternate;
    }

    @keyframes cardPulse {
        0% {
            filter: saturate(1) brightness(1);
        }
        100% {
            filter: saturate(1.08) brightness(1.03);
        }
    }
</style>

{#if showAddSourceModal}
    <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 md:p-6 backdrop-blur-sm bg-slate-900/40 animate-in fade-in duration-300">
        <div class="bg-white dark:bg-[#0f172a] w-full max-w-xl rounded-[2.5rem] shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden animate-in zoom-in-95 duration-300">
            <div class="p-8 space-y-6">
                <div class="flex items-center justify-between">
                    <h3 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tight">Connect Repository</h3>
                    <button onclick={() => showAddSourceModal = false} aria-label="Close connect repository dialog" class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                <div class="space-y-4">
                    <div class="grid grid-cols-2 gap-4">
                        <div class="space-y-1.5">
                            <label for="src-name" class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">Friendly Name</label>
                            <input id="src-name" bind:value={newSource.name} placeholder="Production Stacks" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white" />
                        </div>
                        <div class="space-y-1.5">
                            <label for="src-branch" class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">Branch</label>
                            <input id="src-branch" bind:value={newSource.branch} placeholder="main" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white" />
                        </div>
                    </div>

                    <div class="space-y-1.5">
                        <label for="src-url" class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">Repository URL (HTTPS or SSH)</label>
                        <input id="src-url" bind:value={newSource.url} placeholder="https://github.com/user/repo.git" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white" />
                    </div>

                    <div class="space-y-1.5">
                        <label for="src-dir" class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">Target Sub-directory</label>
                        <input id="src-dir" bind:value={newSource.targetDir} placeholder="my-stack" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white" />
                        <p class="text-[10px] text-slate-500 ml-1 italic">The repo will be synced to {configStore.gitOpsMasterDirectory}/{newSource.targetDir || '...'}</p>
                    </div>

                    <div class="space-y-1.5 pt-2">
                        <p class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">Authentication Method</p>
                        <div class="flex flex-wrap gap-2">
                            {#each ['none', 'http_token', 'ssh_key'] as method}
                                <button 
                                    onclick={() => newSource.authMethod = method as any}
                                    class="px-4 py-2 rounded-xl text-[10px] font-black uppercase tracking-widest border transition-all {newSource.authMethod === method ? 'bg-brand-600 text-white border-brand-600' : 'bg-slate-50 dark:bg-slate-900/50 text-slate-500 border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600'}"
                                >
                                    {method === 'none' ? 'Public / None' : method === 'http_token' ? 'HTTP Token' : 'SSH Key'}
                                </button>
                            {/each}
                        </div>
                    </div>

                    {#if newSource.authMethod !== 'none'}
                        <div class="space-y-1.5 animate-in slide-in-from-top-2 duration-300">
                            <label for="src-secret" class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">
                                {newSource.authMethod === 'http_token' ? 'Personal Access Token' : 'Private SSH Key'}
                            </label>
                            {#if newSource.authMethod === 'ssh_key'}
                                <textarea id="src-secret" bind:value={newSource.authSecret} rows="4" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-xs font-mono outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"></textarea>
                            {:else}
                                <input id="src-secret" type="password" bind:value={newSource.authSecret} placeholder="ghp_xxxxxxxxxxxx" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white" />
                            {/if}
                        </div>
                    {/if}
                </div>

                <div class="flex gap-3 pt-4">
                    <button 
                        onclick={() => showAddSourceModal = false}
                        class="flex-1 px-6 py-3 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 rounded-2xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-slate-200 dark:hover:bg-slate-700"
                    >
                        Cancel
                    </button>
                    <button 
                        onclick={addSource}
                        class="flex-[2] px-6 py-3 bg-brand-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-brand-700 shadow-xl shadow-brand-500/20"
                    >
                        Connect Repository
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}

{#if showEditDeploymentModal}
    <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 md:p-6 backdrop-blur-sm bg-slate-900/40 animate-in fade-in duration-300">
        <div class="bg-white dark:bg-[#0f172a] w-full max-w-xl rounded-[2.5rem] shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden animate-in zoom-in-95 duration-300">
            <div class="p-8 space-y-6">
                <div class="flex items-center justify-between">
                    <div>
                        <h3 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tight">Edit Deployment Rule</h3>
                        <p class="text-[10px] font-bold text-slate-500 uppercase tracking-widest mt-1">Source: {sources.find(s => s.id === editingSourceId)?.name}</p>
                    </div>
                    <button onclick={() => { showEditDeploymentModal = false; editingDeploymentId = null; editingSourceId = null; }} aria-label="Close edit deployment dialog" class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                <div class="space-y-4">
                    <div class="space-y-1.5">
                        <div class="flex items-center justify-between ml-1">
                            <label for="edit-dep-path" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Compose File Path</label>
                            <button
                                onclick={() => editingSourceId && loadSourceFilesForPicker(editingSourceId, true)}
                                disabled={!editingSourceId || loadingSourceFiles[editingSourceId || ""]}
                                class="text-[9px] font-black uppercase tracking-widest text-brand-600 hover:underline disabled:opacity-40"
                            >
                                {editingSourceId && loadingSourceFiles[editingSourceId || ""] ? 'Refreshing...' : 'Refresh File List'}
                            </button>
                        </div>
                        <input id="edit-dep-path" bind:value={editDeployment.composePath} list={`compose-picker-edit-${editingSourceId || 'none'}`} placeholder="docker-compose.yml" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white font-mono" />
                        <datalist id={`compose-picker-edit-${editingSourceId || 'none'}`}>
                            {#each sourceFiles[editingSourceId || ""]?.composeFiles || [] as file}
                                <option value={file}></option>
                            {/each}
                        </datalist>
                    </div>

                    <div class="space-y-1.5">
                        <label for="edit-dep-env-file" class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">Env File Path (optional)</label>
                        <input id="edit-dep-env-file" bind:value={editDeployment.envFilePath} list={`env-picker-edit-${editingSourceId || 'none'}`} placeholder=".env or /mnt/Storage-SSD/dockercompose/app/.env" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white font-mono" />
                        <datalist id={`env-picker-edit-${editingSourceId || 'none'}`}>
                            {#each sourceFiles[editingSourceId || ""]?.envFiles || [] as file}
                                <option value={file}></option>
                            {/each}
                        </datalist>
                        {#if editDeployment.envInlineEnabled}
                            <p class="text-[10px] text-sky-600 dark:text-sky-300 ml-1 italic">Ignored while HarborWatch env override is enabled.</p>
                        {/if}
                    </div>

                    <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30 p-3 flex items-center justify-between">
                        <div>
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Deployment Active</p>
                            <p class="text-[10px] text-slate-500 mt-1">Keep disabled until this rule is fully configured.</p>
                        </div>
                        <button
                            onclick={() => editDeployment.enabled = !editDeployment.enabled}
                            class="w-10 h-5 rounded-full relative transition-colors {editDeployment.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                            aria-label="Toggle deployment active state"
                        >
                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {editDeployment.enabled ? 'right-1' : 'left-1'}"></div>
                        </button>
                    </div>

                    <div class="space-y-3 rounded-2xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30 p-4">
                        <div class="flex items-center justify-between gap-4">
                            <div>
                                <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">HarborWatch Env Override</p>
                                <p class="text-[10px] text-slate-500 mt-1">Uses a HarborWatch-managed <span class="font-mono">.env</span> outside the repo checkout and ignores any repo/default env file at deploy time.</p>
                            </div>
                            <button
                                onclick={() => editDeployment.envInlineEnabled = !editDeployment.envInlineEnabled}
                                class="w-10 h-5 rounded-full relative transition-colors {editDeployment.envInlineEnabled ? 'bg-sky-600' : 'bg-slate-300'}"
                                aria-label="Toggle HarborWatch env override"
                            >
                                <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {editDeployment.envInlineEnabled ? 'right-1' : 'left-1'}"></div>
                            </button>
                        </div>
                        <div class="flex items-center justify-between gap-4 rounded-xl border border-slate-200 dark:border-slate-700 bg-white/60 dark:bg-slate-950/40 px-3 py-2">
                            <div>
                                <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Image Refresh Policy</p>
                                <p class="text-[10px] text-slate-500 mt-1">Pull newer images before applying this stack.</p>
                            </div>
                            <button
                                onclick={() => editDeployment.pullOnDeploy = !editDeployment.pullOnDeploy}
                                class="w-10 h-5 rounded-full relative transition-colors {editDeployment.pullOnDeploy ? 'bg-emerald-600' : 'bg-slate-300'}"
                                aria-label="Toggle pull before deploy"
                            >
                                <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {editDeployment.pullOnDeploy ? 'right-1' : 'left-1'}"></div>
                            </button>
                        </div>
                        <textarea
                            bind:value={editDeployment.envInlineContent}
                            rows="10"
                            spellcheck="false"
                            autocapitalize="off"
                            autocomplete="off"
                            placeholder={"APP_ENV=prod\nAPI_BASE=https://example.com\nFEATURE_FLAG=true"}
                            disabled={!editDeployment.envInlineEnabled}
                            class="w-full bg-white dark:bg-slate-950/60 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-xs font-mono outline-none focus:ring-2 focus:ring-sky-500 transition-all text-slate-900 dark:text-white disabled:opacity-50"
                        ></textarea>
                    </div>
                </div>

                <div class="flex gap-3 pt-4">
                    <button
                        onclick={() => { showEditDeploymentModal = false; editingDeploymentId = null; editingSourceId = null; }}
                        class="flex-1 px-6 py-3 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 rounded-2xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-slate-200 dark:hover:bg-slate-700"
                    >
                        Cancel
                    </button>
                    <button
                        onclick={saveDeploymentEdits}
                        class="flex-[2] px-6 py-3 bg-brand-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-brand-700 shadow-xl shadow-brand-500/20"
                    >
                        Save Changes
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}

{#if showAddDeploymentModal}
    <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 md:p-6 backdrop-blur-sm bg-slate-900/40 animate-in fade-in duration-300">
        <div class="bg-white dark:bg-[#0f172a] w-full max-w-xl rounded-[2.5rem] shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden animate-in zoom-in-95 duration-300">
            <div class="p-8 space-y-6">
                <div class="flex items-center justify-between">
                    <div>
                        <h3 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tight">Deploy Stack</h3>
                        <p class="text-[10px] font-bold text-slate-500 uppercase tracking-widest mt-1">Source: {sources.find(s => s.id === selectedSourceId)?.name}</p>
                    </div>
                    <button onclick={() => showAddDeploymentModal = false} aria-label="Close deploy stack dialog" class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                <div class="space-y-4">
                    <div class="space-y-1.5">
                        <div class="flex items-center justify-between ml-1">
                            <label for="dep-path" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Compose File Path (relative to repo root)</label>
                            <button
                                onclick={() => selectedSourceId && loadSourceFilesForPicker(selectedSourceId, true)}
                                disabled={!selectedSourceId || loadingSourceFiles[selectedSourceId || ""]}
                                class="text-[9px] font-black uppercase tracking-widest text-brand-600 hover:underline disabled:opacity-40"
                            >
                                {selectedSourceId && loadingSourceFiles[selectedSourceId || ""] ? 'Refreshing...' : 'Refresh File List'}
                            </button>
                        </div>
                        <input id="dep-path" bind:value={newDeployment.composePath} list={`compose-picker-${selectedSourceId || 'none'}`} placeholder="docker-compose.yml" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white font-mono" />
                        <datalist id={`compose-picker-${selectedSourceId || 'none'}`}>
                            {#each sourceFiles[selectedSourceId || ""]?.composeFiles || [] as file}
                                <option value={file}></option>
                            {/each}
                        </datalist>
                        {#if selectedSourceId && (sourceFiles[selectedSourceId || ""]?.composeFiles || []).length === 0}
                            <p class="text-[10px] text-slate-500 ml-1 italic">No compose files discovered yet. Sync repository and refresh.</p>
                        {/if}
                        <p class="text-[10px] text-slate-500 ml-1 italic">
                            Mapped path: {resolvedComposeMapping(sources.find(s => s.id === selectedSourceId)?.targetDir || "", newDeployment.composePath)}
                        </p>
                    </div>

                    <div class="space-y-1.5">
                        <label for="dep-env-file" class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">Env File Path (optional)</label>
                        <input id="dep-env-file" bind:value={newDeployment.envFilePath} list={`env-picker-${selectedSourceId || 'none'}`} placeholder=".env or /mnt/Storage-SSD/dockercompose/app/.env" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white font-mono" />
                        <datalist id={`env-picker-${selectedSourceId || 'none'}`}>
                            {#each sourceFiles[selectedSourceId || ""]?.envFiles || [] as file}
                                <option value={file}></option>
                            {/each}
                        </datalist>
                        <p class="text-[10px] text-slate-500 ml-1 italic">Supports repo-relative paths and absolute host paths.</p>
                        {#if newDeployment.envInlineEnabled}
                            <p class="text-[10px] text-sky-600 dark:text-sky-300 ml-1 italic">Ignored while HarborWatch env override is enabled.</p>
                        {/if}
                    </div>

                    <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30 p-3 flex items-center justify-between">
                        <div>
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Deployment Active</p>
                            <p class="text-[10px] text-slate-500 mt-1">Disabled rules are kept but skipped during auto-sync deploys.</p>
                        </div>
                        <button
                            onclick={() => newDeployment.enabled = !newDeployment.enabled}
                            class="w-10 h-5 rounded-full relative transition-colors {newDeployment.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                            aria-label="Toggle deployment active state"
                        >
                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {newDeployment.enabled ? 'right-1' : 'left-1'}"></div>
                        </button>
                    </div>

                    <div class="space-y-3 rounded-2xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30 p-4">
                        <div class="flex items-center justify-between gap-4">
                            <div>
                                <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">HarborWatch Env Override</p>
                                <p class="text-[10px] text-slate-500 mt-1">Use a HarborWatch-managed <span class="font-mono">.env</span> outside the repo checkout. This ignores any repo/default env file during deploy.</p>
                            </div>
                            <button
                                onclick={() => newDeployment.envInlineEnabled = !newDeployment.envInlineEnabled}
                                class="w-10 h-5 rounded-full relative transition-colors {newDeployment.envInlineEnabled ? 'bg-sky-600' : 'bg-slate-300'}"
                                aria-label="Toggle HarborWatch env override"
                            >
                                <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {newDeployment.envInlineEnabled ? 'right-1' : 'left-1'}"></div>
                            </button>
                        </div>
                        <div class="flex items-center justify-between gap-4 rounded-xl border border-slate-200 dark:border-slate-700 bg-white/60 dark:bg-slate-950/40 px-3 py-2">
                            <div>
                                <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Image Refresh Policy</p>
                                <p class="text-[10px] text-slate-500 mt-1">Pull newer images before applying this stack.</p>
                            </div>
                            <button
                                onclick={() => newDeployment.pullOnDeploy = !newDeployment.pullOnDeploy}
                                class="w-10 h-5 rounded-full relative transition-colors {newDeployment.pullOnDeploy ? 'bg-emerald-600' : 'bg-slate-300'}"
                                aria-label="Toggle pull before deploy"
                            >
                                <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {newDeployment.pullOnDeploy ? 'right-1' : 'left-1'}"></div>
                            </button>
                        </div>
                        <textarea
                            bind:value={newDeployment.envInlineContent}
                            rows="10"
                            spellcheck="false"
                            autocapitalize="off"
                            autocomplete="off"
                            placeholder={"APP_ENV=prod\nAPI_BASE=https://example.com\nFEATURE_FLAG=true"}
                            disabled={!newDeployment.envInlineEnabled}
                            class="w-full bg-white dark:bg-slate-950/60 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-xs font-mono outline-none focus:ring-2 focus:ring-sky-500 transition-all text-slate-900 dark:text-white disabled:opacity-50"
                        ></textarea>
                    </div>
                </div>

                <div class="flex gap-3 pt-4">
                    <button 
                        onclick={() => showAddDeploymentModal = false}
                        class="flex-1 px-6 py-3 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 rounded-2xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-slate-200 dark:hover:bg-slate-700"
                    >
                        Cancel
                    </button>
                    <button 
                        onclick={addDeployment}
                        class="flex-[2] px-6 py-3 bg-emerald-600 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-emerald-700 shadow-xl shadow-emerald-500/20"
                    >
                        Save Deployment Rule
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}
