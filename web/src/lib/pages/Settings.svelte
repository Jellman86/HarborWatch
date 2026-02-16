<script lang="ts">
    import { onMount } from "svelte";
    import type { Settings } from "../api-types";
    import { toasts } from "../stores/ToastStore";

    // Component State
    let settings = $state<Settings>({
        discordWebhookUrl: "",
        gotifyUrl: "",
        gotifyToken: "",
        portainerUrl: "",
        portainerApiKey: "",
        openaiKey: "",
        openaiModel: "",
        instanceUrl: "",
        validateUrlPattern: "",
        environmentOverrides: {}
    });

    let activeTab = $state("notifications");
    let saving = $state(false);

    async function loadSettings() {
        try {
            const res = await fetch("/api/settings");
            if (res.ok) {
                const data = await res.json();
                data.environmentOverrides = data.environmentOverrides || {};
                settings = data;
            }
        } catch (e) {
            console.error("Failed to load settings", e);
            toasts.error("Failed to connect to backend service.");
        }
    }

    onMount(() => {
        loadSettings();
    });

    async function saveSettings() {
        saving = true;
        try {
            const res = await fetch("/api/settings", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(settings)
            });

            if (!res.ok) throw new Error("Backend refused settings update");
            
            toasts.success("Appliance configuration synchronized.");
            await loadSettings(); // Reload to get fresh override state
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Synchronization failed");
        } finally {
            saving = false;
        }
    }

    const isLocked = (key: string) => settings.environmentOverrides?.[key] || false;
</script>

