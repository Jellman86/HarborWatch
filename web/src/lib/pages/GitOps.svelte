<script lang="ts">
    import { onDestroy, onMount } from "svelte";
    import { toasts } from "../stores/ToastStore";
    import { configStore } from "../stores/config.svelte";
    import StaticNoise from "../components/StaticNoise.svelte";

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
        return "Repo Default";
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

    function deploymentActivityText(dep: GitDeployment): string {
        if (dep.deployStatusMessage) return dep.deployStatusMessage;
        if (dep.lastError) return dep.lastError;
        if (dep.lastDeployedAt) {
            return `Last deployed ${formatRelativeTime(dep.lastDeployedAt)}${dep.lastDeployedHash ? ` · ${dep.lastDeployedHash.slice(0, 7)}` : ''}`;
        }
        return "Not yet deployed";
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

    function sourceStateChipClass(source: GitSource): string {
        if (sourceBusy(source)) return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300";
        if (source.lastSyncError) return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300";
        if (source.lastCommitHash) return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300";
        return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
    }

    function sourceStateLabel(source: GitSource): string {
        if (sourceBusy(source)) return "Syncing";
        if (source.lastSyncError) return "Sync Error";
        if (source.lastCommitHash) return "Synced";
        return "Not Synced";
    }

    function sourceCardAccent(source: GitSource): string {
        if (sourceBusy(source)) return "from-amber-400 via-amber-300/20 to-transparent";
        if (source.lastSyncError) return "from-rose-400 via-rose-300/10 to-transparent";
        if (source.lastCommitHash) return "from-emerald-400/60 via-brand-400/20 to-transparent";
        return "from-slate-400/30 via-transparent to-transparent";
    }

    function deploymentBusy(dep: GitDeployment): boolean {
        return dep.deployStatus === "queued" || dep.deployStatus === "running" || deploying[dep.id] === true;
    }

    function deploymentCardAccent(dep: GitDeployment): string {
        if (deploymentBusy(dep)) return "from-amber-400/80 via-amber-300/20 to-transparent";
        if (dep.deployStatus === "failed") return "from-rose-400/80 via-rose-300/10 to-transparent";
        if (dep.deployStatus === "completed") return "from-emerald-400/60 via-brand-400/20 to-transparent";
        if (dep.enabled === false) return "from-slate-300/30 via-transparent to-transparent";
        return "from-brand-400/40 via-transparent to-transparent";
    }

    function deployBtnLabel(dep: GitDeployment): string {
        if (deploying[dep.id]) return "Queueing...";
        if (dep.deployStatus === "running") return "Deploying...";
        if (dep.deployStatus === "queued") return "Queued";
        return "Deploy Now";
    }
</script>

<!-- ── Wrapper ─────────────────────────────────────────────────── -->
<div class={embedded ? "space-y-4" : "relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.10),_transparent_34%),radial-gradient(circle_at_top_right,_rgba(16,185,129,0.09),_transparent_28%),linear-gradient(180deg,_rgba(248,250,252,1),_rgba(241,245,249,0.72))] dark:bg-[radial-gradient(circle_at_top_left,_rgba(37,99,235,0.14),_transparent_34%),radial-gradient(circle_at_top_right,_rgba(16,185,129,0.12),_transparent_28%),linear-gradient(180deg,_rgba(2,6,23,1),_rgba(15,23,42,0.82))]"}>
    {#if !embedded}
        <div class="absolute inset-0 pointer-events-none opacity-40 bg-[linear-gradient(rgba(148,163,184,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(148,163,184,0.08)_1px,transparent_1px)] bg-[size:42px_42px]"></div>
    {/if}

    <div class={embedded ? "space-y-4" : "relative mx-auto max-w-[96rem] px-4 py-5 md:px-8 md:py-8 space-y-6"}>

        <!-- ── Standalone page header (non-embedded only) ──────── -->
        {#if !embedded}
            <div class="flex flex-wrap items-center justify-between gap-4 opacity-0 animate-reveal">
                <div class="border-l-4 border-brand-600 pl-4">
                    <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">GitOps</h2>
                    <p class="text-xs text-slate-500 font-medium">Repository-backed stacks with automated sync and deploy.</p>
                </div>
                <div class="flex items-center gap-2">
                    <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-[10px] font-black text-slate-600 dark:text-slate-400 uppercase tracking-widest border border-slate-200 dark:border-slate-700">
                        {sources.length} Repos
                    </span>
                    <span class="px-3 py-1 bg-slate-100 dark:bg-slate-800 rounded-full text-[10px] font-black text-slate-600 dark:text-slate-400 uppercase tracking-widest border border-slate-200 dark:border-slate-700">
                        {countDeployments()} Rules
                    </span>
                    {#if countBusyDeployments() > 0}
                        <span class="px-3 py-1 bg-amber-100 dark:bg-amber-900/30 rounded-full text-[10px] font-black text-amber-700 dark:text-amber-300 uppercase tracking-widest border border-amber-200 dark:border-amber-900/50">
                            {countBusyDeployments()} Active
                        </span>
                    {/if}
                    <button
                        onclick={loadSources}
                        class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 transition-colors hover:border-brand-400 hover:text-brand-600 dark:hover:text-brand-300"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                        Refresh
                    </button>
                    <button
                        onclick={() => showAddSourceModal = true}
                        class="inline-flex items-center gap-1.5 rounded-xl bg-brand-600 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-white shadow-sm shadow-brand-500/20 transition-colors hover:bg-brand-700"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M12 4v16m8-8H4" />
                        </svg>
                        Add Repository
                    </button>
                </div>
            </div>
        {/if}

        <!-- ── Embedded action bar (embedded only) ─────────────── -->
        {#if embedded}
            <div class="flex flex-wrap items-center justify-between gap-3 opacity-0 animate-reveal">
                <div>
                    <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">GitOps Repositories</p>
                    <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
                        {sources.length} {sources.length === 1 ? 'repository' : 'repositories'}
                        {#if countDeployments() > 0}, {countDeployments()} deployment {countDeployments() === 1 ? 'rule' : 'rules'}{/if}
                        {#if countBusyDeployments() > 0} · <span class="text-amber-600 dark:text-amber-400 font-semibold">{countBusyDeployments()} active</span>{/if}
                    </p>
                </div>
                <div class="flex items-center gap-2">
                    <button
                        onclick={loadSources}
                        class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 transition-colors hover:border-brand-400 hover:text-brand-600 dark:hover:text-brand-300"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                        Refresh
                    </button>
                    <button
                        onclick={() => showAddSourceModal = true}
                        class="inline-flex items-center gap-1.5 rounded-xl bg-brand-600 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-white shadow-sm shadow-brand-500/20 transition-colors hover:bg-brand-700"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M12 4v16m8-8H4" />
                        </svg>
                        Add Repository
                    </button>
                </div>
            </div>
        {/if}

        <!-- ── Loading ─────────────────────────────────────────── -->
        {#if loading && sources.length === 0}
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950/80 px-6 py-14 text-center">
                <div class="mx-auto h-10 w-10 rounded-full border-4 border-brand-500 border-t-transparent animate-spin"></div>
                <p class="mt-4 text-[10px] font-black uppercase tracking-[0.28em] text-slate-400">Discovering repositories</p>
            </div>

        <!-- ── Empty state ─────────────────────────────────────── -->
        {:else if sources.length === 0}
            <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-800 bg-white/75 dark:bg-slate-950/50 px-8 py-14 text-center opacity-0 animate-reveal">
                <div class="mx-auto mb-5 flex h-14 w-14 items-center justify-center rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900 text-slate-300 dark:text-slate-700">
                    <!-- git branch icon -->
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                    </svg>
                </div>
                <h3 class="text-lg font-black text-slate-900 dark:text-white uppercase tracking-tight">No Repositories Connected</h3>
                <p class="mt-2 max-w-md mx-auto text-sm text-slate-500 dark:text-slate-400">Add a Git repository to start managing compose stacks with automated sync and deploy.</p>
                <button
                    onclick={() => showAddSourceModal = true}
                    class="mt-6 inline-flex items-center gap-2 rounded-2xl bg-brand-600 px-6 py-2.5 text-[11px] font-black uppercase tracking-widest text-white shadow-lg shadow-brand-500/20 transition-all hover:bg-brand-700"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M12 4v16m8-8H4" />
                    </svg>
                    Connect First Repository
                </button>
            </div>

        <!-- ── Repository list ─────────────────────────────────── -->
        {:else}
            <div class="space-y-5">
                {#each sources as source, index}
                    <article
                        style="animation-delay: {0.06 + index * 0.04}s"
                        class="opacity-0 animate-reveal group relative overflow-hidden rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-md
                            {syncing[source.id] ? 'border-amber-400 dark:border-amber-500 ring-2 ring-amber-400/30' :
                             source.lastSyncError ? 'border-rose-300 dark:border-rose-800/60' :
                             'hover:border-brand-400 dark:hover:border-brand-500'}"
                    >
                        <!-- TV static noise during sync -->
                        <StaticNoise active={syncing[source.id]} />

                        <!-- Top accent bar — hidden while static is showing -->
                        {#if !syncing[source.id]}
                            <div class="relative h-0.5 bg-gradient-to-r {sourceCardAccent(source)}"></div>
                        {/if}

                        <!-- Card body -->
                        <div class="relative space-y-4 p-5">

                            <!-- ── Identity + State zone ──────── -->
                            <div class="flex items-start justify-between gap-4">
                                <div class="min-w-0 flex-1">
                                    <p class="text-[10px] font-black uppercase tracking-[0.24em] text-slate-400">Git Repository</p>
                                    <h3 class="mt-1 truncate text-lg font-black tracking-tight text-slate-900 dark:text-white" title={source.name}>
                                        {source.name}
                                    </h3>
                                    <p class="mt-0.5 truncate text-[10px] font-mono text-slate-500 dark:text-slate-400" title={source.url}>
                                        {source.url}
                                    </p>
                                </div>
                                <div class="flex shrink-0 flex-col items-end gap-1.5">
                                    <span class="rounded-full px-2.5 py-1 text-[10px] font-black uppercase tracking-widest {sourceStateChipClass(source)}">
                                        {#if syncing[source.id]}
                                            <span class="mr-1.5 inline-block h-2 w-2 rounded-full bg-amber-500 animate-pulse align-middle"></span>
                                        {/if}
                                        {sourceStateLabel(source)}
                                    </span>
                                </div>
                            </div>

                            <!-- ── Metadata chips ─────────────── -->
                            <div class="flex flex-wrap items-center gap-1.5">
                                <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300">
                                    {source.branch}
                                </span>
                                <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300">
                                    {source.authMethod === 'none' ? 'Public' : source.authMethod === 'http_token' ? 'Token Auth' : 'SSH Key'}
                                </span>
                                <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300">
                                    Sync {source.syncIntervalMins}m
                                </span>
                                {#if source.lastCommitHash}
                                    <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-2.5 py-0.5 text-[10px] font-mono text-slate-500 dark:text-slate-400">
                                        {source.lastCommitHash.slice(0, 7)}
                                    </span>
                                {/if}
                                {#if source.targetDir}
                                    <span class="rounded-full bg-slate-100 dark:bg-slate-800 px-2.5 py-0.5 text-[10px] font-mono text-slate-500 dark:text-slate-400">
                                        /{source.targetDir}
                                    </span>
                                {/if}
                            </div>

                            <!-- Sync error banner -->
                            {#if source.lastSyncError}
                                <div class="flex items-start gap-2.5 rounded-xl border border-rose-200 dark:border-rose-900/40 bg-rose-50 dark:bg-rose-950/20 px-3.5 py-3 text-xs text-rose-700 dark:text-rose-300">
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                    </svg>
                                    <span>{source.lastSyncError}</span>
                                </div>
                            {/if}

                            <!-- ── Controls zone ──────────────── -->
                            <div class="flex items-center justify-between border-t border-slate-100 dark:border-slate-800 pt-4 gap-3">
                                <span class="text-[10px] text-slate-400 font-medium">
                                    Synced {formatRelativeTime(source.lastSyncAt)}
                                    {#if (deployments[source.id] || []).length > 0}
                                        · {(deployments[source.id] || []).length} stack{(deployments[source.id] || []).length !== 1 ? 's' : ''}
                                    {/if}
                                </span>
                                <div class="flex flex-wrap items-center gap-2">
                                    <button
                                        onclick={() => syncSource(source.id)}
                                        disabled={syncing[source.id]}
                                        class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/50 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-700 dark:text-slate-200 transition-all hover:border-brand-400 hover:text-brand-600 dark:hover:text-brand-300 disabled:opacity-50"
                                    >
                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 {syncing[source.id] ? 'animate-spin' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                        </svg>
                                        {syncing[source.id] ? 'Syncing...' : 'Sync'}
                                    </button>
                                    <button
                                        onclick={() => openAddDeploymentModal(source.id)}
                                        class="inline-flex items-center gap-1.5 rounded-xl bg-brand-600 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-white shadow-sm shadow-brand-500/20 transition-all hover:bg-brand-700"
                                    >
                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M12 4v16m8-8H4" />
                                        </svg>
                                        Add Stack
                                    </button>
                                    <button
                                        onclick={() => deleteSource(source.id)}
                                        class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/50 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-700 dark:text-slate-200 transition-all hover:border-rose-400 hover:text-rose-600 dark:hover:text-rose-300"
                                    >
                                        Remove
                                    </button>
                                </div>
                            </div>
                        </div>

                        <!-- ── Deployment rules section ──────────────── -->
                        {#if (deployments[source.id] || []).length === 0}
                            <div class="border-t border-slate-100 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-950/20 px-5 py-5 text-center">
                                <p class="text-xs text-slate-400 dark:text-slate-500 italic">No deployment rules yet.</p>
                                <button
                                    onclick={() => openAddDeploymentModal(source.id)}
                                    class="mt-2 text-[10px] font-black uppercase tracking-widest text-brand-600 dark:text-brand-400 hover:underline"
                                >
                                    Configure first stack
                                </button>
                            </div>
                        {:else}
                            <div class="border-t border-slate-100 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-950/20 px-5 py-4 space-y-3">
                                <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">
                                    Deployment Rules
                                    <span class="ml-1.5 rounded-full bg-slate-200 dark:bg-slate-800 px-2 py-0.5 text-[9px] text-slate-500 dark:text-slate-400">
                                        {(deployments[source.id] || []).length}
                                    </span>
                                </p>

                                {#each deployments[source.id] as dep}
                                    <article
                                        class="relative overflow-hidden rounded-2xl border transition-all
                                            {deploymentBusy(dep) ? 'border-amber-400/70 dark:border-amber-500/50 ring-1 ring-amber-400/20' :
                                             dep.deployStatus === 'failed' ? 'border-rose-300 dark:border-rose-800/60' :
                                             dep.enabled === false ? 'border-slate-200 dark:border-slate-800 opacity-75' :
                                             'border-slate-200 dark:border-slate-700 hover:border-brand-400/60 dark:hover:border-brand-500/40'}
                                            bg-white dark:bg-slate-900 shadow-sm"
                                    >
                                        <!-- TV static noise during deploy -->
                                        <StaticNoise active={deploymentBusy(dep)} />

                                        <!-- Accent bar — hidden while static is showing -->
                                        {#if !deploymentBusy(dep)}
                                            <div class="relative h-0.5 bg-gradient-to-r {deploymentCardAccent(dep)}"></div>
                                        {/if}

                                        <div class="relative space-y-3 p-4">
                                            <!-- ── Identity + state row ──── -->
                                            <div class="flex items-start justify-between gap-3">
                                                <div class="min-w-0 flex-1">
                                                    <div class="flex flex-wrap items-center gap-1.5 mb-1.5">
                                                        <p class="text-[10px] font-black uppercase tracking-[0.22em] text-slate-400">Stack</p>
                                                        <span class="rounded-full px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest {deploymentStateClass(dep)}">
                                                            {#if deploymentBusy(dep)}
                                                                <span class="mr-1 inline-block h-1.5 w-1.5 rounded-full bg-current animate-pulse align-middle"></span>
                                                            {/if}
                                                            {deploymentStateLabel(dep)}
                                                        </span>
                                                        {#if dep.autoCreated && dep.enabled === false}
                                                            <span class="rounded-full bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300 px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest">
                                                                Discovered
                                                            </span>
                                                        {/if}
                                                    </div>
                                                    <h4 class="truncate text-sm font-black tracking-tight text-slate-900 dark:text-white font-mono" title={dep.composePath}>
                                                        {dep.composePath}
                                                    </h4>
                                                    <p class="mt-0.5 truncate text-[10px] font-mono text-slate-400 dark:text-slate-500" title={resolvedComposeMapping(source.targetDir, dep.composePath)}>
                                                        {resolvedComposeMapping(source.targetDir, dep.composePath)}
                                                    </p>
                                                </div>

                                                <!-- Policy badges (right-aligned on desktop) -->
                                                <div class="hidden sm:flex shrink-0 flex-col items-end gap-1">
                                                    <span class="rounded-full px-2 py-0.5 text-[9px] font-black uppercase tracking-widest {activeEnvSourceClass(dep)}">
                                                        {activeEnvSourceLabel(dep)}
                                                    </span>
                                                    <span class="rounded-full px-2 py-0.5 text-[9px] font-black uppercase tracking-widest {deployImagePolicyClass(dep)}">
                                                        {deployImagePolicyLabel(dep)}
                                                    </span>
                                                </div>
                                            </div>

                                            <!-- Mobile policy badges -->
                                            <div class="flex flex-wrap gap-1.5 sm:hidden">
                                                <span class="rounded-full px-2 py-0.5 text-[9px] font-black uppercase tracking-widest {activeEnvSourceClass(dep)}">{activeEnvSourceLabel(dep)}</span>
                                                <span class="rounded-full px-2 py-0.5 text-[9px] font-black uppercase tracking-widest {deployImagePolicyClass(dep)}">{deployImagePolicyLabel(dep)}</span>
                                            </div>

                                            <!-- Error banner -->
                                            {#if dep.lastError && dep.deployStatus === 'failed'}
                                                <div class="flex items-start gap-2 rounded-xl border border-rose-200 dark:border-rose-900/40 bg-rose-50 dark:bg-rose-950/20 px-3 py-2.5 text-xs text-rose-700 dark:text-rose-300">
                                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                                    </svg>
                                                    <span class="break-all">{dep.lastError}</span>
                                                </div>
                                            {/if}

                                            <!-- ── Footer: activity + actions ─ -->
                                            <div class="flex flex-wrap items-center justify-between gap-2 border-t border-slate-100 dark:border-slate-800 pt-3">
                                                <p class="text-[10px] text-slate-500 dark:text-slate-400 min-w-0 truncate">
                                                    {deploymentActivityText(dep)}
                                                </p>
                                                <div class="flex shrink-0 items-center gap-1.5">
                                                    <button
                                                        onclick={() => deleteDeployment(dep.id, source.id)}
                                                        aria-label="Delete deployment rule"
                                                        class="inline-flex items-center justify-center h-7 w-7 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/50 text-slate-400 transition-all hover:border-rose-400 hover:text-rose-600 dark:hover:text-rose-400"
                                                        title="Delete rule"
                                                    >
                                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                                        </svg>
                                                    </button>
                                                    <button
                                                        onclick={() => openEditDeploymentModal(source.id, dep)}
                                                        class="inline-flex items-center gap-1 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/50 px-2.5 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 transition-all hover:border-brand-400 hover:text-brand-600 dark:hover:text-brand-300"
                                                    >
                                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                                        </svg>
                                                        Edit
                                                    </button>
                                                    <button
                                                        onclick={() => toggleDeployment(dep.id, source.id, dep.enabled === false)}
                                                        class="inline-flex items-center gap-1 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800/50 px-2.5 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 transition-all hover:border-brand-400 hover:text-brand-600 dark:hover:text-brand-300"
                                                    >
                                                        {dep.enabled === false ? 'Enable' : 'Disable'}
                                                    </button>
                                                    <button
                                                        onclick={() => deployNow(dep.id)}
                                                        disabled={deploying[dep.id] || deploymentBusy(dep) || dep.enabled === false}
                                                        class="inline-flex items-center gap-1.5 rounded-xl bg-brand-600 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-white shadow-sm shadow-brand-500/20 transition-all hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-50"
                                                    >
                                                        {#if deploymentBusy(dep)}
                                                            <span class="h-2.5 w-2.5 rounded-full border-2 border-white border-t-transparent animate-spin"></span>
                                                        {:else}
                                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                            </svg>
                                                        {/if}
                                                        {deployBtnLabel(dep)}
                                                    </button>
                                                </div>
                                            </div>
                                        </div>
                                    </article>
                                {/each}
                            </div>
                        {/if}
                    </article>
                {/each}
            </div>
        {/if}

    </div>
</div>


<!-- ── Add Repository Modal ──────────────────────────────────────── -->
{#if showAddSourceModal}
    <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 md:p-6 backdrop-blur-sm bg-slate-900/50">
        <div class="bg-white dark:bg-slate-950 w-full max-w-lg rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden">

            <!-- Modal header -->
            <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 px-6 py-4">
                <div>
                    <h3 class="text-base font-black text-slate-900 dark:text-white uppercase tracking-tight">Connect Repository</h3>
                    <p class="text-[10px] text-slate-500 mt-0.5">Add a Git repository as a GitOps deployment source.</p>
                </div>
                <button
                    onclick={() => showAddSourceModal = false}
                    aria-label="Close"
                    class="rounded-xl p-1.5 text-slate-400 transition-colors hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-600 dark:hover:text-slate-200"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                </button>
            </div>

            <div class="px-6 py-5 space-y-4 max-h-[70vh] overflow-y-auto">

                <!-- Name + Branch -->
                <div class="grid grid-cols-2 gap-4">
                    <div class="space-y-1.5">
                        <label for="src-name" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Friendly Name</label>
                        <input
                            id="src-name"
                            bind:value={newSource.name}
                            placeholder="Production Stacks"
                            class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                        />
                    </div>
                    <div class="space-y-1.5">
                        <label for="src-branch" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Branch</label>
                        <input
                            id="src-branch"
                            bind:value={newSource.branch}
                            placeholder="main"
                            class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                        />
                    </div>
                </div>

                <!-- URL -->
                <div class="space-y-1.5">
                    <label for="src-url" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Repository URL</label>
                    <input
                        id="src-url"
                        bind:value={newSource.url}
                        placeholder="https://github.com/user/repo.git"
                        class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                    />
                </div>

                <!-- Target dir + Sync interval -->
                <div class="grid grid-cols-2 gap-4">
                    <div class="space-y-1.5">
                        <label for="src-dir" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Target Sub-directory</label>
                        <input
                            id="src-dir"
                            bind:value={newSource.targetDir}
                            placeholder="my-stack"
                            class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                        />
                        <p class="text-[10px] text-slate-400">Synced to: <span class="font-mono">{configStore.gitOpsMasterDirectory}/{newSource.targetDir || '...'}</span></p>
                    </div>
                    <div class="space-y-1.5">
                        <label for="src-interval" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Sync Interval (minutes)</label>
                        <input
                            id="src-interval"
                            type="number"
                            bind:value={newSource.syncIntervalMins}
                            min="1"
                            step="1"
                            class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                        />
                    </div>
                </div>

                <!-- Auth method -->
                <div class="space-y-2">
                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Authentication</p>
                    <div class="flex flex-wrap gap-2">
                        {#each ['none', 'http_token', 'ssh_key'] as method}
                            <button
                                onclick={() => newSource.authMethod = method as any}
                                class="px-3.5 py-2 rounded-xl text-[10px] font-black uppercase tracking-widest border transition-all {newSource.authMethod === method ? 'bg-brand-600 text-white border-brand-600' : 'bg-slate-50 dark:bg-slate-900/50 text-slate-500 dark:text-slate-400 border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600'}"
                            >
                                {method === 'none' ? 'Public / None' : method === 'http_token' ? 'HTTP Token' : 'SSH Key'}
                            </button>
                        {/each}
                    </div>
                </div>

                {#if newSource.authMethod !== 'none'}
                    <div class="space-y-1.5">
                        <label for="src-secret" class="text-[10px] font-black uppercase tracking-widest text-slate-400">
                            {newSource.authMethod === 'http_token' ? 'Personal Access Token' : 'Private SSH Key'}
                        </label>
                        {#if newSource.authMethod === 'ssh_key'}
                            <textarea
                                id="src-secret"
                                bind:value={newSource.authSecret}
                                rows="4"
                                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                                class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-xs font-mono outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                            ></textarea>
                        {:else}
                            <input
                                id="src-secret"
                                type="password"
                                bind:value={newSource.authSecret}
                                placeholder="ghp_xxxxxxxxxxxx"
                                class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                            />
                        {/if}
                    </div>
                {/if}
            </div>

            <!-- Modal footer -->
            <div class="flex gap-3 border-t border-slate-100 dark:border-slate-800 px-6 py-4">
                <button
                    onclick={() => showAddSourceModal = false}
                    class="flex-1 px-5 py-2.5 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-slate-200 dark:hover:bg-slate-700"
                >
                    Cancel
                </button>
                <button
                    onclick={addSource}
                    class="flex-[2] px-5 py-2.5 bg-brand-600 text-white rounded-xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-brand-700 shadow-lg shadow-brand-500/20"
                >
                    Connect Repository
                </button>
            </div>
        </div>
    </div>
{/if}

<!-- ── Edit Deployment Modal ─────────────────────────────────────── -->
{#if showEditDeploymentModal}
    <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 md:p-6 backdrop-blur-sm bg-slate-900/50">
        <div class="bg-white dark:bg-slate-950 w-full max-w-lg rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden">

            <!-- Modal header -->
            <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 px-6 py-4">
                <div>
                    <h3 class="text-base font-black text-slate-900 dark:text-white uppercase tracking-tight">Edit Deployment Rule</h3>
                    <p class="text-[10px] text-slate-500 mt-0.5">Source: <span class="font-semibold text-slate-700 dark:text-slate-300">{sources.find(s => s.id === editingSourceId)?.name}</span></p>
                </div>
                <button
                    onclick={() => { showEditDeploymentModal = false; editingDeploymentId = null; editingSourceId = null; }}
                    aria-label="Close"
                    class="rounded-xl p-1.5 text-slate-400 transition-colors hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-600 dark:hover:text-slate-200"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                </button>
            </div>

            <div class="px-6 py-5 space-y-4 max-h-[70vh] overflow-y-auto">

                <!-- Compose file path -->
                <div class="space-y-1.5">
                    <div class="flex items-center justify-between">
                        <label for="edit-dep-path" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Compose File Path</label>
                        <button
                            onclick={() => editingSourceId && loadSourceFilesForPicker(editingSourceId, true)}
                            disabled={!editingSourceId || loadingSourceFiles[editingSourceId || ""]}
                            class="text-[9px] font-black uppercase tracking-widest text-brand-600 dark:text-brand-400 hover:underline disabled:opacity-40"
                        >
                            {editingSourceId && loadingSourceFiles[editingSourceId || ""] ? 'Refreshing...' : 'Refresh File List'}
                        </button>
                    </div>
                    <input
                        id="edit-dep-path"
                        bind:value={editDeployment.composePath}
                        list={`compose-picker-edit-${editingSourceId || 'none'}`}
                        placeholder="docker-compose.yml"
                        class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                    />
                    <datalist id={`compose-picker-edit-${editingSourceId || 'none'}`}>
                        {#each sourceFiles[editingSourceId || ""]?.composeFiles || [] as file}
                            <option value={file}></option>
                        {/each}
                    </datalist>
                </div>

                <!-- Env file path -->
                <div class="space-y-1.5">
                    <label for="edit-dep-env-file" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Env File Path <span class="normal-case font-normal text-slate-400">(optional)</span></label>
                    <input
                        id="edit-dep-env-file"
                        bind:value={editDeployment.envFilePath}
                        list={`env-picker-edit-${editingSourceId || 'none'}`}
                        placeholder=".env"
                        class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                    />
                    <datalist id={`env-picker-edit-${editingSourceId || 'none'}`}>
                        {#each sourceFiles[editingSourceId || ""]?.envFiles || [] as file}
                            <option value={file}></option>
                        {/each}
                    </datalist>
                    {#if editDeployment.envInlineEnabled}
                        <p class="text-[10px] text-sky-600 dark:text-sky-400 italic">Ignored while HarborWatch env override is enabled.</p>
                    {/if}
                </div>

                <!-- Toggles -->
                <div class="space-y-2 rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-4">
                    <!-- Deployment active -->
                    <div class="flex items-center justify-between gap-4 py-1">
                        <div class="min-w-0">
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Deployment Active</p>
                            <p class="text-[10px] text-slate-400 mt-0.5">Disabled rules are skipped during auto-sync deploys.</p>
                        </div>
                        <button
                            onclick={() => editDeployment.enabled = !editDeployment.enabled}
                            class="relative shrink-0 w-10 h-5 rounded-full transition-colors {editDeployment.enabled ? 'bg-brand-600' : 'bg-slate-300 dark:bg-slate-600'}"
                            aria-label="Toggle deployment active"
                        >
                            <span class="absolute top-1 w-3 h-3 rounded-full bg-white shadow transition-all {editDeployment.enabled ? 'right-1' : 'left-1'}"></span>
                        </button>
                    </div>

                    <div class="border-t border-slate-200 dark:border-slate-700/60 my-1"></div>

                    <!-- HarborWatch env override -->
                    <div class="flex items-center justify-between gap-4 py-1">
                        <div class="min-w-0">
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">HarborWatch Env Override</p>
                            <p class="text-[10px] text-slate-400 mt-0.5">Use a HarborWatch-managed <span class="font-mono">.env</span>, ignoring repo files at deploy time.</p>
                        </div>
                        <button
                            onclick={() => editDeployment.envInlineEnabled = !editDeployment.envInlineEnabled}
                            class="relative shrink-0 w-10 h-5 rounded-full transition-colors {editDeployment.envInlineEnabled ? 'bg-sky-600' : 'bg-slate-300 dark:bg-slate-600'}"
                            aria-label="Toggle HarborWatch env override"
                        >
                            <span class="absolute top-1 w-3 h-3 rounded-full bg-white shadow transition-all {editDeployment.envInlineEnabled ? 'right-1' : 'left-1'}"></span>
                        </button>
                    </div>

                    <!-- Image refresh policy -->
                    <div class="flex items-center justify-between gap-4 py-1">
                        <div class="min-w-0">
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Pull Before Deploy</p>
                            <p class="text-[10px] text-slate-400 mt-0.5">Pull newer images before applying this stack.</p>
                        </div>
                        <button
                            onclick={() => editDeployment.pullOnDeploy = !editDeployment.pullOnDeploy}
                            class="relative shrink-0 w-10 h-5 rounded-full transition-colors {editDeployment.pullOnDeploy ? 'bg-emerald-600' : 'bg-slate-300 dark:bg-slate-600'}"
                            aria-label="Toggle pull before deploy"
                        >
                            <span class="absolute top-1 w-3 h-3 rounded-full bg-white shadow transition-all {editDeployment.pullOnDeploy ? 'right-1' : 'left-1'}"></span>
                        </button>
                    </div>
                </div>

                <!-- Inline env content -->
                {#if editDeployment.envInlineEnabled}
                    <div class="space-y-1.5">
                        <label for="edit-env-content" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Env Content</label>
                        <textarea
                            id="edit-env-content"
                            bind:value={editDeployment.envInlineContent}
                            rows="8"
                            spellcheck="false"
                            autocapitalize="off"
                            autocomplete="off"
                            placeholder={"APP_ENV=prod\nAPI_BASE=https://example.com\nFEATURE_FLAG=true"}
                            class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-xs font-mono outline-none focus:ring-2 focus:ring-sky-500 transition-all text-slate-900 dark:text-white"
                        ></textarea>
                    </div>
                {/if}
            </div>

            <!-- Modal footer -->
            <div class="flex gap-3 border-t border-slate-100 dark:border-slate-800 px-6 py-4">
                <button
                    onclick={() => { showEditDeploymentModal = false; editingDeploymentId = null; editingSourceId = null; }}
                    class="flex-1 px-5 py-2.5 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-slate-200 dark:hover:bg-slate-700"
                >
                    Cancel
                </button>
                <button
                    onclick={saveDeploymentEdits}
                    class="flex-[2] px-5 py-2.5 bg-brand-600 text-white rounded-xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-brand-700 shadow-lg shadow-brand-500/20"
                >
                    Save Changes
                </button>
            </div>
        </div>
    </div>
{/if}

<!-- ── Add Deployment Modal ──────────────────────────────────────── -->
{#if showAddDeploymentModal}
    <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 md:p-6 backdrop-blur-sm bg-slate-900/50">
        <div class="bg-white dark:bg-slate-950 w-full max-w-lg rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden">

            <!-- Modal header -->
            <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 px-6 py-4">
                <div>
                    <h3 class="text-base font-black text-slate-900 dark:text-white uppercase tracking-tight">Add Deployment Rule</h3>
                    <p class="text-[10px] text-slate-500 mt-0.5">Repository: <span class="font-semibold text-slate-700 dark:text-slate-300">{sources.find(s => s.id === selectedSourceId)?.name}</span></p>
                </div>
                <button
                    onclick={() => showAddDeploymentModal = false}
                    aria-label="Close"
                    class="rounded-xl p-1.5 text-slate-400 transition-colors hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-600 dark:hover:text-slate-200"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                </button>
            </div>

            <div class="px-6 py-5 space-y-4 max-h-[70vh] overflow-y-auto">

                <!-- Compose file path -->
                <div class="space-y-1.5">
                    <div class="flex items-center justify-between">
                        <label for="dep-path" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Compose File Path</label>
                        <button
                            onclick={() => selectedSourceId && loadSourceFilesForPicker(selectedSourceId, true)}
                            disabled={!selectedSourceId || loadingSourceFiles[selectedSourceId || ""]}
                            class="text-[9px] font-black uppercase tracking-widest text-brand-600 dark:text-brand-400 hover:underline disabled:opacity-40"
                        >
                            {selectedSourceId && loadingSourceFiles[selectedSourceId || ""] ? 'Refreshing...' : 'Refresh File List'}
                        </button>
                    </div>
                    <input
                        id="dep-path"
                        bind:value={newDeployment.composePath}
                        list={`compose-picker-${selectedSourceId || 'none'}`}
                        placeholder="docker-compose.yml"
                        class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                    />
                    <datalist id={`compose-picker-${selectedSourceId || 'none'}`}>
                        {#each sourceFiles[selectedSourceId || ""]?.composeFiles || [] as file}
                            <option value={file}></option>
                        {/each}
                    </datalist>
                    {#if selectedSourceId && (sourceFiles[selectedSourceId || ""]?.composeFiles || []).length === 0}
                        <p class="text-[10px] text-slate-400 italic">No compose files discovered. Sync the repository first.</p>
                    {/if}
                    <p class="text-[10px] text-slate-400 font-mono">
                        → {resolvedComposeMapping(sources.find(s => s.id === selectedSourceId)?.targetDir || "", newDeployment.composePath)}
                    </p>
                </div>

                <!-- Env file path -->
                <div class="space-y-1.5">
                    <label for="dep-env-file" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Env File Path <span class="normal-case font-normal text-slate-400">(optional)</span></label>
                    <input
                        id="dep-env-file"
                        bind:value={newDeployment.envFilePath}
                        list={`env-picker-${selectedSourceId || 'none'}`}
                        placeholder=".env"
                        class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white"
                    />
                    <datalist id={`env-picker-${selectedSourceId || 'none'}`}>
                        {#each sourceFiles[selectedSourceId || ""]?.envFiles || [] as file}
                            <option value={file}></option>
                        {/each}
                    </datalist>
                    <p class="text-[10px] text-slate-400">Supports repo-relative paths and absolute host paths.</p>
                    {#if newDeployment.envInlineEnabled}
                        <p class="text-[10px] text-sky-600 dark:text-sky-400 italic">Ignored while HarborWatch env override is enabled.</p>
                    {/if}
                </div>

                <!-- Toggles -->
                <div class="space-y-2 rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-4">
                    <!-- Deployment active -->
                    <div class="flex items-center justify-between gap-4 py-1">
                        <div class="min-w-0">
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Deployment Active</p>
                            <p class="text-[10px] text-slate-400 mt-0.5">Disabled rules are kept but skipped during auto-sync deploys.</p>
                        </div>
                        <button
                            onclick={() => newDeployment.enabled = !newDeployment.enabled}
                            class="relative shrink-0 w-10 h-5 rounded-full transition-colors {newDeployment.enabled ? 'bg-brand-600' : 'bg-slate-300 dark:bg-slate-600'}"
                            aria-label="Toggle deployment active"
                        >
                            <span class="absolute top-1 w-3 h-3 rounded-full bg-white shadow transition-all {newDeployment.enabled ? 'right-1' : 'left-1'}"></span>
                        </button>
                    </div>

                    <div class="border-t border-slate-200 dark:border-slate-700/60 my-1"></div>

                    <!-- HarborWatch env override -->
                    <div class="flex items-center justify-between gap-4 py-1">
                        <div class="min-w-0">
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">HarborWatch Env Override</p>
                            <p class="text-[10px] text-slate-400 mt-0.5">Use a HarborWatch-managed <span class="font-mono">.env</span>, ignoring repo files at deploy time.</p>
                        </div>
                        <button
                            onclick={() => newDeployment.envInlineEnabled = !newDeployment.envInlineEnabled}
                            class="relative shrink-0 w-10 h-5 rounded-full transition-colors {newDeployment.envInlineEnabled ? 'bg-sky-600' : 'bg-slate-300 dark:bg-slate-600'}"
                            aria-label="Toggle HarborWatch env override"
                        >
                            <span class="absolute top-1 w-3 h-3 rounded-full bg-white shadow transition-all {newDeployment.envInlineEnabled ? 'right-1' : 'left-1'}"></span>
                        </button>
                    </div>

                    <!-- Pull policy -->
                    <div class="flex items-center justify-between gap-4 py-1">
                        <div class="min-w-0">
                            <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">Pull Before Deploy</p>
                            <p class="text-[10px] text-slate-400 mt-0.5">Pull newer images before applying this stack.</p>
                        </div>
                        <button
                            onclick={() => newDeployment.pullOnDeploy = !newDeployment.pullOnDeploy}
                            class="relative shrink-0 w-10 h-5 rounded-full transition-colors {newDeployment.pullOnDeploy ? 'bg-emerald-600' : 'bg-slate-300 dark:bg-slate-600'}"
                            aria-label="Toggle pull before deploy"
                        >
                            <span class="absolute top-1 w-3 h-3 rounded-full bg-white shadow transition-all {newDeployment.pullOnDeploy ? 'right-1' : 'left-1'}"></span>
                        </button>
                    </div>
                </div>

                <!-- Inline env content (conditional) -->
                {#if newDeployment.envInlineEnabled}
                    <div class="space-y-1.5">
                        <label for="new-env-content" class="text-[10px] font-black uppercase tracking-widest text-slate-400">Env Content</label>
                        <textarea
                            id="new-env-content"
                            bind:value={newDeployment.envInlineContent}
                            rows="8"
                            spellcheck="false"
                            autocapitalize="off"
                            autocomplete="off"
                            placeholder={"APP_ENV=prod\nAPI_BASE=https://example.com\nFEATURE_FLAG=true"}
                            class="w-full bg-slate-50 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700 rounded-xl px-3.5 py-2.5 text-xs font-mono outline-none focus:ring-2 focus:ring-sky-500 transition-all text-slate-900 dark:text-white"
                        ></textarea>
                    </div>
                {/if}
            </div>

            <!-- Modal footer -->
            <div class="flex gap-3 border-t border-slate-100 dark:border-slate-800 px-6 py-4">
                <button
                    onclick={() => showAddDeploymentModal = false}
                    class="flex-1 px-5 py-2.5 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 rounded-xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-slate-200 dark:hover:bg-slate-700"
                >
                    Cancel
                </button>
                <button
                    onclick={addDeployment}
                    class="flex-[2] px-5 py-2.5 bg-brand-600 text-white rounded-xl font-black uppercase tracking-widest text-[10px] transition-all hover:bg-brand-700 shadow-lg shadow-brand-500/20"
                >
                    Save Deployment Rule
                </button>
            </div>
        </div>
    </div>
{/if}
