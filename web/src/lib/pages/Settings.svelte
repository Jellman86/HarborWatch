<script lang="ts">
    import { onMount } from "svelte";
    import AutomationFlowChart from "../components/AutomationFlowChart.svelte";
    import ThemeSwitcher from "../components/ThemeSwitcher.svelte";
    import type { Settings } from "../api-types";
    import { toasts } from "../stores/ToastStore";

    interface Schedule {
        id: string;
        cronSpec: string;
        enabled: boolean;
        lastRun?: number;
    }

    type ScheduleCadence = "daily" | "weekly" | "monthly";

    interface ScheduleDraft {
        cadence: ScheduleCadence;
        time: string;
        weeklyDays: number[];
        monthDays: number[];
    }

    type AutomationDomain = "upgrades" | "maintenance" | "security";
    type AIProvider = "openai" | "anthropic" | "gemini";

    interface ModelOption {
        value: string;
        label: string;
    }

    const weekdayOptions: Array<{ value: number; label: string }> = [
        { value: 0, label: "Sun" },
        { value: 1, label: "Mon" },
        { value: 2, label: "Tue" },
        { value: 3, label: "Wed" },
        { value: 4, label: "Thu" },
        { value: 5, label: "Fri" },
        { value: 6, label: "Sat" }
    ];
    const monthDayOptions = Array.from({ length: 31 }, (_, idx) => idx + 1);

    const defaultSettings: Settings = {
        discordWebhookUrl: "",
        discordEnabled: true,
        portainerUrl: "",
        portainerApiKey: "",
        portainerEnabled: true,
        aiEnabled: true,
        aiProvider: "",
        openaiKey: "",
        openaiModel: "",
        anthropicKey: "",
        anthropicModel: "",
        geminiKey: "",
        geminiModel: "",
        instanceUrl: "",
        validateUrlPattern: "",
        uiAnimationsEnabled: true,
        environmentOverrides: {}
    };

    let settings = $state<Settings>({ ...defaultSettings });
    let schedules = $state<Schedule[]>([]);
    let scheduleDrafts = $state<Record<string, ScheduleDraft>>({});

    let activeTab = $state("automations");
    let activeAutomationTab = $state<AutomationDomain>("upgrades");

    let loading = $state(false);
    let saving = $state(false);
    let savingScheduleId = $state("");
    let testingProvider = $state("");

    // Latest curated model choices (validated against provider docs, February 2026).
    const latestModelsByProvider: Record<AIProvider, ModelOption[]> = {
        openai: [
            { value: "gpt-5-mini", label: "GPT-5 mini (Fast, cost-efficient)" },
            { value: "gpt-5.2", label: "GPT-5.2 (Recommended)" },
            { value: "gpt-5.2-pro", label: "GPT-5.2 Pro (Most capable)" }
        ],
        anthropic: [
            { value: "claude-haiku-4-5", label: "Claude Haiku 4.5 (Fastest)" },
            { value: "claude-sonnet-4-5", label: "Claude Sonnet 4.5 (Recommended)" },
            { value: "claude-opus-4-6", label: "Claude Opus 4.6 (Most capable)" }
        ],
        gemini: [
            { value: "gemini-2.5-flash-lite", label: "Gemini 2.5 Flash Lite (Lowest cost)" },
            { value: "gemini-2.5-flash", label: "Gemini 2.5 Flash (Recommended)" },
            { value: "gemini-2.5-pro", label: "Gemini 2.5 Pro (Most capable)" }
        ]
    };

    const automationConfig: Record<AutomationDomain, {
        title: string;
        subtitle: string;
        accent: string;
        tasks: string[];
        flow: string[];
    }> = {
        upgrades: {
            title: "Upgrade Automation",
            subtitle: "Detect updates, assess risk, and prepare safe rollouts",
            accent: "#0ea5e9",
            tasks: ["container_update_check"],
            flow: ["Discover Tags", "Risk Review", "Stage Update", "Health Verify", "Promote/Rollback"]
        },
        maintenance: {
            title: "Maintenance Automation",
            subtitle: "Keep host resources healthy and control data growth",
            accent: "#14b8a6",
            tasks: ["docker_system_prune", "metrics_prune", "diag_log_prune"],
            flow: ["Measure Usage", "Prune Targets", "Reclaim Space", "Verify Capacity", "Notify Team"]
        },
        security: {
            title: "Security Automation",
            subtitle: "Continuously sweep vulnerabilities and malware",
            accent: "#f97316",
            tasks: ["security_sweep_trivy", "malware_sweep_clamav"],
            flow: ["Inventory Assets", "Run Trivy", "Run ClamAV", "Prioritize Findings", "Escalate Action"]
        }
    };

    function isLocked(key: string) {
        return settings.environmentOverrides?.[key] || false;
    }

    function scheduleById(id: string): Schedule | undefined {
        return schedules.find((s) => s.id === id);
    }

    function schedulesForDomain(domain: AutomationDomain): Schedule[] {
        return automationConfig[domain].tasks
            .map((id) => scheduleById(id))
            .filter((v): v is Schedule => !!v);
    }

    function domainEnabled(domain: AutomationDomain): boolean {
        const scoped = schedulesForDomain(domain);
        return scoped.length > 0 && scoped.some((s) => s.enabled);
    }

    async function setScheduleEnabled(id: string, enabled: boolean) {
        const current = scheduleById(id);
        if (!current || current.enabled === enabled) return;
        const res = await fetch("/api/scheduler/toggle", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ id, enabled })
        });
        if (!res.ok) throw new Error(`toggle failed (${res.status})`);
        schedules = schedules.map((s) => (s.id === id ? { ...s, enabled } : s));
    }

    async function setDomainEnabled(domain: AutomationDomain, enabled: boolean) {
        const scoped = schedulesForDomain(domain);
        try {
            for (const task of scoped) {
                await setScheduleEnabled(task.id, enabled);
            }
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to toggle automation domain");
        }
    }

    async function toggleMetricsCollector() {
        const enabled = !(scheduleById("metrics_collector")?.enabled ?? false);
        try {
            await setScheduleEnabled("metrics_collector", enabled);
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to toggle metrics collection");
        }
    }

    function flowSteps(domain: AutomationDomain): Array<{ label: string; state: "active" | "idle" | "warning" }> {
        const scoped = schedulesForDomain(domain);
        const total = Math.max(scoped.length, 1);
        const active = scoped.filter((s) => s.enabled).length;
        return automationConfig[domain].flow.map((label, idx, arr) => {
            if (active === 0) return { label, state: "idle" as const };
            if (active >= total) return { label, state: "active" as const };
            if (idx === 0 || idx === arr.length - 1) return { label, state: "active" as const };
            return { label, state: "warning" as const };
        });
    }

    function normalizeCronSpecClient(spec: string): string {
        const fields = String(spec || "").trim().split(/\s+/).filter(Boolean);
        if (fields.length === 5) return `0 ${fields.join(" ")}`;
        if (fields.length === 6) return fields.join(" ");
        return String(spec || "").trim();
    }

    function parseCronFieldValues(field: string, min: number, max: number): number[] {
        const out = new Set<number>();
        const cleaned = String(field || "").trim();
        if (!cleaned || cleaned === "*") return [];

        const addValue = (v: number) => {
            if (v < min || v > max) return;
            out.add(v);
        };

        for (const rawPart of cleaned.split(",")) {
            const part = rawPart.trim();
            if (!part) continue;

            const [rangeExpr, stepExpr] = part.split("/");
            const step = Math.max(1, Number(stepExpr || "1") || 1);
            const range = rangeExpr || "*";

            let start = min;
            let end = max;
            if (range !== "*") {
                if (range.includes("-")) {
                    const [s, e] = range.split("-");
                    start = Number(s);
                    end = Number(e);
                } else {
                    const value = Number(range);
                    if (!Number.isFinite(value)) continue;
                    addValue(value === 7 && max === 6 ? 0 : value);
                    continue;
                }
            }

            if (!Number.isFinite(start) || !Number.isFinite(end)) continue;
            if (end < start) [start, end] = [end, start];
            for (let v = start; v <= end; v += step) {
                addValue(v === 7 && max === 6 ? 0 : v);
            }
        }

        return Array.from(out).sort((a, b) => a - b);
    }

    function parseScheduleDraft(spec: string): ScheduleDraft {
        const normalized = normalizeCronSpecClient(spec);
        const fields = normalized.split(/\s+/);
        const fallback: ScheduleDraft = {
            cadence: "daily",
            time: "00:00",
            weeklyDays: [1],
            monthDays: [1]
        };
        if (fields.length !== 6) return fallback;

        const minute = Number(fields[1]);
        const hour = Number(fields[2]);
        const safeMinute = Number.isInteger(minute) && minute >= 0 && minute <= 59 ? minute : 0;
        const safeHour = Number.isInteger(hour) && hour >= 0 && hour <= 23 ? hour : 0;
        const time = `${String(safeHour).padStart(2, "0")}:${String(safeMinute).padStart(2, "0")}`;

        const dom = fields[3];
        const dow = fields[5];

        if (dom !== "*" && dow === "*") {
            const monthDays = parseCronFieldValues(dom, 1, 31);
            return { cadence: "monthly", time, weeklyDays: [1], monthDays: monthDays.length ? monthDays : [1] };
        }
        if (dow !== "*" && dom === "*") {
            const weeklyDays = parseCronFieldValues(dow, 0, 6);
            return { cadence: "weekly", time, weeklyDays: weeklyDays.length ? weeklyDays : [1], monthDays: [1] };
        }
        return { cadence: "daily", time, weeklyDays: [1], monthDays: [1] };
    }

    function draftToCronSpec(draft: ScheduleDraft): string {
        const match = /^([01]\d|2[0-3]):([0-5]\d)$/.exec(String(draft.time || "").trim());
        const hour = match ? Number(match[1]) : 0;
        const minute = match ? Number(match[2]) : 0;
        const hh = String(hour);
        const mm = String(minute);

        if (draft.cadence === "weekly") {
            const days = Array.from(
                new Set((draft.weeklyDays || []).filter((d) => Number.isInteger(d) && d >= 0 && d <= 6))
            ).sort((a, b) => a - b);
            return `0 ${mm} ${hh} * * ${(days.length ? days : [1]).join(",")}`;
        }
        if (draft.cadence === "monthly") {
            const days = Array.from(
                new Set((draft.monthDays || []).filter((d) => Number.isInteger(d) && d >= 1 && d <= 31))
            ).sort((a, b) => a - b);
            return `0 ${mm} ${hh} ${(days.length ? days : [1]).join(",")} * *`;
        }
        return `0 ${mm} ${hh} * * *`;
    }

    function syncScheduleDrafts() {
        const next: Record<string, ScheduleDraft> = {};
        for (const schedule of schedules) {
            next[schedule.id] = parseScheduleDraft(schedule.cronSpec);
        }
        scheduleDrafts = next;
    }

    function draftForTask(task: Schedule): ScheduleDraft {
        return scheduleDrafts[task.id] ?? parseScheduleDraft(task.cronSpec);
    }

    function patchScheduleDraft(id: string, patch: Partial<ScheduleDraft>) {
        const schedule = scheduleById(id);
        const current = scheduleDrafts[id] ?? parseScheduleDraft(schedule?.cronSpec || "0 0 0 * * *");
        scheduleDrafts = { ...scheduleDrafts, [id]: { ...current, ...patch } };
    }

    function toggleWeeklyDay(id: string, day: number) {
        const current = scheduleDrafts[id] ?? parseScheduleDraft(scheduleById(id)?.cronSpec || "0 0 0 * * *");
        const exists = current.weeklyDays.includes(day);
        const next = exists ? current.weeklyDays.filter((d) => d !== day) : [...current.weeklyDays, day];
        patchScheduleDraft(id, { weeklyDays: next.length ? next : [1] });
    }

    function toggleMonthDay(id: string, day: number) {
        const current = scheduleDrafts[id] ?? parseScheduleDraft(scheduleById(id)?.cronSpec || "0 0 0 * * *");
        const exists = current.monthDays.includes(day);
        const next = exists ? current.monthDays.filter((d) => d !== day) : [...current.monthDays, day];
        patchScheduleDraft(id, { monthDays: next.length ? next : [1] });
    }

    function scheduleDirty(task: Schedule): boolean {
        const draft = scheduleDrafts[task.id];
        if (!draft) return false;
        return draftToCronSpec(draft) !== normalizeCronSpecClient(task.cronSpec);
    }

    async function loadSettings() {
        const res = await fetch("/api/settings");
        if (!res.ok) throw new Error(`settings read failed (${res.status})`);
        const data = await res.json();
        data.environmentOverrides = data.environmentOverrides || {};
        settings = { ...defaultSettings, ...data };
    }

    async function loadSchedules() {
        const res = await fetch("/api/scheduler/schedules");
        if (!res.ok) throw new Error(`schedules read failed (${res.status})`);
        schedules = await res.json();
        syncScheduleDrafts();
    }

    async function loadAll() {
        loading = true;
        try {
            await Promise.all([loadSettings(), loadSchedules()]);
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to load settings");
        } finally {
            loading = false;
        }
    }

    async function saveSettings() {
        saving = true;
        try {
            const res = await fetch("/api/settings", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(settings)
            });
            if (!res.ok) throw new Error(`settings save failed (${res.status})`);
            toasts.success("Settings saved.");
            await loadSettings();
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to save settings");
        } finally {
            saving = false;
        }
    }

    async function toggleTask(id: string, enabled: boolean) {
        try {
            await setScheduleEnabled(id, !enabled);
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to toggle automation task");
        }
    }

    async function runTask(id: string) {
        try {
            const res = await fetch("/api/scheduler/run", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ id })
            });
            if (!res.ok) throw new Error(`run failed (${res.status})`);
            toasts.success(`Task ${taskLabel(id)} triggered.`);
            await loadSchedules();
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to run task");
        }
    }

    async function saveTaskSchedule(id: string) {
        const draft = scheduleDrafts[id];
        if (!draft) return;
        const cronSpec = draftToCronSpec(draft);
        savingScheduleId = id;
        try {
            const res = await fetch("/api/scheduler/update", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ id, cronSpec })
            });
            if (!res.ok) {
                const body = await res.json().catch(() => ({}));
                throw new Error(body?.message || `schedule update failed (${res.status})`);
            }
            schedules = schedules.map((s) => (s.id === id ? { ...s, cronSpec } : s));
            scheduleDrafts = { ...scheduleDrafts, [id]: parseScheduleDraft(cronSpec) };
            toasts.success(`Schedule updated for ${taskLabel(id)}.`);
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to update schedule");
        } finally {
            savingScheduleId = "";
        }
    }

    function providerModels(provider: AIProvider): ModelOption[] {
        return latestModelsByProvider[provider];
    }

    function normalizeModel(provider: AIProvider, model: string | undefined): string {
        const options = providerModels(provider);
        if (options.length === 0) return "";
        const normalized = String(model || "").trim();
        return options.some((opt) => opt.value === normalized) ? normalized : options[0].value;
    }

    async function testProvider(provider: "openai" | "anthropic" | "gemini", model: string) {
        testingProvider = provider;
        try {
            const res = await fetch("/api/ai/test", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ provider, model })
            });
            const body = await res.json().catch(() => ({}));
            if (!res.ok) throw new Error(body?.message || `AI test failed (${res.status})`);
            toasts.success(`AI test passed: ${provider} (${model || "default"})`);
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "AI test failed");
        } finally {
            testingProvider = "";
        }
    }

    function cronLabel(spec: string): string {
        const draft = parseScheduleDraft(spec);
        if (draft.cadence === "weekly") {
            const labels = draft.weeklyDays
                .map((day) => weekdayOptions.find((opt) => opt.value === day)?.label || String(day))
                .join(", ");
            return `Weekly (${labels || "Mon"} ${draft.time})`;
        }
        if (draft.cadence === "monthly") {
            const days = draft.monthDays.join(", ");
            return `Monthly (Day ${days || "1"} ${draft.time})`;
        }
        return `Daily (${draft.time})`;
    }

    function taskLabel(id: string): string {
        switch (id) {
            case "container_update_check": return "Container Update Check";
            case "docker_system_prune": return "Docker System Prune";
            case "metrics_prune": return "Metrics Retention Prune";
            case "diag_log_prune": return "Diagnostics Log Prune";
            case "security_sweep_trivy": return "Trivy Security Sweep";
            case "malware_sweep_clamav": return "ClamAV Malware Sweep";
            default: return id.replace(/_/g, " ");
        }
    }

    function formatTime(ts?: number): string {
        return ts && ts > 0 ? new Date(ts * 1000).toLocaleString() : "Never";
    }

    $effect(() => {
        settings.openaiModel = normalizeModel("openai", settings.openaiModel);
        settings.anthropicModel = normalizeModel("anthropic", settings.anthropicModel);
        settings.geminiModel = normalizeModel("gemini", settings.geminiModel);
    });

    $effect(() => {
        if (typeof document === "undefined") return;
        document.documentElement.classList.toggle("no-motion", !settings.uiAnimationsEnabled);
    });

    onMount(() => {
        loadAll();
    });