<div class="max-w-4xl space-y-8">
    <div class="flex items-center justify-between opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-6 py-2">
            <h2 class="text-3xl font-black text-slate-900 dark:text-white tracking-tighter uppercase">Appliance Configuration</h2>
            <p class="text-sm text-slate-500 font-medium mt-1">Universal control for HarborWatch intelligence and integrations.</p>
        </div>
    </div>

    <!-- Tab Navigation -->
    <div class="flex gap-1 bg-slate-100 dark:bg-slate-900/50 p-1.5 rounded-2xl w-fit border border-slate-200 dark:border-slate-800 shadow-inner opacity-0 animate-reveal stagger-1">
        {#each [
            { id: 'notifications', label: 'Notifications', icon: 'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9' },
            { id: 'keys', label: 'API Keys', icon: 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z' },
            { id: 'system', label: 'System', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z' }
        ] as tab}
            <button 
                onclick={() => activeTab = tab.id}
                class="px-6 py-2.5 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all flex items-center gap-2 {activeTab === tab.id ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-md' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={tab.icon} />
                </svg>
                {tab.label}
            </button>
        {/each}
    </div>

    <!-- Settings Content -->
    <div class="bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-xl overflow-hidden min-h-[500px] flex flex-col opacity-0 animate-reveal stagger-2">
        <div class="p-8 flex-1 space-y-8">
            {#if activeTab === 'notifications'}
                <div class="space-y-8">
                    <div class="space-y-6">
                        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-700 pb-4">
                            <div class="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center text-blue-600">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 8h2a2 2 0 012 2v6a2 2 0 01-2 2h-2v4l-4-4H9a1.994 1.994 0 01-1.414-.586m0 0L11 14h4a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2v4l.586-.586z" /></svg>
                            </div>
                            <h3 class="font-black text-slate-900 dark:text-white uppercase tracking-wider text-sm">Discord Alerts</h3>
                        </div>
                        <div class="space-y-2">
                            <label for="discord" class="text-[10px] font-black uppercase text-slate-400 ml-1">Webhook URL</label>
                            <div class="relative group">
                                <input 
                                    id="discord"
                                    type="password" 
                                    bind:value={settings.discordWebhookUrl} 
                                    disabled={isLocked('discordWebhookUrl')}
                                    placeholder={isLocked('discordWebhookUrl') ? "Managed by Environment Variable" : "https://discord.com/api/webhooks/..."}
                                    class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-5 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all disabled:opacity-60 disabled:cursor-not-allowed font-mono" 
                                />
                                {#if isLocked('discordWebhookUrl')}
                                    <div class="absolute right-4 top-1/2 -translate-y-1/2 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 text-slate-500 rounded text-[8px] font-black uppercase tracking-tighter">Locked (ENV)</div>
                                {/if}
                            </div>
                        </div>
                    </div>

                    <div class="space-y-6">
                        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-700 pb-4">
                            <div class="w-8 h-8 rounded-lg bg-emerald-500/10 flex items-center justify-center text-emerald-600">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                            </div>
                            <h3 class="font-black text-slate-900 dark:text-white uppercase tracking-wider text-sm">Gotify Notifications</h3>
                        </div>
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                            <div class="space-y-2">
                                <label for="gotify-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Server URL</label>
                                <div class="relative flex items-center">
                                    <input id="gotify-url" bind:value={settings.gotifyUrl} disabled={isLocked('gotifyUrl')} placeholder="https://gotify.example.com" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-5 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all disabled:opacity-60" />
                                    {#if isLocked('gotifyUrl')}
                                        <div class="absolute right-4 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 text-slate-500 rounded text-[8px] font-black uppercase tracking-tighter">ENV</div>
                                    {/if}
                                </div>
                            </div>
                            <div class="space-y-2">
                                <label for="gotify-token" class="text-[10px] font-black uppercase text-slate-400 ml-1">App Token</label>
                                <div class="relative flex items-center">
                                    <input id="gotify-token" type="password" bind:value={settings.gotifyToken} disabled={isLocked('gotifyToken')} placeholder="A..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-5 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all disabled:opacity-60 font-mono" />
                                    {#if isLocked('gotifyToken')}
                                        <div class="absolute right-4 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 text-slate-500 rounded text-[8px] font-black uppercase tracking-tighter">ENV</div>
                                    {/if}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            {:else if activeTab === 'keys'}
                <div class="space-y-10">
                    <div class="space-y-6">
                        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-700 pb-4">
                            <div class="w-8 h-8 rounded-lg bg-orange-500/10 flex items-center justify-center text-orange-600">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
                            </div>
                            <h3 class="font-black text-slate-900 dark:text-white uppercase tracking-wider text-sm">OpenAI Intelligence</h3>
                        </div>
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                            <div class="space-y-2 md:col-span-2">
                                <label for="openai-key" class="text-[10px] font-black uppercase text-slate-400 ml-1">API Key</label>
                                <div class="relative flex items-center">
                                    <input id="openai-key" type="password" bind:value={settings.openaiKey} disabled={isLocked('openaiKey')} placeholder="sk-..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-5 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all disabled:opacity-60 font-mono" />
                                    {#if isLocked('openaiKey')}
                                        <div class="absolute right-4 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 text-slate-500 rounded text-[8px] font-black uppercase tracking-tighter">ENV</div>
                                    {/if}
                                </div>
                            </div>
                            <div class="space-y-2">
                                <label for="openai-model" class="text-[10px] font-black uppercase text-slate-400 ml-1">Model</label>
                                <div class="relative flex items-center">
                                    <input id="openai-model" bind:value={settings.openaiModel} disabled={isLocked('openaiModel')} placeholder="gpt-4o-mini" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-5 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all disabled:opacity-60" />
                                    {#if isLocked('openaiModel')}
                                        <div class="absolute right-4 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 text-slate-500 rounded text-[8px] font-black uppercase tracking-tighter">ENV</div>
                                    {/if}
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="space-y-6">
                        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-700 pb-4">
                            <div class="w-8 h-8 rounded-lg bg-indigo-500/10 flex items-center justify-center text-indigo-600">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" /></svg>
                            </div>
                            <h3 class="font-black text-slate-900 dark:text-white uppercase tracking-wider text-sm">Portainer Integration</h3>
                        </div>
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                            <div class="space-y-2">
                                <label for="portainer-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Instance URL</label>
                                <div class="relative flex items-center">
                                    <input id="portainer-url" bind:value={settings.portainerUrl} disabled={isLocked('portainerUrl')} placeholder="https://portainer.example.com" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-5 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all disabled:opacity-60" />
                                    {#if isLocked('portainerUrl')}
                                        <div class="absolute right-4 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 text-slate-500 rounded text-[8px] font-black uppercase tracking-tighter">ENV</div>
                                    {/if}
                                </div>
                            </div>
                            <div class="space-y-2">
                                <label for="portainer-key" class="text-[10px] font-black uppercase text-slate-400 ml-1">API Key</label>
                                <div class="relative flex items-center">
                                    <input id="portainer-key" type="password" bind:value={settings.portainerApiKey} disabled={isLocked('portainerApiKey')} placeholder="ptr_..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-5 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all disabled:opacity-60 font-mono" />
                                    {#if isLocked('portainerApiKey')}
                                        <div class="absolute right-4 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 text-slate-500 rounded text-[8px] font-black uppercase tracking-tighter">ENV</div>
                                    {/if}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            {:else if activeTab === 'system'}
                <div class="space-y-8">
                    <div class="space-y-6">
                        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-700 pb-4">
                            <div class="w-8 h-8 rounded-lg bg-slate-500/10 flex items-center justify-center text-slate-600">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" /></svg>
                            </div>
                            <h3 class="font-black text-slate-900 dark:text-white uppercase tracking-wider text-sm">Universal Settings</h3>
                        </div>
                        <div class="space-y-6">
                            <div class="space-y-2">
                                <label for="instance-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Public Instance URL</label>
                                <div class="relative flex items-center">
                                    <input id="instance-url" bind:value={settings.instanceUrl} disabled={isLocked('instanceUrl')} placeholder="https://harborwatch.pownet.uk" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-5 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all disabled:opacity-60" />
                                    {#if isLocked('instanceUrl')}
                                        <div class="absolute right-4 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 text-slate-500 rounded text-[8px] font-black uppercase tracking-tighter">ENV</div>
                                    {/if}
                                </div>
                                <p class="text-[9px] text-slate-500 italic ml-1">Used for external links and webhook callbacks.</p>
                            </div>
                            <div class="space-y-2">
                                <label for="validate-pattern" class="text-[10px] font-black uppercase text-slate-400 ml-1">Default Validation URL Pattern</label>
                                <div class="relative flex items-center">
                                    <input id="validate-pattern" bind:value={settings.validateUrlPattern} disabled={isLocked('validateUrlPattern')} placeholder="http://localhost:{{PORT}}/health" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl px-5 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 transition-all disabled:opacity-60 font-mono" />
                                    {#if isLocked('validateUrlPattern')}
                                        <div class="absolute right-4 px-2 py-0.5 bg-slate-200 dark:bg-slate-700 text-slate-500 rounded text-[8px] font-black uppercase tracking-tighter">ENV</div>
                                    {/if}
                                </div>
                                <p class="text-[9px] text-slate-500 italic ml-1">Pattern to auto-generate health check URLs for updates.</p>
                            </div>
                        </div>
                    </div>
                </div>
            {/if}
        </div>

        <!-- Footer Actions -->
        <div class="px-8 py-6 bg-slate-50 dark:bg-slate-900/50 border-t border-slate-100 dark:border-slate-700 flex items-center justify-between">
            <div class="flex flex-col">
                <span class="text-[10px] text-slate-400 font-medium italic">Some settings may be read-only if defined in <code>docker-compose.yml</code>.</span>
            </div>
            <button 
                onclick={saveSettings}
                disabled={saving}
                class="px-8 py-3 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] transition-all shadow-lg shadow-brand-500/20 active:scale-95"
            >
                {saving ? 'Updating...' : 'Save Appliance Config'}
            </button>
        </div>
    </div>
</div>

<style>
    /* Custom scrollbar for the settings area */
    .flex-1::-webkit-scrollbar {
        width: 4px;
    }
    .flex-1::-webkit-scrollbar-track {
        background: transparent;
    }
    .flex-1::-webkit-scrollbar-thumb {
        @apply bg-slate-200 dark:bg-slate-700 rounded-full;
    }
</style>
