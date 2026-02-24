<script lang="ts">
    import { onMount } from "svelte";
    import { slide } from "svelte/transition";
    import { toasts } from "../stores/ToastStore";

    interface AIConversation {
        timestamp: number;
        provider: string;
        model: string;
        feature: string;
        prompt: string;
        response: string;
    }

    interface AIUsageDaily {
        day: string;
        calls: number;
        inputTokens: number;
        outputTokens: number;
        totalTokens: number;
    }

    interface AIUsageSummary {
        daily: AIUsageDaily[];
    }

    // State
    let conversations = $state<AIConversation[]>([]);
    let usageSummary = $state<AIUsageSummary | null>(null);
    let loading = $state(true);
    let expandedId = $state<number | null>(null);
    let searchQuery = $state("");
    let filterFeature = $state("");
    let pageSize = $state(25);
    let pageIndex = $state(0);
    let hasNextPage = $state(false);
    let historyInitialized = false;

    // Timeline helpers
    function buildTimeline14Days(daily: AIUsageDaily[]): AIUsageDaily[] {
        const byDay = new Map((daily || []).map((d) => [d.day, d]));
        const out: AIUsageDaily[] = [];
        const now = new Date();
        now.setHours(0, 0, 0, 0);
        for (let offset = 13; offset >= 0; offset--) {
            const d = new Date(now);
            d.setDate(now.getDate() - offset);
            const dayKey = d.toISOString().slice(0, 10);
            const existing = byDay.get(dayKey);
            out.push(existing || { day: dayKey, calls: 0, inputTokens: 0, outputTokens: 0, totalTokens: 0 });
        }
        return out;
    }

    let timelineDays = $derived(buildTimeline14Days(usageSummary?.daily || []));
    let maxCalls = $derived(Math.max(...timelineDays.map(d => d.calls), 1));
    let timelineHasActivity = $derived(timelineDays.some(d => d.calls > 0));

    function normalizeText(raw: string | null | undefined): string {
        return String(raw || "").replace(/\r\n/g, "\n");
    }

    function unwrapSingleCodeFence(raw: string): { text: string; lang: string } {
        const text = normalizeText(raw).trim();
        const m = text.match(/^```([a-zA-Z0-9_-]+)?\n([\s\S]*?)\n```$/);
        if (!m) return { text: normalizeText(raw), lang: "" };
        return { text: m[2], lang: (m[1] || "").toLowerCase() };
    }

    function prettyJson(raw: string): string | null {
        try {
            const parsed = JSON.parse(raw);
            return JSON.stringify(parsed, null, 2);
        } catch {
            return null;
        }
    }

    function looksLikeYAML(raw: string): boolean {
        const text = raw.trim();
        if (!text || text.startsWith("{") || text.startsWith("[")) return false;
        return /(^|\n)\s*[A-Za-z0-9_.-]+\s*:\s*/.test(text);
    }

    function looksLikeMarkdown(raw: string): boolean {
        const text = raw.trim();
        if (!text) return false;
        return /(^|\n)\s*(#{1,6}\s+|[-*]\s+|\d+\.\s+|>\s+)/.test(text) || text.includes("```");
    }

    function formattedContent(raw: string, role: "prompt" | "response"): { text: string; kind: "json" | "yaml" | "markdown" | "text"; mono: boolean; label: string } {
        const unwrapped = unwrapSingleCodeFence(raw);
        const base = unwrapped.text.trim() ? unwrapped.text : normalizeText(raw);
        const lang = unwrapped.lang;

        if (lang === "json" || (!lang && (base.trim().startsWith("{") || base.trim().startsWith("[")))) {
            const pretty = prettyJson(base);
            if (pretty) return { text: pretty, kind: "json", mono: true, label: "JSON" };
        }
        if (lang === "yaml" || lang === "yml" || looksLikeYAML(base)) {
            return { text: base, kind: "yaml", mono: true, label: "YAML" };
        }
        if (lang === "markdown" || looksLikeMarkdown(base)) {
            return { text: base, kind: "markdown", mono: false, label: "Markdown" };
        }
        return { text: base, kind: "text", mono: role === "prompt", label: role === "prompt" ? "Text Prompt" : "Text Response" };
    }

    async function loadData() {
        loading = true;
        try {
            const [convRes, usageRes] = await Promise.all([
                fetch(`/api/ai/conversations?limit=${pageSize}&offset=${pageIndex * pageSize}`),
                fetch("/api/ai/usage?span=30d")
            ]);
            
            if (convRes.ok) {
                const rows = await convRes.json();
                conversations = Array.isArray(rows) ? rows : [];
                hasNextPage = conversations.length === pageSize;
                expandedId = null;
            }
            if (usageRes.ok) usageSummary = await usageRes.json();
        } catch (e) {
            toasts.error("Failed to load AI history");
        } finally {
            loading = false;
        }
    }

    const filteredConvs = $derived(
        conversations.filter(c => {
            const matchesSearch = !searchQuery || 
                c.prompt.toLowerCase().includes(searchQuery.toLowerCase()) || 
                c.response.toLowerCase().includes(searchQuery.toLowerCase());
            const matchesFeature = !filterFeature || c.feature === filterFeature;
            return matchesSearch && matchesFeature;
        })
    );

    const features = $derived([...new Set(conversations.map(c => c.feature))]);

    const formatFeature = (f: string) => f.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());

    onMount(async () => {
        historyInitialized = true;
        await loadData();
    });

    $effect(() => {
        pageIndex;
        pageSize;
        if (!historyInitialized) return;
        loadData();
    });
