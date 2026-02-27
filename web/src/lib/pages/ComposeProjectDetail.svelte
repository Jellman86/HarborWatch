<script lang="ts">
    import { onMount } from "svelte";
    import { toasts } from "../stores/ToastStore";

    interface LocalComposeProjectMember {
        containerId: string;
        containerName?: string;
        serviceName?: string;
        image?: string;
        state?: string;
        updateAvailable?: boolean;
    }

    interface EditorFileState {
        path: string;
        content?: string;
        sha256?: string;
        exists: boolean;
        writable: boolean;
        sizeBytes?: number;
    }

    interface ValidationDiagnostic {
        severity: string;
        source: string;
        path?: string;
        message: string;
    }

    interface ComposeProjectEditorResponse {
        projectKey: string;
        projectName: string;
        workingDir?: string;
        sourceStatus: string;
        sourceVerified: boolean;
        sourceWritable: boolean;
        composeFiles: EditorFileState[];
        envFile?: EditorFileState;
        members?: LocalComposeProjectMember[];
    }

    let { params, onNavigate } = $props<{
        params?: { projectKey?: string };
        onNavigate: (route: string, params?: any) => void;
    }>();

    let loading = $state(true);
    let saving = $state(false);
    let validating = $state(false);
    let error = $state("");
    let project = $state<ComposeProjectEditorResponse | null>(null);
    let composeDrafts = $state<Record<string, string>>({});
    let composeExpectedHashes = $state<Record<string, string>>({});
    let activeComposePath = $state("");
    let envDraft = $state("");
    let envExpectedHash = $state("");
    let diagnostics = $state<ValidationDiagnostic[]>([]);

    const projectKey = $derived(String(params?.projectKey || "").trim());
    const activeComposeDraft = $derived(activeComposePath ? (composeDrafts[activeComposePath] || "") : "");
    const hasComposeChanges = $derived(
        !!project && project.composeFiles.some((f) => (composeDrafts[f.path] || "") !== (f.content || ""))
    );
    const hasEnvChanges = $derived(
        !!project?.envFile && envDraft !== (project.envFile.content || "")
    );
    const hasAnyChanges = $derived(hasComposeChanges || hasEnvChanges);

    function setActiveComposeDraft(value: string) {
        if (!activeComposePath) return;
        composeDrafts[activeComposePath] = value;
    }

    async function loadProject() {
        if (!projectKey) {
            error = "Project key is missing.";
            loading = false;
            return;
        }
        loading = true;
        error = "";
        diagnostics = [];
        try {
            const res = await fetch(`/api/compose/projects/${encodeURIComponent(projectKey)}`);
            const data = await res.json().catch(() => ({}));
            if (!res.ok) {
                error = data.message || "Failed to load compose project.";
                project = null;
                return;
            }
            project = data as ComposeProjectEditorResponse;
            composeDrafts = {};
            composeExpectedHashes = {};
            for (const file of project.composeFiles || []) {
                composeDrafts[file.path] = file.content || "";
                composeExpectedHashes[file.path] = file.sha256 || "";
            }
            activeComposePath = (project.composeFiles && project.composeFiles.length > 0) ? project.composeFiles[0].path : "";
            if (project.envFile) {
                envDraft = project.envFile.content || "";
                envExpectedHash = project.envFile.sha256 || "";
            } else {
                envDraft = "";
                envExpectedHash = "";
            }
        } catch (e) {
            error = "Failed to load compose project.";
            project = null;
        } finally {
            loading = false;
        }
    }

    async function validateDraft() {
        if (!project) return;
        validating = true;
        diagnostics = [];
        try {
            const payload: any = {
                composeFiles: (project.composeFiles || []).map((f) => ({
                    path: f.path,
                    content: composeDrafts[f.path] || ""
                }))
            };
            if (project.envFile) {
                payload.envFile = {
                    path: project.envFile.path,
                    content: envDraft,
                    exists: project.envFile.exists
                };
            }
            const res = await fetch(`/api/compose/projects/${encodeURIComponent(project.projectKey)}/validate`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(payload)
            });
            const data = await res.json().catch(() => ({}));
            diagnostics = Array.isArray(data.diagnostics) ? data.diagnostics : [];
            if (res.ok) {
                toasts.success("Compose validation passed.");
            } else {
                toasts.error("Validation failed. Review diagnostics.");
            }
        } catch (e) {
            toasts.error("Failed to validate compose draft.");
        } finally {
            validating = false;
        }
    }

    async function prettifyDraft() {
        if (!project) return;
        try {
            const payload: any = {
                composeFiles: (project.composeFiles || []).map((f) => ({
                    path: f.path,
                    content: composeDrafts[f.path] || ""
                }))
            };
            const res = await fetch(`/api/compose/projects/${encodeURIComponent(project.projectKey)}/prettify`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(payload)
            });
            const data = await res.json().catch(() => ({}));
            if (!res.ok) {
                toasts.error(data.message || "Failed to prettify compose draft.");
                return;
            }
            const updated = Array.isArray(data.composeFiles) ? data.composeFiles : [];
            for (const item of updated) {
                if (item && item.path) {
                    composeDrafts[item.path] = item.content || "";
                }
            }
            toasts.success("Compose files prettified.");
        } catch (e) {
            toasts.error("Failed to prettify compose draft.");
        }
    }

    async function saveDraft() {
        if (!project) return;
        saving = true;
        try {
            const payload: any = {
                composeFiles: (project.composeFiles || []).map((f) => ({
                    path: f.path,
                    content: composeDrafts[f.path] || "",
                    expectedSha256: composeExpectedHashes[f.path] || ""
                }))
            };
            if (project.envFile) {
                payload.envFile = {
                    path: project.envFile.path,
                    content: envDraft,
                    exists: project.envFile.exists,
                    expectedSha256: envExpectedHash
                };
            }

            const res = await fetch(`/api/compose/projects/${encodeURIComponent(project.projectKey)}`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(payload)
            });
            const data = await res.json().catch(() => ({}));
            if (!res.ok) {
                const message = data.message || "Failed to save compose draft.";
                toasts.error(message);
                diagnostics = Array.isArray(data.diagnostics) ? data.diagnostics : diagnostics;
                return;
            }
            const updatedComposeFiles = Array.isArray(data.composeFiles) ? data.composeFiles : [];
            const updatedEnvFile = data.envFile || undefined;
            project = {
                ...project,
                composeFiles: updatedComposeFiles,
                envFile: updatedEnvFile
            };
            composeExpectedHashes = {};
            for (const file of updatedComposeFiles) {
                composeExpectedHashes[file.path] = file.sha256 || "";
                composeDrafts[file.path] = file.content || "";
            }
            if (updatedEnvFile) {
                envExpectedHash = updatedEnvFile.sha256 || "";
                envDraft = updatedEnvFile.content || "";
            }
            toasts.success("Compose project saved.");
        } catch (e) {
            toasts.error("Failed to save compose draft.");
        } finally {
            saving = false;
        }
    }

    onMount(() => {
        loadProject();
    });
