<script lang="ts">
    import { onMount } from "svelte";

    // HarborWatch Settings (Persistence via LocalStorage for now)
    let harborwatchUrl = $state(typeof localStorage !== 'undefined' ? (localStorage.getItem('hw_url') ?? window.location.origin) : "");
    let validateUrlPattern = $state(typeof localStorage !== 'undefined' ? (localStorage.getItem('hw_validate_pattern') ?? "http://localhost:18080/health") : "");
    let notifyDiscord = $state(false);
    let autoScan = $state(true);
    let aiEnabled = $state(false);

    async function loadAIStatus() {
        try {
            const res = await fetch("/api/ai/status");
            const data = await res.json();
            aiEnabled = data.enabled;
        } catch {}
    }

    onMount(() => {
        loadAIStatus();
    });

    function saveSettings() {
        if (typeof localStorage !== 'undefined') {
            localStorage.setItem('hw_url', harborwatchUrl);
            localStorage.setItem('hw_validate_pattern', validateUrlPattern);
            alert("Settings saved locally.");
        }
    }
</script>

<div class="max-w-2xl space-y-8">
    <div>
        <h2 class="text-2xl font-bold text-slate-900 dark:text-white">System Settings</h2>
        <p class="text-sm text-slate-500 mt-1">Configure global application behavior and integrations.</p>
    </div>

    <div class="space-y-6">
        <section class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-4">
            <h3 class="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                </svg>
                Connectivity
            </h3>
            
            <div class="space-y-4">
                <div class="space-y-1">
                    <label class="text-[10px] font-black uppercase text-slate-400 ml-1">Instance URL</label>
                    <input bind:value={harborwatchUrl} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-2 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
                </div>
                <div class="space-y-1">
                    <label class="text-[10px] font-black uppercase text-slate-400 ml-1">Default Validation Pattern</label>
                    <input bind:value={validateUrlPattern} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-2 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all" />
                </div>
            </div>
        </section>

        <section class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-4">
            <h3 class="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
                AI Intelligence
            </h3>
            
            <div class="flex items-center justify-between p-4 bg-slate-50 dark:bg-slate-900 rounded-xl">
                <div>
                    <span class="text-sm font-bold text-slate-700 dark:text-slate-300">AI Provider Status</span>
                    <p class="text-[10px] text-slate-500">Enable by setting <code>OPENAI_API_KEY</code> in environment.</p>
                </div>
                <span class="px-3 py-1 rounded-full text-[10px] font-black uppercase {aiEnabled ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-200 text-slate-500'}">
                    {aiEnabled ? 'Active' : 'Disabled'}
                </span>
            </div>
        </section>

        <section class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-4">
            <h3 class="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                </svg>
                Notifications (Coming Soon)
            </h3>
            
            <div class="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-900 rounded-xl opacity-50 cursor-not-allowed">
                <span class="text-sm font-medium text-slate-700 dark:text-slate-300">Enable Discord Webhooks</span>
                <div class="w-10 h-5 bg-slate-300 rounded-full relative">
                    <div class="absolute left-1 top-1 w-3 h-3 bg-white rounded-full"></div>
                </div>
            </div>
        </section>

        <section class="bg-white dark:bg-slate-800 p-6 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm space-y-4">
            <h3 class="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-brand-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
                </svg>
                Automation
            </h3>
            
            <div class="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-900 rounded-xl">
                <span class="text-sm font-medium text-slate-700 dark:text-slate-300">Periodic Security Scans</span>
                <button 
                    onclick={() => autoScan = !autoScan}
                    class="w-10 h-5 rounded-full relative transition-colors {autoScan ? 'bg-brand-600' : 'bg-slate-300'}"
                >
                    <div class="absolute top-1 w-3 h-3 bg-white rounded-full transition-all {autoScan ? 'right-1' : 'left-1'}"></div>
                </button>
            </div>
        </section>

        <div class="flex justify-end pt-4">
            <button 
                onclick={saveSettings}
                class="px-8 py-3 bg-brand-600 hover:bg-brand-700 text-white rounded-xl font-bold transition-all shadow-lg shadow-brand-500/20"
            >
                Save Configuration
            </button>
        </div>
    </div>
</div>