</script>

<div class="w-full space-y-8">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
            <h2 class="text-3xl font-black text-slate-900 dark:text-white tracking-tight">Settings</h2>
            <p class="text-sm text-slate-500 mt-1">Global configuration and automation policy control plane.</p>
        </div>
        <button
            onclick={saveSettings}
            disabled={saving || loading}
            class="px-6 py-3 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-brand-500/20"
        >
            {saving ? "Saving..." : "Save Settings"}
        </button>
    </div>

    <div class="flex flex-wrap gap-2 bg-slate-100 dark:bg-slate-900/50 p-1.5 rounded-2xl border border-slate-200 dark:border-slate-800 w-fit">
        {#each [
            { id: "automations", label: "Automations" },
            { id: "ai", label: "AI" },
            { id: "integrations", label: "Integrations" },
            { id: "system", label: "System" },
            { id: "appearance", label: "Appearance" }
        ] as tab}
            <button
                onclick={() => activeTab = tab.id}
                class="px-5 py-2.5 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all {activeTab === tab.id ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
            >
                {tab.label}
            </button>
        {/each}
    </div>

    <div class="w-full bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm min-h-[620px]">
        {#if loading}
            <div class="p-10 text-sm text-slate-500">Loading settings...</div>
        {:else if activeTab === "automations"}
            <div class="p-6 md:p-8 space-y-6">
                <div class="flex flex-wrap gap-2 bg-slate-100 dark:bg-slate-900/50 p-1.5 rounded-2xl border border-slate-200 dark:border-slate-700 w-fit">
                    {#each [
                        { id: "upgrades", label: "Upgrades" },
                        { id: "maintenance", label: "Maintenance" },
                        { id: "security", label: "Security" }
                    ] as tab}
                        <button
                            onclick={() => activeAutomationTab = tab.id as AutomationDomain}
                            class="px-4 py-2 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all {activeAutomationTab === tab.id ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
                        >
                            {tab.label}
                        </button>
                    {/each}
                </div>

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 flex flex-wrap items-center justify-between gap-3">
                    <div>
                        <p class="text-xs font-black uppercase tracking-wider text-slate-500">{automationConfig[activeAutomationTab].title}</p>
                        <p class="text-[11px] text-slate-500 mt-1">
                            Domain status: <span class="font-bold">{domainEnabled(activeAutomationTab) ? "Enabled" : "Disabled"}</span>
                        </p>
                    </div>
                    <div class="flex items-center gap-2">
                        <button
                            onclick={() => setDomainEnabled(activeAutomationTab, true)}
                            class="px-3 py-2 rounded-xl border border-emerald-200 text-emerald-700 bg-emerald-50 dark:bg-emerald-900/20 dark:text-emerald-300 dark:border-emerald-900/40 text-[10px] font-black uppercase tracking-widest"
                        >
                            Enable Domain
                        </button>
                        <button
                            onclick={() => setDomainEnabled(activeAutomationTab, false)}
                            class="px-3 py-2 rounded-xl border border-rose-200 text-rose-700 bg-rose-50 dark:bg-rose-900/20 dark:text-rose-300 dark:border-rose-900/40 text-[10px] font-black uppercase tracking-widest"
                        >
                            Disable Domain
                        </button>
                    </div>
                </div>

                <div class="space-y-6">
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 bg-slate-50/60 dark:bg-slate-900/40">
                        <AutomationFlowChart
                            title={automationConfig[activeAutomationTab].title}
                            subtitle={automationConfig[activeAutomationTab].subtitle}
                            accent={automationConfig[activeAutomationTab].accent}
                            steps={flowSteps(activeAutomationTab)}
                        />
                        <p class="mt-2 text-xs text-slate-500">
                            Global policy is managed by scheduler tasks. Containers can inherit this policy or override it in container lifecycle settings.
                        </p>
                    </div>

                    <div class="grid grid-cols-1 xl:grid-cols-2 gap-3">
                        {#each schedulesForDomain(activeAutomationTab) as task}
                            {@const draft = draftForTask(task)}
                            <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 bg-white dark:bg-slate-900/30 space-y-4">
                                <div class="flex flex-wrap items-center justify-between gap-3">
                                    <div>
                                        <p class="text-sm font-black text-slate-800 dark:text-slate-100">{taskLabel(task.id)}</p>
                                        <p class="text-[10px] uppercase tracking-wider text-slate-500 font-bold">{cronLabel(task.cronSpec)} | Last run: {formatTime(task.lastRun)}</p>
                                    </div>
                                    <div class="flex items-center gap-2">
                                        <button
                                            onclick={() => runTask(task.id)}
                                            class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800"
                                        >Run Now</button>
                                        <button
                                            onclick={() => toggleTask(task.id, task.enabled)}
                                            class="w-10 h-5 rounded-full relative transition-colors {task.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                                            aria-label="Toggle task"
                                        >
                                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {task.enabled ? 'right-1' : 'left-1'}"></div>
                                        </button>
                                    </div>
                                </div>

                                <div class="grid grid-cols-1 md:grid-cols-[1fr_1fr_auto] gap-3 items-end">
                                    <div class="space-y-1">
                                        <label for={"cadence-" + task.id} class="text-[10px] font-black uppercase tracking-wider text-slate-400">Cadence</label>
                                        <select
                                            id={"cadence-" + task.id}
                                            value={draft.cadence}
                                            onchange={(e) => patchScheduleDraft(task.id, { cadence: (e.currentTarget as HTMLSelectElement).value as ScheduleCadence })}
                                            class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                        >
                                            <option value="daily">Daily</option>
                                            <option value="weekly">Weekly</option>
                                            <option value="monthly">Monthly</option>
                                        </select>
                                    </div>

                                    <div class="space-y-1">
                                        <label for={"time-" + task.id} class="text-[10px] font-black uppercase tracking-wider text-slate-400">Run Time</label>
                                        <input
                                            id={"time-" + task.id}
                                            type="time"
                                            value={draft.time}
                                            onchange={(e) => patchScheduleDraft(task.id, { time: (e.currentTarget as HTMLInputElement).value || "00:00" })}
                                            class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                        />
                                    </div>

                                    <button
                                        onclick={() => saveTaskSchedule(task.id)}
                                        disabled={!scheduleDirty(task) || savingScheduleId === task.id}
                                        class="px-3 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest"
                                    >
                                        {savingScheduleId === task.id ? "Saving..." : "Save Schedule"}
                                    </button>
                                </div>

                                {#if draft.cadence === "weekly"}
                                    <div class="space-y-1">
                                        <p class="text-[10px] font-black uppercase tracking-wider text-slate-400">Run On Days</p>
                                        <div class="flex flex-wrap gap-2">
                                            {#each weekdayOptions as day}
                                                <button
                                                    onclick={() => toggleWeeklyDay(task.id, day.value)}
                                                    class="px-2.5 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-wider border transition-colors {draft.weeklyDays.includes(day.value) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                                >
                                                    {day.label}
                                                </button>
                                            {/each}
                                        </div>
                                    </div>
                                {:else if draft.cadence === "monthly"}
                                    <div class="space-y-1">
                                        <p class="text-[10px] font-black uppercase tracking-wider text-slate-400">Run On Dates</p>
                                        <div class="flex flex-wrap gap-1.5">
                                            {#each monthDayOptions as day}
                                                <button
                                                    onclick={() => toggleMonthDay(task.id, day)}
                                                    class="min-w-8 px-2 py-1 rounded-lg text-[10px] font-black border transition-colors {draft.monthDays.includes(day) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                                >
                                                    {day}
                                                </button>
                                            {/each}
                                        </div>
                                    </div>
                                {/if}
                            </div>
                        {:else}
                            <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-700 p-6 text-sm text-slate-500 italic">
                                No scheduler tasks found for this automation domain.
                            </div>
                        {/each}
                    </div>
                </div>

            </div>

        {:else if activeTab === "ai"}
            <div class="p-6 md:p-8 space-y-8">
                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 flex items-center justify-between gap-4">
                    <div>
                        <p class="text-xs font-black uppercase tracking-wider text-slate-500">AI Features</p>
                        <p class="text-[11px] text-slate-500 mt-1">Disable this to fully turn off AI analysis and provider usage.</p>
                    </div>
                    <button
                        onclick={() => settings.aiEnabled = !settings.aiEnabled}
                        disabled={isLocked("aiEnabled")}
                        class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.aiEnabled ? 'bg-brand-600' : 'bg-slate-300'}"
                        aria-label="Toggle AI features"
                    >
                        <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.aiEnabled ? 'right-1' : 'left-1'}"></div>
                    </button>
                </div>

                <div class="flex flex-wrap items-end gap-4">
                    <div class="space-y-2 min-w-[260px]">
                        <label for="ai-provider" class="text-[10px] font-black uppercase text-slate-400 ml-1">Preferred Provider</label>
                        <select id="ai-provider" bind:value={settings.aiProvider} disabled={isLocked("aiProvider") || !settings.aiEnabled} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                            <option value="">Auto (first configured)</option>
                            <option value="openai">OpenAI</option>
                            <option value="anthropic">Anthropic</option>
                            <option value="gemini">Gemini</option>
                        </select>
                    </div>
                </div>

                <div class="grid grid-cols-1 xl:grid-cols-3 gap-6">
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                        <h3 class="text-sm font-black uppercase tracking-wider text-slate-500">OpenAI</h3>
                        <input type="password" bind:value={settings.openaiKey} disabled={isLocked("openaiKey") || !settings.aiEnabled} placeholder="sk-..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <select bind:value={settings.openaiModel} disabled={isLocked("openaiModel") || !settings.aiEnabled} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                            {#each providerModels("openai") as model}
                                <option value={model.value}>{model.label}</option>
                            {/each}
                        </select>
                        <button onclick={() => testProvider("openai", settings.openaiModel || "")} disabled={testingProvider === "openai" || !settings.aiEnabled} class="w-full px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest">{testingProvider === "openai" ? "Testing..." : "Test OpenAI"}</button>
                    </div>

                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                        <h3 class="text-sm font-black uppercase tracking-wider text-slate-500">Anthropic</h3>
                        <input type="password" bind:value={settings.anthropicKey} disabled={isLocked("anthropicKey") || !settings.aiEnabled} placeholder="sk-ant-..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <select bind:value={settings.anthropicModel} disabled={isLocked("anthropicModel") || !settings.aiEnabled} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                            {#each providerModels("anthropic") as model}
                                <option value={model.value}>{model.label}</option>
                            {/each}
                        </select>
                        <button onclick={() => testProvider("anthropic", settings.anthropicModel || "")} disabled={testingProvider === "anthropic" || !settings.aiEnabled} class="w-full px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest">{testingProvider === "anthropic" ? "Testing..." : "Test Anthropic"}</button>
                    </div>

                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                        <h3 class="text-sm font-black uppercase tracking-wider text-slate-500">Gemini</h3>
                        <input type="password" bind:value={settings.geminiKey} disabled={isLocked("geminiKey") || !settings.aiEnabled} placeholder="AIza..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <select bind:value={settings.geminiModel} disabled={isLocked("geminiModel") || !settings.aiEnabled} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                            {#each providerModels("gemini") as model}
                                <option value={model.value}>{model.label}</option>
                            {/each}
                        </select>
                        <button onclick={() => testProvider("gemini", settings.geminiModel || "")} disabled={testingProvider === "gemini" || !settings.aiEnabled} class="w-full px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest">{testingProvider === "gemini" ? "Testing..." : "Test Gemini"}</button>
                    </div>
                </div>
            </div>

        {:else if activeTab === "integrations"}
            <div class="p-6 md:p-8 space-y-8">
                <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 flex items-center justify-between gap-3">
                        <div>
                            <p class="text-xs font-black uppercase tracking-wider text-slate-500">Discord Notifications</p>
                            <p class="text-[11px] text-slate-500 mt-1">Enable or disable outbound Discord alerts without deleting credentials.</p>
                        </div>
                        <button
                            onclick={() => settings.discordEnabled = !settings.discordEnabled}
                            disabled={isLocked("discordEnabled")}
                            class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.discordEnabled ? 'bg-brand-600' : 'bg-slate-300'}"
                            aria-label="Toggle Discord notifications"
                        >
                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.discordEnabled ? 'right-1' : 'left-1'}"></div>
                        </button>
                    </div>

                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 flex items-center justify-between gap-3">
                        <div>
                            <p class="text-xs font-black uppercase tracking-wider text-slate-500">Portainer Integration</p>
                            <p class="text-[11px] text-slate-500 mt-1">Control whether HarborWatch should query Portainer APIs.</p>
                        </div>
                        <button
                            onclick={() => settings.portainerEnabled = !settings.portainerEnabled}
                            disabled={isLocked("portainerEnabled")}
                            class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.portainerEnabled ? 'bg-brand-600' : 'bg-slate-300'}"
                            aria-label="Toggle Portainer integration"
                        >
                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.portainerEnabled ? 'right-1' : 'left-1'}"></div>
                        </button>
                    </div>
                </div>

                <div class="grid grid-cols-1 xl:grid-cols-2 gap-6">
                    <div class="space-y-2">
                        <label for="discord-webhook" class="text-[10px] font-black uppercase text-slate-400 ml-1">Discord Webhook</label>
                        <input id="discord-webhook" type="password" bind:value={settings.discordWebhookUrl} disabled={isLocked("discordWebhookUrl") || !settings.discordEnabled} placeholder="https://discord.com/api/webhooks/..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    </div>
                    <div class="space-y-2">
                        <label for="portainer-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Portainer URL</label>
                        <input id="portainer-url" bind:value={settings.portainerUrl} disabled={isLocked("portainerUrl") || !settings.portainerEnabled} placeholder="https://portainer.example.com" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    </div>
                    <div class="space-y-2 xl:col-span-2">
                        <label for="portainer-api-key" class="text-[10px] font-black uppercase text-slate-400 ml-1">Portainer API Key</label>
                        <input id="portainer-api-key" type="password" bind:value={settings.portainerApiKey} disabled={isLocked("portainerApiKey") || !settings.portainerEnabled} placeholder="ptr_..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                    </div>
                </div>
            </div>

        {:else if activeTab === "system"}
            <div class="p-6 md:p-8 space-y-6">
                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 flex items-center justify-between gap-4">
                    <div>
                        <p class="text-xs font-black uppercase tracking-wider text-slate-500">Metrics Collection</p>
                        <p class="text-[11px] text-slate-500 mt-1">Control background `metrics_collector` scheduler activity.</p>
                    </div>
                    <button
                        onclick={toggleMetricsCollector}
                        disabled={!scheduleById("metrics_collector")}
                        class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {(scheduleById('metrics_collector')?.enabled ?? false) ? 'bg-brand-600' : 'bg-slate-300'}"
                        aria-label="Toggle metrics collector"
                    >
                        <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {(scheduleById('metrics_collector')?.enabled ?? false) ? 'right-1' : 'left-1'}"></div>
                    </button>
                </div>

                <div class="grid grid-cols-1 xl:grid-cols-2 gap-6">
                    <div class="space-y-2">
                        <label for="instance-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Instance URL</label>
                        <input id="instance-url" bind:value={settings.instanceUrl} disabled={isLocked("instanceUrl")} placeholder="https://harborwatch.example.com" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <p class="text-[11px] text-slate-500">Used for webhook callbacks and external links.</p>
                    </div>
                    <div class="space-y-2">
                        <label for="validate-pattern" class="text-[10px] font-black uppercase text-slate-400 ml-1">Validation URL Pattern</label>
                        <input id="validate-pattern" bind:value={settings.validateUrlPattern} disabled={isLocked("validateUrlPattern")} placeholder={"http://localhost:{{PORT}}/health"} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <p class="text-[11px] text-slate-500">Template for deriving per-container validation URLs.</p>
                    </div>
                </div>
            </div>

        {:else if activeTab === "appearance"}
            <div class="p-6 md:p-8 space-y-6">
                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 flex items-center justify-between gap-4">
                    <div>
                        <p class="text-xs font-black uppercase tracking-wider text-slate-500">UI Animations</p>
                        <p class="text-[11px] text-slate-500 mt-1">Turn off transitions and motion effects for reduced visual movement.</p>
                    </div>
                    <button
                        onclick={() => settings.uiAnimationsEnabled = !settings.uiAnimationsEnabled}
                        disabled={isLocked("uiAnimationsEnabled")}
                        class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.uiAnimationsEnabled ? 'bg-brand-600' : 'bg-slate-300'}"
                        aria-label="Toggle UI animations"
                    >
                        <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.uiAnimationsEnabled ? 'right-1' : 'left-1'}"></div>
                    </button>
                </div>
                <ThemeSwitcher />
            </div>
        {/if}

        <div class="px-6 md:px-8 py-4 border-t border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 text-[11px] text-slate-500">
            Some fields may be read-only when defined via environment variables.
        </div>
    </div>
</div>