</script>

<div class="space-y-6">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div>
            <button
                onclick={() => onNavigate("stacks")}
                class="mb-2 inline-flex items-center gap-2 px-3 py-1.5 rounded-lg border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
            >
                Back to Stacks
            </button>
            <h2 class="text-2xl font-black tracking-tight text-slate-900 dark:text-white">Compose Project Editor</h2>
            <p class="text-xs text-slate-500">Edit local compose source and project environment file.</p>
        </div>
        {#if project}
            <span class="px-2 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300">
                {project.projectName}
            </span>
        {/if}
    </div>

    {#if loading}
        <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 p-8 text-sm text-slate-500">
            Loading compose project...
        </div>
    {:else if error}
        <div class="bg-rose-50 dark:bg-rose-900/10 rounded-2xl border border-rose-200 dark:border-rose-900/40 p-4 text-sm text-rose-700 dark:text-rose-300">
            {error}
        </div>
    {:else if project}
        <section class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-4">
            <div class="flex flex-wrap items-center justify-between gap-2">
                <div>
                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Compose Files</p>
                    <p class="text-xs text-slate-500">Top-level source editor. Validate before save.</p>
                </div>
                <div class="flex flex-wrap gap-2">
                    <button
                        onclick={validateDraft}
                        disabled={validating}
                        class="px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700 disabled:opacity-50"
                    >
                        {validating ? "Validating..." : "Validate"}
                    </button>
                    <button
                        onclick={prettifyDraft}
                        class="px-3 py-2 rounded-lg border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700"
                    >
                        Prettify
                    </button>
                    <button
                        onclick={saveDraft}
                        disabled={saving || !hasAnyChanges}
                        class="px-3 py-2 rounded-lg bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest"
                    >
                        {saving ? "Saving..." : "Save"}
                    </button>
                </div>
            </div>

            <div class="flex flex-wrap gap-2">
                {#each project.composeFiles as file}
                    <button
                        onclick={() => activeComposePath = file.path}
                        class="px-3 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-widest border {activeComposePath === file.path ? 'bg-slate-900 text-white border-slate-900 dark:bg-slate-100 dark:text-slate-900 dark:border-slate-100' : 'bg-slate-100 dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                    >
                        {file.path.split("/").pop() || file.path}
                    </button>
                {/each}
            </div>

            {#if activeComposePath}
                <textarea
                    class="w-full min-h-[24rem] rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 p-3 text-xs font-mono text-slate-800 dark:text-slate-100"
                    value={activeComposeDraft}
                    oninput={(event) => setActiveComposeDraft((event.target as HTMLTextAreaElement).value)}
                ></textarea>
            {/if}
        </section>

        {#if project.envFile}
            <section class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                <div>
                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Project .env</p>
                    <p class="text-xs text-slate-500">{project.envFile.path}</p>
                </div>
                <textarea
                    class="w-full min-h-[12rem] rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 p-3 text-xs font-mono text-slate-800 dark:text-slate-100"
                    bind:value={envDraft}
                ></textarea>
            </section>
        {/if}

        {#if diagnostics.length > 0}
            <section class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-2">
                <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Validation Diagnostics</p>
                {#each diagnostics as d}
                    <div class="rounded-lg px-3 py-2 text-xs border {d.severity === 'error' ? 'bg-rose-50 dark:bg-rose-900/10 border-rose-200 dark:border-rose-900/40 text-rose-700 dark:text-rose-300' : 'bg-amber-50 dark:bg-amber-900/10 border-amber-200 dark:border-amber-900/40 text-amber-700 dark:text-amber-300'}">
                        <span class="font-black uppercase tracking-widest text-[9px]">{d.source}</span>
                        {#if d.path}
                            <span class="ml-2 font-mono text-[10px]">{d.path}</span>
                        {/if}
                        <p class="mt-1">{d.message}</p>
                    </div>
                {/each}
            </section>
        {/if}

        <section class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
            <div class="flex items-center justify-between">
                <p class="text-[10px] font-black uppercase tracking-widest text-slate-400">Project Containers</p>
                <span class="text-[10px] font-black uppercase tracking-widest text-slate-500">{project.members?.length || 0} Services</span>
            </div>
            <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-2">
                {#each project.members || [] as m}
                    <button
                        onclick={() => onNavigate("container-detail", { id: m.containerId, tab: "lifecycle" })}
                        class="text-left rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 px-3 py-2 hover:border-brand-400"
                    >
                        <p class="text-[10px] font-black uppercase tracking-widest text-slate-700 dark:text-slate-200">{m.serviceName || m.containerName || m.containerId}</p>
                        <p class="text-[10px] text-slate-500 truncate">{m.image || ""}</p>
                    </button>
                {/each}
            </div>
        </section>
    {/if}
</div>