</script>

<div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 opacity-0 animate-reveal">
        <div class="border-l-4 border-brand-600 pl-4">
            <h2 class="text-2xl font-black text-slate-900 dark:text-white uppercase tracking-tighter">AI Intelligence Logs</h2>
            <p class="text-xs text-slate-500 font-medium">Full audit trail of all prompts and automated decisions.</p>
        </div>
        <div class="flex items-center gap-2">
            <button 
                onclick={() => pageIndex = Math.max(0, pageIndex - 1)}
                disabled={pageIndex === 0 || loading}
                class="px-3 py-2 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-xl font-bold transition-all border border-slate-200 dark:border-slate-700 shadow-sm disabled:opacity-50"
            >
                Prev
            </button>
            <div class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-[10px] font-black uppercase tracking-widest text-slate-500">
                Page {pageIndex + 1}
            </div>
            <button 
                onclick={() => pageIndex = pageIndex + 1}
                disabled={!hasNextPage || loading}
                class="px-3 py-2 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-xl font-bold transition-all border border-slate-200 dark:border-slate-700 shadow-sm disabled:opacity-50"
            >
                Next
            </button>
            <button 
                onclick={loadData}
                class="px-4 py-2 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-xl font-bold transition-all border border-slate-200 dark:border-slate-700 shadow-sm flex items-center justify-center gap-2"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Refresh
            </button>
        </div>
    </div>

    <!-- Timeline / Stats -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div class="lg:col-span-2 bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 p-6 shadow-sm">
            <div class="flex items-center justify-between mb-6">
                <h3 class="text-xs font-black uppercase tracking-widest text-slate-400">Activity Timeline (14 Days)</h3>
                <span class="text-[10px] font-bold text-brand-600 uppercase">Requests per day</span>
            </div>
            
            <div class="relative flex items-end justify-between gap-1 h-32 px-2">
                {#if usageSummary}
                    {#each timelineDays as day}
                        <div class="flex-1 flex flex-col items-center gap-2 group">
                            <div 
                                class="w-full bg-brand-500/20 group-hover:bg-brand-500/40 transition-all rounded-t-sm relative"
                                style="height: {day.calls > 0 ? Math.max((day.calls / maxCalls) * 100, 6) : 4}%"
                            >
                                {#if day.calls > 0}
                                    <div class="absolute -top-6 left-1/2 -translate-x-1/2 bg-slate-900 text-white text-[9px] px-1.5 py-0.5 rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap z-10">
                                        {day.calls} calls
                                    </div>
                                {/if}
                            </div>
                            <span class="text-[8px] font-bold text-slate-400 uppercase tracking-tighter truncate w-full text-center">
                                {day.day.split('-').slice(1).join('/')}
                            </span>
                        </div>
                    {/each}
                    {#if !timelineHasActivity}
                        <div class="absolute inset-0 flex items-center justify-center pointer-events-none">
                            <div class="px-3 py-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white/90 dark:bg-slate-900/80 text-[11px] text-slate-500 italic">
                                No AI activity recorded in the last 14 days yet.
                            </div>
                        </div>
                    {/if}
                {:else}
                    <div class="w-full h-full flex items-center justify-center text-slate-400 text-xs italic">
                        Loading timeline data...
                    </div>
                {/if}
            </div>
        </div>

        <div class="bg-brand-600 rounded-3xl p-6 text-white shadow-xl shadow-brand-500/20 flex flex-col justify-between">
            <div>
                <h3 class="text-xs font-black uppercase tracking-widest opacity-80">Quick Stats</h3>
                <p class="text-[11px] mt-1 opacity-70">Appliance-wide AI metrics</p>
            </div>
            <div class="grid grid-cols-2 gap-4">
                <div>
                    <p class="text-2xl font-black">{conversations.length}</p>
                    <p class="text-[9px] font-bold uppercase tracking-widest opacity-70">Cached Logs</p>
                </div>
                <div>
                    <p class="text-2xl font-black">{features.length}</p>
                    <p class="text-[9px] font-bold uppercase tracking-widest opacity-70">Active Features</p>
                </div>
            </div>
            <div class="pt-4 border-t border-white/10">
                <p class="text-[10px] italic opacity-80">"Auditing AI ensures transparency in autonomous container lifecycle management."</p>
            </div>
        </div>
    </div>

    <!-- Filters & Search -->
    <div class="flex flex-wrap items-center gap-3 bg-white dark:bg-slate-900/40 p-4 rounded-2xl border border-slate-200 dark:border-slate-800">
        <div class="flex-1 min-w-[200px] relative">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input 
                bind:value={searchQuery}
                placeholder="Search prompt or response content..." 
                class="w-full pl-10 pr-4 py-2 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl text-xs outline-none focus:ring-2 focus:ring-brand-500"
            />
        </div>
        <select 
            bind:value={filterFeature}
            class="px-4 py-2 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl text-xs outline-none focus:ring-2 focus:ring-brand-500"
        >
            <option value="">All Features</option>
            {#each features as f}
                <option value={f}>{formatFeature(f)}</option>
            {/each}
        </select>
    </div>

    <!-- Interaction List -->
    <div class="space-y-3">
        {#if loading}
            {#each Array(3) as _}
                <div class="h-24 bg-white dark:bg-slate-800 rounded-2xl animate-pulse border border-slate-200 dark:border-slate-700"></div>
            {/each}
        {:else if filteredConvs.length === 0}
            <div class="py-20 text-center bg-white dark:bg-slate-800 rounded-3xl border-2 border-dashed border-slate-200 dark:border-slate-700">
                <p class="text-slate-400 italic">No interactions found matching your filters.</p>
            </div>
        {:else}
            {#each filteredConvs as conv, i (conv.timestamp + i)}
                <div class="bg-white dark:bg-slate-800 rounded-2xl border border-slate-200 dark:border-slate-700 shadow-sm overflow-hidden transition-all hover:border-brand-500/50">
                    <button 
                        onclick={() => expandedId = expandedId === i ? null : i}
                        class="w-full p-5 flex items-center justify-between gap-4 text-left group"
                    >
                        <div class="flex items-center gap-4 flex-1 min-w-0">
                            <div class="w-10 h-10 rounded-xl bg-slate-50 dark:bg-slate-900 flex items-center justify-center text-slate-400 group-hover:text-brand-500 transition-colors">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                                </svg>
                            </div>
                            <div class="min-w-0">
                                <div class="flex items-center gap-2">
                                    <span class="text-xs font-black text-slate-900 dark:text-white uppercase tracking-tight">{formatFeature(conv.feature)}</span>
                                    <span class="text-[10px] text-slate-400 font-mono">{new Date(conv.timestamp * 1000).toLocaleString()}</span>
                                </div>
                                <p class="text-[11px] text-slate-500 truncate mt-0.5">
                                    {conv.prompt.slice(0, 120)}...
                                </p>
                            </div>
                        </div>
                        <div class="flex items-center gap-4">
                            <div class="hidden md:flex flex-col items-end">
                                <span class="text-[9px] font-black uppercase text-slate-400 tracking-widest">{conv.provider}</span>
                                <span class="text-[10px] font-bold text-slate-600 dark:text-slate-300">{conv.model}</span>
                            </div>
                            <svg 
                                xmlns="http://www.w3.org/2000/svg" 
                                class="h-5 w-5 text-slate-300 transition-transform duration-300 {expandedId === i ? 'rotate-180' : ''}" 
                                fill="none" viewBox="0 0 24 24" stroke="currentColor"
                            >
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                            </svg>
                        </div>
                    </button>

                    {#if expandedId === i}
                        <div class="px-5 pb-5 pt-0 grid grid-cols-1 lg:grid-cols-2 gap-6" transition:slide>
                            <div class="space-y-2">
                                <div class="flex items-center justify-between px-1">
                                    <div class="flex items-center gap-2">
                                        <h4 class="text-[10px] font-black uppercase tracking-widest text-slate-400">Prompt</h4>
                                        <span class="px-1.5 py-0.5 rounded border border-slate-200 dark:border-slate-700 text-[9px] font-bold uppercase tracking-widest text-slate-500">{formattedContent(conv.prompt, "prompt").label}</span>
                                    </div>
                                    <button 
                                        onclick={() => navigator.clipboard.writeText(conv.prompt)}
                                        class="text-[9px] font-bold text-brand-600 hover:underline"
                                    >Copy</button>
                                </div>
                                <div class="bg-slate-50 dark:bg-slate-950 rounded-2xl p-4 text-[11px] {formattedContent(conv.prompt, 'prompt').mono ? 'font-mono' : 'font-sans'} text-slate-600 dark:text-slate-300 whitespace-pre-wrap border border-slate-100 dark:border-slate-800 leading-relaxed max-h-[400px] overflow-y-auto custom-scrollbar">
                                    {formattedContent(conv.prompt, "prompt").text}
                                </div>
                            </div>
                            <div class="space-y-2">
                                <div class="flex items-center justify-between px-1">
                                    <div class="flex items-center gap-2">
                                        <h4 class="text-[10px] font-black uppercase tracking-widest text-brand-500">AI Response</h4>
                                        <span class="px-1.5 py-0.5 rounded border border-brand-200/50 dark:border-brand-900/40 text-[9px] font-bold uppercase tracking-widest text-brand-600 dark:text-brand-300">{formattedContent(conv.response, "response").label}</span>
                                    </div>
                                    <button 
                                        onclick={() => navigator.clipboard.writeText(conv.response)}
                                        class="text-[9px] font-bold text-brand-600 hover:underline"
                                    >Copy</button>
                                </div>
                                <div class="bg-brand-50/30 dark:bg-brand-900/10 rounded-2xl p-4 text-[11px] {formattedContent(conv.response, 'response').mono ? 'font-mono' : 'font-sans'} text-brand-700 dark:text-brand-200 whitespace-pre-wrap border border-brand-100/50 dark:border-brand-900/20 leading-relaxed max-h-[400px] overflow-y-auto custom-scrollbar">
                                    {formattedContent(conv.response, "response").text}
                                </div>
                            </div>
                        </div>
                    {/if}
                </div>
            {/each}
        {/if}
    </div>
</div>

<style>
    .custom-scrollbar::-webkit-scrollbar {
        width: 4px;
    }
    .custom-scrollbar::-webkit-scrollbar-track {
        @apply bg-transparent;
    }
    .custom-scrollbar::-webkit-scrollbar-thumb {
        @apply bg-slate-200 dark:bg-slate-800 rounded-full;
    }
</style>
