<script lang="ts">
    import { onMount } from "svelte";
    import { toasts } from "../stores/ToastStore";
    import { configStore } from "../stores/config.svelte";

    let { onNavigate } = $props<{
        onNavigate: (route: string, params?: any) => void;
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
        lastDeployedHash?: string;
        lastDeployedAt: number;
        lastError?: string;
    }

    // Component State
    let sources = $state<GitSource[]>([]);
    let deployments = $state<Record<string, GitDeployment[]>>({});
    let loading = $state(true);
    let syncing = $state<Record<string, boolean>>({});
    let deploying = $state<Record<string, boolean>>({});
    let showAddSourceModal = $state(false);
    let showAddDeploymentModal = $state(false);
    let selectedSourceId = $state<string | null>(null);
    let expandedSourceId = $state<string | null>(null);

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

    let newDeployment = $state({
        composePath: "docker-compose.yml",
        envVars: [] as { key: string, value: string }[]
    });

    onMount(() => {
        loadSources();
    });

    async function loadSources() {
        loading = true;
        try {
            const res = await fetch("/api/gitops/sources");
            if (res.ok) {
                sources = await res.json();
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
                deployments[sourceId] = await res.json();
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
        
        // Convert envVars array to JSON map
        const envMap: Record<string, string> = {};
        newDeployment.envVars.forEach(v => {
            if (v.key.trim()) envMap[v.key.trim()] = v.value;
        });

        try {
            const res = await fetch("/api/gitops/deployments", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    gitSourceId: selectedSourceId,
                    composePath: newDeployment.composePath,
                    envVarsJson: JSON.stringify(envMap)
                })
            });
            if (res.ok) {
                toasts.success("Deployment rule added");
                showAddDeploymentModal = false;
                loadDeployments(selectedSourceId);
                // Reset
                newDeployment = {
                    composePath: "docker-compose.yml",
                    envVars: []
                };
            } else {
                const data = await res.json();
                toasts.error(data.message || "Failed to add deployment");
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

    async function deployNow(id: string) {
        if (deploying[id]) return;
        deploying[id] = true;
        toasts.info("Triggering deployment...");
        try {
            const res = await fetch(`/api/gitops/deployments/${id}/deploy`, { method: "POST" });
            if (res.ok) {
                toasts.success("Deployment successful");
                // Refresh list to show new hash/status
                const sourceID = sources.find(s => (deployments[s.id] || []).some(d => d.id === id))?.id;
                if (sourceID) loadDeployments(sourceID);
            } else {
                const data = await res.json();
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

    function addEnvVar() {
        newDeployment.envVars = [...newDeployment.envVars, { key: "", value: "" }];
    }

    function removeEnvVar(index: number) {
        newDeployment.envVars = newDeployment.envVars.filter((_, i) => i !== index);
    }

    function formatRelativeTime(ts: number) {
        if (!ts) return "Never";
        const diff = Math.floor(Date.now() / 1000) - ts;
        if (diff < 60) return "Just now";
        if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
        return new Date(ts * 1000).toLocaleDateString();
    }
</script>

<div class="p-6 md:p-8 space-y-8 max-w-7xl mx-auto">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
            <h2 class="text-3xl font-black text-slate-900 dark:text-white tracking-tight flex items-center gap-3">
                <span class="p-2.5 rounded-2xl bg-brand-500/10 text-brand-600 dark:text-brand-400 border border-brand-500/20">
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 4a3 3 0 00-3 3v4a3 3 0 003 3h4a3 3 0 003-3V7a3 3 0 00-3-3H8zM2 14a2 2 0 012-2h16a2 2 0 012 2v2a2 2 0 01-2 2H4a2 2 0 01-2-2v-2z" />
                    </svg>
                </span>
                GitOps Repositories
            </h2>
            <p class="text-slate-500 dark:text-slate-400 mt-2 font-medium">Manage container stacks directly from Git repositories with automatic synchronization.</p>
        </div>
        <button 
            onclick={() => showAddSourceModal = true}
            class="px-5 py-2.5 bg-brand-600 hover:bg-brand-700 text-white rounded-xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-brand-500/20 transition-all hover:scale-105 active:scale-95 flex items-center gap-2 self-start"
        >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M12 4v16m8-8H4" />
            </svg>
            Add Repository
        </button>
    </div>

    {#if loading && sources.length === 0}
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {#each Array(3) as _}
                <div class="h-64 rounded-3xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 animate-pulse"></div>
            {/each}
        </div>
    {:else if sources.length === 0}
        <div class="rounded-[2.5rem] border-2 border-dashed border-slate-200 dark:border-slate-800 p-16 text-center">
            <div class="w-20 h-20 bg-slate-50 dark:bg-slate-900 rounded-3xl flex items-center justify-center mx-auto mb-6">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-10 w-10 text-slate-300 dark:text-slate-700" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 4a3 3 0 00-3 3v4a3 3 0 003 3h4a3 3 0 003-3V7a3 3 0 00-3-3H8zM2 14a2 2 0 012-2h16a2 2 0 012 2v2a2 2 0 01-2 2H4a2 2 0 01-2-2v-2z" />
                </svg>
            </div>
            <h3 class="text-xl font-black text-slate-900 dark:text-white uppercase tracking-tight">No Repositories Connected</h3>
            <p class="text-slate-500 dark:text-slate-400 mt-2 max-w-sm mx-auto font-medium">Add a Git repository to start managing your compose stacks with native GitOps workflows.</p>
            <button 
                onclick={() => showAddSourceModal = true}
                class="mt-8 px-8 py-3 bg-brand-600 hover:bg-brand-700 text-white rounded-2xl font-black uppercase tracking-widest text-[11px] transition-all hover:shadow-xl hover:shadow-brand-500/20"
            >
                Connect Your First Repository
            </button>
        </div>
    {:else}
        <div class="grid grid-cols-1 gap-6">
            {#each sources as source}
                <div class="group relative bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-[2rem] overflow-hidden transition-all duration-500 hover:shadow-2xl hover:shadow-brand-500/5 {expandedSourceId === source.id ? 'border-brand-500/40' : 'hover:border-brand-500/30'}">
                    <div class="p-6 md:p-8 space-y-6">
                        <div class="flex items-start justify-between gap-4">
                            <div class="flex items-center gap-4">
                                <button 
                                    onclick={() => toggleExpand(source.id)}
                                    class="w-14 h-14 rounded-2xl bg-slate-50 dark:bg-slate-800 flex items-center justify-center border border-slate-100 dark:border-slate-700 transition-all group-hover:bg-brand-500/5 group-hover:border-brand-500/20 active:scale-90"
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7 text-slate-400 dark:text-slate-500 group-hover:text-brand-500 transition-transform duration-300 {expandedSourceId === source.id ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                                    </svg>
                                </button>
                                <div>
                                    <h4 class="text-xl font-black text-slate-900 dark:text-white uppercase tracking-tight leading-tight">{source.name}</h4>
                                    <div class="flex items-center gap-2 mt-1.5">
                                        <span class="px-2 py-0.5 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 text-[10px] font-black uppercase tracking-widest border border-slate-200/50 dark:border-slate-700/50">{source.branch}</span>
                                        <span class="text-[11px] text-slate-400 dark:text-slate-500 font-medium truncate max-w-[200px]">{source.url}</span>
                                    </div>
                                </div>
                            </div>
                            <div class="flex items-center gap-2">
                                <button 
                                    onclick={() => {
                                        selectedSourceId = source.id;
                                        showAddDeploymentModal = true;
                                    }}
                                    class="px-4 py-2 rounded-xl bg-brand-500/10 text-brand-600 dark:text-brand-400 text-[10px] font-black uppercase tracking-widest border border-brand-500/20 hover:bg-brand-500/20 transition-all flex items-center gap-2"
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M12 4v16m8-8H4" />
                                    </svg>
                                    Add Stack
                                </button>
                                <button 
                                    onclick={() => syncSource(source.id)}
                                    disabled={syncing[source.id]}
                                    class="p-2.5 rounded-xl bg-slate-50 dark:bg-slate-800 text-slate-600 dark:text-slate-400 hover:bg-brand-500/10 hover:text-brand-600 dark:hover:text-brand-400 transition-all border border-slate-100 dark:border-slate-700 disabled:opacity-50"
                                    title="Sync Repository"
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 {syncing[source.id] ? 'animate-spin' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                    </svg>
                                </button>
                                <button 
                                    onclick={() => deleteSource(source.id)}
                                    class="p-2.5 rounded-xl bg-slate-50 dark:bg-slate-800 text-slate-600 dark:text-slate-400 hover:bg-rose-500/10 hover:text-rose-600 transition-all border border-slate-100 dark:border-slate-700"
                                    title="Remove Repository"
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                    </svg>
                                </button>
                            </div>
                        </div>

                        <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <div class="p-4 rounded-2xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                <p class="text-[9px] font-black text-slate-400 dark:text-slate-500 uppercase tracking-widest">Last Sync</p>
                                <p class="text-sm font-bold text-slate-700 dark:text-slate-200 mt-1">{formatRelativeTime(source.lastSyncAt)}</p>
                            </div>
                            <div class="p-4 rounded-2xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                <p class="text-[9px] font-black text-slate-400 dark:text-slate-500 uppercase tracking-widest">Commit</p>
                                <p class="text-sm font-mono font-bold text-slate-700 dark:text-slate-200 mt-1">{source.lastCommitHash ? source.lastCommitHash.slice(0, 7) : "N/A"}</p>
                            </div>
                            <div class="p-4 rounded-2xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                <p class="text-[9px] font-black text-slate-400 dark:text-slate-500 uppercase tracking-widest">Auth</p>
                                <p class="text-sm font-bold text-slate-700 dark:text-slate-200 mt-1 uppercase tracking-tight">
                                    {source.authMethod === 'none' ? 'Public' : source.authMethod === 'http_token' ? 'Token' : 'SSH Key'}
                                </p>
                            </div>
                            <div class="p-4 rounded-2xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                <p class="text-[9px] font-black text-slate-400 dark:text-slate-500 uppercase tracking-widest">Active Stacks</p>
                                <p class="text-sm font-bold text-slate-700 dark:text-slate-200 mt-1">{deployments[source.id]?.length || 0}</p>
                            </div>
                        </div>

                        {#if expandedSourceId === source.id}
                            <div class="pt-6 border-t border-slate-100 dark:border-slate-800 space-y-4 animate-in slide-in-from-top-4 duration-500">
                                <h5 class="text-xs font-black text-slate-400 uppercase tracking-widest ml-1">Deployment Rules</h5>
                                {#if (deployments[source.id] || []).length === 0}
                                    <div class="p-8 text-center bg-slate-50 dark:bg-slate-800/30 rounded-3xl border border-dashed border-slate-200 dark:border-slate-700">
                                        <p class="text-sm text-slate-500 font-medium italic">No compose files selected for deployment from this repository.</p>
                                        <button 
                                            onclick={() => {
                                                selectedSourceId = source.id;
                                                showAddDeploymentModal = true;
                                            }}
                                            class="mt-4 text-[10px] font-black uppercase tracking-widest text-brand-600 dark:text-brand-400 hover:underline"
                                        >
                                            Configure first stack
                                        </button>
                                    </div>
                                {:else}
                                    <div class="grid grid-cols-1 gap-3">
                                        {#each deployments[source.id] as dep}
                                            <div class="flex items-center justify-between p-4 bg-slate-50 dark:bg-slate-800/50 rounded-2xl border border-slate-100 dark:border-slate-700">
                                                <div class="flex items-center gap-4">
                                                    <div class="p-2 rounded-xl bg-white dark:bg-slate-900 shadow-sm border border-slate-100 dark:border-slate-800">
                                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-brand-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6M7 4h10a2 2 0 012 2v12a2 2 0 01-2 2H7a2 2 0 01-2-2V6a2 2 0 012-2z" />
                                                        </svg>
                                                    </div>
                                                    <div>
                                                        <p class="text-sm font-bold text-slate-900 dark:text-white font-mono">{dep.composePath}</p>
                                                        <p class="text-[10px] text-slate-500 mt-0.5">Last deployed: {formatRelativeTime(dep.lastDeployedAt)} {dep.lastDeployedHash ? `(${dep.lastDeployedHash.slice(0, 7)})` : ''}</p>
                                                    </div>
                                                </div>
                                                <div class="flex items-center gap-2">
                                                    {#if dep.lastError}
                                                        <span class="p-2 text-rose-500" title={dep.lastError}>
                                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                            </svg>
                                                        </span>
                                                    {/if}
                                                    <button 
                                                        onclick={() => deployNow(dep.id)}
                                                        disabled={deploying[dep.id]}
                                                        class="px-3 py-1.5 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-400 hover:text-brand-600 dark:hover:text-brand-400 transition-all disabled:opacity-50"
                                                    >
                                                        {deploying[dep.id] ? 'Deploying...' : 'Deploy Now'}
                                                    </button>
                                                    <button 
                                                        onclick={() => deleteDeployment(dep.id, source.id)}
                                                        class="p-1.5 text-slate-400 hover:text-rose-500 transition-colors"
                                                    >
                                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                                        </svg>
                                                    </button>
                                                </div>
                                            </div>
                                        {/each}
                                    </div>
                                {/if}
                            </div>
                        {/if}

                        {#if source.lastSyncError}
                            <div class="p-4 rounded-2xl bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/20 flex items-start gap-3">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-rose-500 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                </svg>
                                <p class="text-[11px] text-rose-700 dark:text-rose-400 font-medium leading-relaxed">{source.lastSyncError}</p>
                            </div>
                        {/if}
                    </div>
                </div>
            {/each}
        </div>
    {/if}
</div>

{#if showAddSourceModal}
    <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 md:p-6 backdrop-blur-sm bg-slate-900/40 animate-in fade-in duration-300">
        <div class="bg-white dark:bg-[#0f172a] w-full max-w-xl rounded-[2.5rem] shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden animate-in zoom-in-95 duration-300">
            <div class="p-8 space-y-6">
                <div class="flex items-center justify-between">
                    <h3 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tight">Connect Repository</h3>
                    <button onclick={() => showAddSourceModal = false} class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
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
                        <label class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">Authentication Method</label>
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

{#if showAddDeploymentModal}
    <div class="fixed inset-0 z-[100] flex items-center justify-center p-4 md:p-6 backdrop-blur-sm bg-slate-900/40 animate-in fade-in duration-300">
        <div class="bg-white dark:bg-[#0f172a] w-full max-w-xl rounded-[2.5rem] shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden animate-in zoom-in-95 duration-300">
            <div class="p-8 space-y-6">
                <div class="flex items-center justify-between">
                    <div>
                        <h3 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tight">Deploy Stack</h3>
                        <p class="text-[10px] font-bold text-slate-500 uppercase tracking-widest mt-1">Source: {sources.find(s => s.id === selectedSourceId)?.name}</p>
                    </div>
                    <button onclick={() => showAddDeploymentModal = false} class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                <div class="space-y-4">
                    <div class="space-y-1.5">
                        <label for="dep-path" class="text-[10px] font-black uppercase tracking-widest text-slate-400 ml-1">Compose File Path (relative to repo root)</label>
                        <input id="dep-path" bind:value={newDeployment.composePath} placeholder="docker-compose.yml" class="w-full bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-2xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all text-slate-900 dark:text-white font-mono" />
                    </div>

                    <div class="space-y-3">
                        <div class="flex items-center justify-between ml-1">
                            <label class="text-[10px] font-black uppercase tracking-widest text-slate-400">Environment Variables (.env)</label>
                            <button onclick={addEnvVar} class="text-[9px] font-black text-brand-600 uppercase tracking-widest hover:underline">+ Add Variable</button>
                        </div>
                        
                        {#if newDeployment.envVars.length === 0}
                            <p class="text-[11px] text-slate-500 italic ml-1">No custom environment variables defined.</p>
                        {:else}
                            <div class="space-y-2 max-h-48 overflow-y-auto pr-2 custom-scrollbar">
                                {#each newDeployment.envVars as env, i}
                                    <div class="flex gap-2 items-center animate-in slide-in-from-left-2 duration-200" style="--index: {i}">
                                        <input bind:value={env.key} placeholder="KEY" class="flex-1 bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs font-mono outline-none focus:ring-2 focus:ring-brand-500 text-slate-900 dark:text-white" />
                                        <input bind:value={env.value} type="password" placeholder="VALUE" class="flex-1 bg-slate-50 dark:bg-slate-900/50 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs font-mono outline-none focus:ring-2 focus:ring-brand-500 text-slate-900 dark:text-white" />
                                        <button onclick={() => removeEnvVar(i)} class="p-2 text-slate-400 hover:text-rose-500">
                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                            </svg>
                                        </button>
                                    </div>
                                {/each}
                            </div>
                        {/if}
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
