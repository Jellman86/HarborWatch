<script lang="ts">
    import { onMount } from "svelte";

    let yaml = $state("");
    let analysis = $state("");
    let loading = $state(false);
    let error = $state("");
    let aiEnabled = $state(false);

    async function checkStatus() {
        try {
            const res = await fetch("/api/ai/status");
            const data = await res.json();
            aiEnabled = data.enabled;
        } catch {}
    }

    async function runAudit() {
        if (!yaml.trim()) return;
        
        loading = true;
        analysis = "";
        error = "";
        
        try {
            const res = await fetch("/api/ai/audit-compose", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ yaml })
            });
            
            const data = await res.json();
            if (!res.ok) throw new Error(data.message || "Audit failed");
            
            analysis = data.analysis;
        } catch (e) {
            error = e instanceof Error ? e.message : "An unexpected error occurred";
        } finally {
            loading = false;
        }
    }

    onMount(checkStatus);
</script>

<div class="space-y-6">
    <div>
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">Compose Doctor</h2>
        <p class="text-sm text-slate-500 mt-1">AI-powered security and best-practice audit for your Docker Compose files.</p>
    </div>

    {#if !aiEnabled}
        <div class="p-6 bg-amber-50 border border-amber-100 rounded-2xl">
            <h3 class="font-bold text-amber-800 flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" />
                </svg>
                AI Not Configured
            </h3>
            <p class="text-sm text-amber-700 mt-2">
                The Compose Doctor requires an AI API Key. Please set <code>OPENAI_API_KEY</code> in your environment to enable this feature.
            </p>
        </div>
    {/if}

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div class="space-y-4">
            <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden flex flex-col h-[600px]">
                <div class="p-4 border-b border-slate-100 dark:border-slate-700 bg-slate-50/50 dark:bg-slate-900/50 flex justify-between items-center">
                    <span class="text-xs font-black uppercase text-slate-400 tracking-widest">Compose Input (YAML)</span>
                </div>
                <textarea 
                    bind:value={yaml}
                    placeholder="paste your docker-compose.yml here..."
                    class="flex-1 p-6 font-mono text-xs bg-transparent outline-none resize-none dark:text-slate-300"
                ></textarea>
                <div class="p-4 border-t border-slate-100 dark:border-slate-700 flex justify-end">
                    <button 
                        onclick={runAudit}
                        disabled={!aiEnabled || loading || !yaml.trim()}
                        class="px-8 py-2 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-xl font-bold transition-all shadow-lg shadow-brand-500/20 flex items-center gap-2"
                    >
                        {#if loading}
                            <div class="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                            Analyzing...
                        {:else}
                            Run Security Audit
                        {/if}
                    </button>
                </div>
            </div>
        </div>

        <div class="space-y-4">
            <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden flex flex-col h-[600px]">
                <div class="p-4 border-b border-slate-100 dark:border-slate-700 bg-slate-50/50 dark:bg-slate-900/50">
                    <span class="text-xs font-black uppercase text-slate-400 tracking-widest">AI Analysis Results</span>
                </div>
                <div class="flex-1 p-6 overflow-y-auto prose dark:prose-invert prose-sm max-w-none">
                    {#if error}
                        <div class="p-4 bg-rose-50 border border-rose-100 text-rose-700 rounded-xl font-medium">
                            {error}
                        </div>
                    {:else if analysis}
                        <div class="whitespace-pre-wrap font-sans text-slate-600 dark:text-slate-300 leading-relaxed">
                            {analysis}
                        </div>
                    {:else}
                        <div class="flex flex-col items-center justify-center h-full text-slate-400 gap-4 opacity-40">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                            </svg>
                            <span class="font-medium italic">Ready for analysis</span>
                        </div>
                    {/if}
                </div>
            </div>
        </div>
    </div>
</div>
