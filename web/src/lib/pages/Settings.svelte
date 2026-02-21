<script lang="ts">
    import { onMount } from "svelte";
    import AutomationFlowChart from "../components/AutomationFlowChart.svelte";
    import ThemeSwitcher from "../components/ThemeSwitcher.svelte";
    import type { ContainerSummary, Settings } from "../api-types";
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

    type AutomationDomain = "general" | "upgrades" | "maintenance" | "security";
    type AIProvider = "openai" | "anthropic" | "gemini";

    interface ModelOption {
        value: string;
        label: string;
    }

    interface ClamAVSignatureStatus {
        engineVersion: string;
        databaseVersion?: string;
        databaseTimestamp?: string;
        databasePublished?: number;
        databaseDir?: string;
        databaseFiles?: string[];
        lastLocalUpdate?: number;
    }

    type AIUsageSpan = "24h" | "7d" | "30d" | "90d";

    interface AIUsageBreakdown {
        provider: string;
        model: string;
        feature: string;
        calls: number;
        inputTokens: number;
        outputTokens: number;
        totalTokens: number;
        estimatedCostUsd?: number;
    }

    interface AIUsageDaily {
        day: string;
        calls: number;
        inputTokens: number;
        outputTokens: number;
        totalTokens: number;
        estimatedCostUsd?: number;
        cumulativeCostUsd?: number;
    }

    interface AIUsageSummary {
        span: AIUsageSpan;
        from: number;
        to: number;
        calls: number;
        inputTokens: number;
        outputTokens: number;
        totalTokens: number;
        pricingConfigured: boolean;
        estimatedCostUsd?: number;
        pricingError?: string;
        breakdown: AIUsageBreakdown[];
        daily: AIUsageDaily[];
    }

    interface AIConversation {
        timestamp: number;
        provider: string;
        model: string;
        feature: string;
        prompt: string;
        response: string;
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
        aiBlockRiskThreshold: 80,
        openaiKey: "",
        openaiModel: "",
        anthropicKey: "",
        anthropicModel: "",
        geminiKey: "",
        geminiModel: "",
        aiPricingJson: "",
        instanceUrl: "",
        validateUrlPattern: "",
        uiAnimationsEnabled: true,
        automationIgnoredContainers: "harborwatch",
        malwareIgnoredMounts: "",
        autoUpgradeMaxConcurrency: 1,
        autoUpgradeMinRetryMinutes: 60,
        clamavSnapshotMaxBytes: 2147483648,
        retentionLogsDays: 30,
        retentionMetricsDays: 14,
        retentionScanResultsDays: 30,
        retentionScanJobsDays: 30,
        retentionUpdateRunsDays: 90,
        retentionComposeAuditDays: 90,
        retentionAIUsageDays: 180,
        metricsNormalized: true,
        environmentOverrides: {}
    };

    let settings = $state<Settings>({ ...defaultSettings });
    let schedules = $state<Schedule[]>([]);
    let scheduleDrafts = $state<Record<string, ScheduleDraft>>({});
    let discoveredContainers = $state<ContainerSummary[]>([]);
    let activeTab = $state("automations");
    let activeAutomationTab = $state<AutomationDomain>("general");

    let loading = $state(false);
    let saving = $state(false);
    let domainToggleBusy = $state<AutomationDomain | "">("");
    let savingScheduleId = $state("");
    let testingProvider = $state("");
    let clamavStatus = $state<ClamAVSignatureStatus | null>(null);
    let clamavStatusLoading = $state(false);
    let clamavUpdating = $state(false);
    let aiUsageSpan = $state<AIUsageSpan>("30d");
    let aiUsage = $state<AIUsageSummary | null>(null);
    let aiUsageLoading = $state(false);
    let aiUsageError = $state("");
    let aiSpendRows = $derived(Array.isArray(aiUsage?.daily) ? aiUsage.daily : []);

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
        general: {
            title: "General Settings",
            subtitle: "Global controls that apply to all automation pipelines",
            accent: "#6366f1",
            tasks: [],
            flow: ["Configure Capacity", "Manage Exclusions", "Global Safety"]
        },
        upgrades: {
            title: "Upgrade Automation",
            subtitle: "Detect updates, assess risk, and prepare safe rollouts",
            accent: "#0ea5e9",
            tasks: ["container_update_check", "container_update_apply"],
            flow: ["Discover Updates", "Policy Gate", "AI Risk Review", "Apply Update", "Health Verify", "Promote/Rollback"]
        },
        maintenance: {
            title: "Maintenance Automation",
            subtitle: "Keep host resources healthy and control data growth",
            accent: "#14b8a6",
            tasks: ["docker_system_prune", "metrics_prune", "diag_log_prune", "history_retention_prune"],
            flow: ["Measure Usage", "Prune Targets", "Reclaim Space", "Verify Capacity", "Notify Team"]
        },
        security: {
            title: "Security Automation",
            subtitle: "Continuously sweep vulnerabilities and malware",
            accent: "#f97316",
            tasks: ["security_sweep_trivy", "malware_sweep_clamav", "clamav_signature_update"],
            flow: ["Inventory Assets", "Run Trivy", "Run ClamAV", "Prioritize Findings", "Escalate Action"]
        }
    };

    function isLocked(key: string) {
        return settings.environmentOverrides?.[key] || false;
    }

    function splitDelimitedTokens(raw: string): string[] {
        return String(raw || "")
            .split(/[,;\n\r\t]+/)
            .map((token) => token.trim())
            .filter(Boolean);
    }

    function dedupeTokens(tokens: string[]): string[] {
        const seen = new Set<string>();
        const out: string[] = [];
        for (const token of tokens) {
            const normalized = token.toLowerCase();
            if (!normalized || seen.has(normalized)) continue;
            seen.add(normalized);
            out.push(token);
        }
        return out;
    }

    function containerDisplayName(container: ContainerSummary): string {
        const firstName = (container.names || []).find((name) => String(name || "").trim().length > 0);
        const normalized = String(firstName || "").replace(/^\//, "").trim();
        if (normalized) return normalized;
        return (container.id || "").slice(0, 12) || "unnamed";
    }

    function isHarborWatchContainer(container: ContainerSummary): boolean {
        const name = containerDisplayName(container).toLowerCase();
        const image = String(container.image || "").toLowerCase();
        return name.includes("harborwatch") || image.includes("harborwatch");
    }

    function containerMatchesToken(container: ContainerSummary, token: string): boolean {
        const lowered = token.trim().toLowerCase();
        if (!lowered) return false;

        const id = String(container.id || "").trim().toLowerCase();
        if (id && id.includes(lowered)) return true;

        const image = String(container.image || "").trim().toLowerCase();
        if (image && (image === lowered || image.includes(lowered))) return true;

        for (const rawName of container.names || []) {
            const name = String(rawName || "").trim().replace(/^\//, "").toLowerCase();
            if (!name) continue;
            if (name === lowered || name.includes(lowered)) return true;
        }

        const labels = container.labels || {};
        for (const value of Object.values(labels)) {
            const labelValue = String(value || "").trim().toLowerCase();
            if (!labelValue) continue;
            if (labelValue === lowered || labelValue.includes(lowered)) return true;
        }

        return false;
    }

    function containerMatchesAnyToken(container: ContainerSummary, tokens: string[]): boolean {
        return tokens.some((token) => containerMatchesToken(container, token));
    }

    function ignoredContainerTokens(): string[] {
        return dedupeTokens(splitDelimitedTokens(settings.automationIgnoredContainers || ""));
    }

    function saveIgnoredContainerTokens(tokens: string[]) {
        settings.automationIgnoredContainers = dedupeTokens(tokens).join(", ");
    }

    function isContainerIgnored(container: ContainerSummary): boolean {
        return containerMatchesAnyToken(container, ignoredContainerTokens());
    }

    function setContainerIgnored(container: ContainerSummary, ignored: boolean) {
        const current = ignoredContainerTokens();
        if (ignored) {
            if (containerMatchesAnyToken(container, current)) return;
            saveIgnoredContainerTokens([...current, container.id]);
            return;
        }
        // Remove all tokens that currently target this container.
        const next = current.filter((token) => !containerMatchesToken(container, token));
        // HarborWatch self-protection is mandatory.
        if (isHarborWatchContainer(container)) {
            next.push("harborwatch");
        }
        saveIgnoredContainerTokens(next);
    }

    function sortedContainers(containers: ContainerSummary[]): ContainerSummary[] {
        return [...containers].sort((a, b) => containerDisplayName(a).localeCompare(containerDisplayName(b)));
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
        if (domainToggleBusy) return;
        domainToggleBusy = domain;
        const scoped = schedulesForDomain(domain);
        try {
            let changed = 0;
            for (const task of scoped) {
                if (task.enabled === enabled) continue;
                await setScheduleEnabled(task.id, enabled);
                changed += 1;
            }
            if (changed === 0) {
                toasts.info(`${automationConfig[domain].title} already ${enabled ? "enabled" : "disabled"}.`);
            } else {
                toasts.success(`${enabled ? "Enabled" : "Disabled"} ${automationConfig[domain].title} (${changed} task${changed === 1 ? "" : "s"}).`);
            }
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to toggle automation domain");
        } finally {
            domainToggleBusy = "";
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

    async function loadContainersForExclusions() {
        try {
            const res = await fetch("/api/docker/containers");
            if (!res.ok) {
                discoveredContainers = [];
                return;
            }
            const data = await res.json();
            discoveredContainers = sortedContainers(Array.isArray(data) ? data : []);
        } catch {
            discoveredContainers = [];
        }
    }

    async function loadClamAVStatus() {
        clamavStatusLoading = true;
        try {
            const res = await fetch("/api/scans/malware/signatures/status");
            if (!res.ok) throw new Error(`clamav status failed (${res.status})`);
            clamavStatus = await res.json();
        } catch {
            clamavStatus = null;
        } finally {
            clamavStatusLoading = false;
        }
    }

    async function loadAIUsage() {
        aiUsageLoading = true;
        aiUsageError = "";
        try {
            const res = await fetch(`/api/ai/usage?span=${encodeURIComponent(aiUsageSpan)}`);
            const payload = await res.json().catch(() => ({}));
            if (!res.ok) {
                throw new Error(payload?.message || `ai usage failed (${res.status})`);
            }
            aiUsage = payload as AIUsageSummary;
        } catch (e) {
            aiUsage = null;
            aiUsageError = e instanceof Error ? e.message : "Failed to load AI usage";
        } finally {
            aiUsageLoading = false;
        }
    }

    async function loadAll() {
        loading = true;
        try {
            await Promise.all([
                loadSettings(),
                loadSchedules(),
                loadClamAVStatus(),
                loadContainersForExclusions(),
                loadAIUsage()
            ]);
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to load settings");
        } finally {
            loading = false;
        }
    }

    async function saveSettings() {
        saving = true;
        try {
            settings.aiBlockRiskThreshold = Math.max(0, Math.min(100, Number(settings.aiBlockRiskThreshold || 80)));
            settings.autoUpgradeMaxConcurrency = Math.max(1, Math.min(20, Number(settings.autoUpgradeMaxConcurrency || 1)));
            settings.autoUpgradeMinRetryMinutes = Math.max(1, Math.min(1440, Number(settings.autoUpgradeMinRetryMinutes || 60)));
            settings.clamavSnapshotMaxBytes = Math.max(1, Number(settings.clamavSnapshotMaxBytes || 2147483648));
            settings.retentionLogsDays = Math.max(1, Math.min(3650, Number(settings.retentionLogsDays || 30)));
            settings.retentionMetricsDays = Math.max(1, Math.min(3650, Number(settings.retentionMetricsDays || 14)));
            settings.retentionScanResultsDays = Math.max(1, Math.min(3650, Number(settings.retentionScanResultsDays || 30)));
            settings.retentionScanJobsDays = Math.max(1, Math.min(3650, Number(settings.retentionScanJobsDays || 30)));
            settings.retentionUpdateRunsDays = Math.max(1, Math.min(3650, Number(settings.retentionUpdateRunsDays || 90)));
            settings.retentionComposeAuditDays = Math.max(1, Math.min(3650, Number(settings.retentionComposeAuditDays || 90)));
            settings.retentionAIUsageDays = Math.max(1, Math.min(3650, Number(settings.retentionAIUsageDays || 180)));
            const res = await fetch("/api/settings", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(settings)
            });
            if (!res.ok) throw new Error(`settings save failed (${res.status})`);
            toasts.success("Settings saved.");
            await loadSettings();
            await loadAIUsage();
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
            if (id === "clamav_signature_update") {
                setTimeout(() => { void loadClamAVStatus(); }, 1000);
            }
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to run task");
        }
    }

    async function updateClamAVSignaturesNow() {
        clamavUpdating = true;
        try {
            const res = await fetch("/api/scans/malware/signatures/update", { method: "POST" });
            const body = await res.json().catch(() => ({}));
            if (!res.ok) throw new Error(body?.message || `signature update failed (${res.status})`);
            toasts.success("ClamAV signatures updated.");
            await Promise.all([loadSchedules(), loadClamAVStatus()]);
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "ClamAV signature update failed");
        } finally {
            clamavUpdating = false;
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
            case "container_update_apply": return "Container Auto-Apply";
            case "docker_system_prune": return "Docker System Prune";
            case "metrics_prune": return "Metrics Retention Prune";
            case "diag_log_prune": return "Diagnostics Log Prune";
            case "history_retention_prune": return "Historical Data Retention Prune";
            case "security_sweep_trivy": return "Trivy Security Sweep";
            case "malware_sweep_clamav": return "ClamAV Malware Sweep";
            case "clamav_signature_update": return "ClamAV Signature Update";
            default: return id.replace(/_/g, " ");
        }
    }

    function taskDescription(id: string): string {
        switch (id) {
            case "container_update_check":
                return "Scans registries for newer image tags and prepares upgrade candidates.";
            case "container_update_apply":
                return "Applies approved upgrades with policy gating, retry limits, and health checks.";
            case "docker_system_prune":
                return "Reclaims disk by removing eligible Docker artifacts based on prune policy.";
            case "metrics_prune":
                return "Trims old metrics to keep database growth predictable.";
            case "diag_log_prune":
                return "Deletes aged diagnostics logs after retention limits are reached.";
            case "history_retention_prune":
                return "Prunes aged scan history, update runs, compose audits, and AI usage records using configured lifecycle limits.";
            case "security_sweep_trivy":
                return "Runs Trivy vulnerability scans and records findings for image risk evaluation.";
            case "malware_sweep_clamav":
                return "Runs scheduled malware scans across in-scope container mounts.";
            case "clamav_signature_update":
                return "Refreshes ClamAV definitions so malware detections use current signatures.";
            default:
                return "Background automation task.";
        }
    }

    function aiFeatureLabel(feature: string): string {
        switch (String(feature || "").toLowerCase()) {
            case "release_analysis": return "Release Analysis";
            case "compose_audit": return "Compose Audit";
            case "metrics_analysis": return "Metrics Analysis";
            case "health_logs": return "Health Logs";
            default: return feature || "Unknown";
        }
    }

    function formatTime(ts?: number): string {
        return ts && ts > 0 ? new Date(ts * 1000).toLocaleString() : "Never";
    }

    function formatInteger(value: number | undefined): string {
        return Number(value || 0).toLocaleString();
    }

    function formatUSD(value: number | undefined): string {
        return new Intl.NumberFormat(undefined, { style: "currency", currency: "USD", maximumFractionDigits: 4 }).format(Number(value || 0));
    }

    function formatUSDCompact(value: number | undefined): string {
        return new Intl.NumberFormat(undefined, { style: "currency", currency: "USD", maximumFractionDigits: 2 }).format(Number(value || 0));
    }

    function formatBytesCompact(value: number | undefined): string {
        const bytes = Number(value || 0);
        if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
        const units = ["B", "KB", "MB", "GB", "TB"];
        let idx = 0;
        let size = bytes;
        while (size >= 1024 && idx < units.length - 1) {
            size /= 1024;
            idx += 1;
        }
        return `${size.toFixed(idx === 0 ? 0 : 2)} ${units[idx]}`;
    }

    function formatDayLabel(day: string | undefined): string {
        const raw = String(day || "").trim();
        if (!raw) return "";
        const dt = new Date(`${raw}T00:00:00Z`);
        if (Number.isNaN(dt.getTime())) return raw;
        return dt.toLocaleDateString(undefined, { month: "short", day: "numeric" });
    }

    function maxDailySpend(rows: AIUsageDaily[]): number {
        const max = rows.reduce((acc, row) => Math.max(acc, Number(row?.estimatedCostUsd || 0)), 0);
        return max > 0 ? max : 1;
    }

    function maxCumulativeSpend(rows: AIUsageDaily[]): number {
        const max = rows.reduce((acc, row) => Math.max(acc, Number(row?.cumulativeCostUsd || 0)), 0);
        return max > 0 ? max : 1;
    }

    function cumulativeLinePoints(rows: AIUsageDaily[]): string {
        if (!rows.length) return "";
        const width = 100;
        const height = 42;
        const pad = 4;
        const usableWidth = width - pad * 2;
        const usableHeight = height - pad * 2;
        const max = maxCumulativeSpend(rows);
        return rows
            .map((row, idx) => {
                const x = rows.length === 1 ? width / 2 : pad + (idx * usableWidth) / (rows.length - 1);
                const value = Number(row?.cumulativeCostUsd || 0);
                const y = pad + usableHeight - (value / max) * usableHeight;
                return `${x},${y}`;
            })
            .join(" ");
    }

    function dailyBars(rows: AIUsageDaily[]): Array<{ x: number; y: number; width: number; height: number; value: number; day: string }> {
        const chartWidth = 100;
        const barAreaTop = 6;
        const baseline = 42;
        const max = maxDailySpend(rows);
        const count = Math.max(rows.length, 1);
        const available = chartWidth / count;
        const barWidth = Math.max(1, Math.min(10, available * 0.62));
        return rows.map((row, index) => {
            const value = Number(row?.estimatedCostUsd || 0);
            const x = available * index + (available - barWidth) / 2;
            const height = Math.max(0.8, ((baseline - barAreaTop) * Math.max(0, value)) / Math.max(max, 1));
            const y = baseline - height;
            return { x, y, width: barWidth, height, value, day: String(row?.day || "") };
        });
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
                        { id: "general", label: "General" },
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
                            {#if activeAutomationTab === "general"}
                                {automationConfig[activeAutomationTab].subtitle}
                            {:else}
                                Domain status: <span class="font-bold">{domainEnabled(activeAutomationTab) ? "Enabled" : "Disabled"}</span>
                            {/if}
                        </p>
                    </div>
                    {#if activeAutomationTab !== "general"}
                        <div class="flex items-center gap-2">
                            <button
                                onclick={() => setDomainEnabled(activeAutomationTab, true)}
                                disabled={domainToggleBusy === activeAutomationTab}
                                class="px-3 py-2 rounded-xl border border-emerald-200 text-emerald-700 bg-emerald-50 dark:bg-emerald-900/20 dark:text-emerald-300 dark:border-emerald-900/40 text-[10px] font-black uppercase tracking-widest disabled:opacity-60"
                            >
                                {domainToggleBusy === activeAutomationTab ? "Applying..." : "Enable Domain"}
                            </button>
                            <button
                                onclick={() => setDomainEnabled(activeAutomationTab, false)}
                                disabled={domainToggleBusy === activeAutomationTab}
                                class="px-3 py-2 rounded-xl border border-rose-200 text-rose-700 bg-rose-50 dark:bg-rose-900/20 dark:text-rose-300 dark:border-rose-900/40 text-[10px] font-black uppercase tracking-widest disabled:opacity-60"
                            >
                                {domainToggleBusy === activeAutomationTab ? "Applying..." : "Disable Domain"}
                            </button>
                        </div>
                    {/if}
                </div>

                <div class="grid grid-cols-1 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)] gap-6">
                    <div class="space-y-4">
                        {#if activeAutomationTab !== "general"}
                            <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 bg-slate-50/60 dark:bg-slate-900/40 animate-in fade-in zoom-in duration-300">
                                <AutomationFlowChart
                                    title={automationConfig[activeAutomationTab].title}
                                    subtitle={automationConfig[activeAutomationTab].subtitle}
                                    accent={automationConfig[activeAutomationTab].accent}
                                    steps={flowSteps(activeAutomationTab)}
                                />
                                <p class="mt-2 text-xs text-slate-500">
                                    Diagram shows the ordered execution path for this domain.
                                </p>
                            </div>
                        {/if}
                        
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/25 p-4">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Task Status Overview</p>
                            <p class="mt-1 text-[11px] text-slate-500">
                                Active means scheduled tasks are enabled. Partial means some are paused. Idle means no tasks are currently scheduled for this domain.
                            </p>
                        </div>
                    </div>

                    <div class="space-y-4">
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4">
                            <p class="text-xs font-black uppercase tracking-wider text-slate-500">
                                {activeAutomationTab === "general" ? "Global Pipeline Configuration" : "Automation Task Controls"}
                            </p>
                            <p class="text-[11px] text-slate-500 mt-1">
                                {activeAutomationTab === "general" 
                                    ? "Manage system-wide throughput and safety rules that affect all background flows." 
                                    : "Use toggles to enable schedules, set cadence/time, and run on-demand checks for validation."}
                            </p>
                        </div>

                        {#if activeAutomationTab === "general"}
                            <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                                <div>
                                    <p class="text-xs font-black uppercase tracking-wider text-slate-500">Global Task Concurrency</p>
                                    <p class="text-[11px] text-slate-500 mt-1">Control how many heavy background operations (updates, scans, redeployments) can run simultaneously.</p>
                                </div>
                                <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
                                    <div class="space-y-2">
                                        <label for="global-max-concurrency" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Max Concurrent Tasks</label>
                                        <input
                                            id="global-max-concurrency"
                                            type="number"
                                            min="1"
                                            max="10"
                                            bind:value={settings.autoUpgradeMaxConcurrency}
                                            disabled={isLocked("autoUpgradeMaxConcurrency")}
                                            class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                        />
                                        <p class="text-[11px] text-slate-500">Global limit for all heavy background jobs across the appliance.</p>
                                    </div>
                                </div>
                            </div>

                            <div class="rounded-2xl border-2 border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4">
                                <div class="flex flex-wrap items-center gap-2">
                                    <span class="px-2.5 py-1 rounded-full text-[9px] font-black uppercase tracking-widest bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300">Safety Section</span>
                                    <p class="text-xs font-black uppercase tracking-wider text-slate-500">Automation Safety Exclusions</p>
                                </div>
                                <p class="text-[11px] text-slate-500 mt-2">Ignored containers are excluded from all container-scoped automations. HarborWatch is always protected and cannot be removed.</p>
                                <div class="mt-3 grid grid-cols-1 xl:grid-cols-2 gap-4">
                                    <div class="space-y-2">
                                        <p class="text-[10px] font-black uppercase tracking-wider text-slate-400">Ignored Containers</p>
                                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30 max-h-[260px] overflow-y-auto">
                                            {#if discoveredContainers.length === 0}
                                                <p class="px-3 py-3 text-[11px] text-slate-500 italic">No containers discovered. Start Docker to use auto-toggle exclusions.</p>
                                            {:else}
                                                {#each discoveredContainers as container, i (i)}
                                                    {@const ignored = isContainerIgnored(container)}
                                                    {@const protectedContainer = isHarborWatchContainer(container)}
                                                    <div class="px-3 py-2 border-b border-slate-200 dark:border-slate-800 last:border-b-0 flex items-center justify-between gap-3">
                                                        <div class="min-w-0">
                                                            <p class="text-xs font-bold text-slate-800 dark:text-slate-200 truncate">{containerDisplayName(container)}</p>
                                                            <p class="text-[10px] text-slate-500 truncate">{container.image}</p>
                                                        </div>
                                                        <button
                                                            onclick={() => setContainerIgnored(container, !ignored)}
                                                            disabled={isLocked("automationIgnoredContainers") || protectedContainer}
                                                            class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-60 {ignored ? 'bg-brand-600' : 'bg-slate-300'}"
                                                            aria-label="Toggle ignored container"
                                                            title={protectedContainer ? "HarborWatch is always excluded for self-protection" : (ignored ? "Excluded" : "Included")}
                                                        >
                                                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {ignored ? 'right-1' : 'left-1'}"></div>
                                                        </button>
                                                    </div>
                                                {/each}
                                            {/if}
                                        </div>
                                        <details class="rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/20 p-2">
                                            <summary class="cursor-pointer text-[11px] font-bold text-slate-600 dark:text-slate-300">Advanced token editor</summary>
                                            <textarea
                                                id="automation-ignore-containers"
                                                rows="3"
                                                bind:value={settings.automationIgnoredContainers}
                                                disabled={isLocked("automationIgnoredContainers")}
                                                placeholder="harborwatch, plex, qbittorrent"
                                                class="mt-2 w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                            ></textarea>
                                            <p class="mt-1 text-[11px] text-slate-500">Supports container name, image text, or ID prefix tokens (comma/newline separated).</p>
                                        </details>
                                    </div>
                                    <div class="space-y-2">
                                        <label for="malware-ignore-mounts" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Ignored Malware Mount Paths</label>
                                        <textarea
                                            id="malware-ignore-mounts"
                                            rows="3"
                                            bind:value={settings.malwareIgnoredMounts}
                                            disabled={isLocked("malwareIgnoredMounts")}
                                            placeholder="/mnt/media, /srv/plex-library"
                                            class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                        ></textarea>
                                        <p class="text-[11px] text-slate-500">These path patterns are skipped during scheduled ClamAV sweeps to avoid scanning very large media mounts.</p>
                                    </div>
                                </div>
                            </div>
                        {:else}
                            <div class="space-y-3">
                                {#if activeAutomationTab === "upgrades"}
                                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                                        <div>
                                            <p class="text-xs font-black uppercase tracking-wider text-slate-500">Upgrade Runtime Controls</p>
                                            <p class="text-[11px] text-slate-500 mt-1">Tune how aggressively auto-apply runs and how long failed containers wait before retry.</p>
                                        </div>
                                        <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
                                            <div class="space-y-2">
                                                <label for="auto-upgrade-min-retry" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Retry Cooldown (minutes)</label>
                                                <input
                                                    id="auto-upgrade-min-retry"
                                                    type="number"
                                                    min="1"
                                                    max="1440"
                                                    bind:value={settings.autoUpgradeMinRetryMinutes}
                                                    disabled={isLocked("autoUpgradeMinRetryMinutes")}
                                                    class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                                />
                                                <p class="text-[11px] text-slate-500">Minimum wait before a previously failed upgrade can be retried automatically.</p>
                                            </div>
                                        </div>
                                    </div>
                                {/if}

                                {#each schedulesForDomain(activeAutomationTab) as task, i (task.id + i)}
                                    {@const draft = draftForTask(task)}
                                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 bg-white dark:bg-slate-900/30 space-y-4">
                                        <div class="flex flex-wrap items-center justify-between gap-3">
                                            <div>
                                                <p class="text-sm font-black text-slate-800 dark:text-slate-100">{taskLabel(task.id)}</p>
                                                <p class="text-[11px] text-slate-500 mt-1">{taskDescription(task.id)}</p>
                                                <p class="text-[10px] uppercase tracking-wider text-slate-500 font-bold mt-1">{cronLabel(task.cronSpec)} | Last run: {formatTime(task.lastRun)}</p>
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
                                                <p class="text-[11px] text-slate-500">Defines how often this task is eligible to run.</p>
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
                                                <p class="text-[11px] text-slate-500">Local time used by the scheduler for this task.</p>
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
                                                <p class="text-[11px] text-slate-500">Select one or more weekdays for weekly execution.</p>
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
                                                <p class="text-[11px] text-slate-500">Select one or more month days. Tasks run on matching calendar dates.</p>
                                            </div>
                                        {/if}
                                    </div>
                                {:else}
                                    <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-700 p-6 text-sm text-slate-500 italic">
                                        No scheduler tasks found for this automation domain.
                                    </div>
                                {/each}
                            </div>
                        {/if}
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
                        <p class="text-[11px] text-slate-500">Auto mode uses the first provider that has a configured key. Set a provider explicitly to pin all AI calls to one backend.</p>
                    </div>
                </div>

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-2">
                    <label for="ai-block-risk-threshold" class="text-[10px] font-black uppercase text-slate-400 ml-1">AI Update Block Risk Threshold</label>
                    <input
                        id="ai-block-risk-threshold"
                        type="number"
                        min="0"
                        max="100"
                        bind:value={settings.aiBlockRiskThreshold}
                        disabled={isLocked("aiBlockRiskThreshold") || !settings.aiEnabled}
                        class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                    />
                    <p class="text-[11px] text-slate-500">Updates are blocked when AI release analysis risk score is greater than or equal to this value.</p>
                </div>

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                    <div class="flex flex-wrap items-center justify-between gap-3">
                        <div>
                            <p class="text-xs font-black uppercase tracking-wider text-slate-500">AI Usage</p>
                            <p class="text-[11px] text-slate-500 mt-1">Token usage is captured per request. Estimated cost appears only when pricing is configured.</p>
                        </div>
                        <div class="flex items-center gap-2">
                            <select
                                bind:value={aiUsageSpan}
                                onchange={() => loadAIUsage()}
                                class="bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                            >
                                <option value="24h">Last 24h</option>
                                <option value="7d">Last 7d</option>
                                <option value="30d">Last 30d</option>
                                <option value="90d">Last 90d</option>
                            </select>
                            <button
                                onclick={loadAIUsage}
                                disabled={aiUsageLoading}
                                class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-60"
                            >
                                {aiUsageLoading ? "Refreshing..." : "Refresh"}
                            </button>
                        </div>
                    </div>

                    {#if aiUsageError}
                        <p class="text-xs text-rose-600 dark:text-rose-300">{aiUsageError}</p>
                    {/if}

                    <div class="grid grid-cols-2 xl:grid-cols-5 gap-3">
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 p-3">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Calls</p>
                            <p class="text-sm font-bold text-slate-800 dark:text-slate-100 mt-1">{formatInteger(aiUsage?.calls)}</p>
                        </div>
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 p-3">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Input Tokens</p>
                            <p class="text-sm font-bold text-slate-800 dark:text-slate-100 mt-1">{formatInteger(aiUsage?.inputTokens)}</p>
                        </div>
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 p-3">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Output Tokens</p>
                            <p class="text-sm font-bold text-slate-800 dark:text-slate-100 mt-1">{formatInteger(aiUsage?.outputTokens)}</p>
                        </div>
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 p-3">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Total Tokens</p>
                            <p class="text-sm font-bold text-slate-800 dark:text-slate-100 mt-1">{formatInteger(aiUsage?.totalTokens)}</p>
                        </div>
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 p-3">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Estimated Cost</p>
                            <p class="text-sm font-bold text-slate-800 dark:text-slate-100 mt-1">
                                {#if aiUsage?.pricingConfigured}
                                    {formatUSD(aiUsage?.estimatedCostUsd)}
                                {:else}
                                    Not configured
                                {/if}
                            </p>
                        </div>
                    </div>

                    {#if aiSpendRows.length > 0}
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-4 space-y-3">
                            <div class="flex flex-wrap items-center justify-between gap-2">
                                <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Spend History</p>
                                <p class="text-[10px] text-slate-500">
                                    {formatDayLabel(aiSpendRows[0]?.day)} to {formatDayLabel(aiSpendRows[aiSpendRows.length - 1]?.day)}
                                </p>
                            </div>

                            {#if aiUsage?.pricingConfigured}
                                <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
                                    <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 p-3">
                                        <p class="text-[10px] font-black uppercase tracking-wider text-slate-500 mb-2">Daily Spend (USD)</p>
                                        <svg viewBox="0 0 100 44" class="w-full h-24">
                                            <line x1="2" y1="42" x2="98" y2="42" stroke="currentColor" class="text-slate-300 dark:text-slate-700" stroke-width="0.5"></line>
                                            {#each dailyBars(aiSpendRows) as bar}
                                                <rect
                                                    x={bar.x}
                                                    y={bar.y}
                                                    width={bar.width}
                                                    height={bar.height}
                                                    rx="0.6"
                                                    class="fill-brand-500/80"
                                                >
                                                    <title>{formatDayLabel(bar.day)}: {formatUSDCompact(bar.value)}</title>
                                                </rect>
                                            {/each}
                                        </svg>
                                        <div class="mt-2 flex items-center justify-between text-[10px] text-slate-500">
                                            <span>{formatDayLabel(aiSpendRows[0]?.day)}</span>
                                            <span>Max {formatUSDCompact(maxDailySpend(aiSpendRows))}</span>
                                            <span>{formatDayLabel(aiSpendRows[aiSpendRows.length - 1]?.day)}</span>
                                        </div>
                                    </div>

                                    <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 p-3">
                                        <p class="text-[10px] font-black uppercase tracking-wider text-slate-500 mb-2">Cumulative Spend (USD)</p>
                                        <svg viewBox="0 0 100 42" class="w-full h-24">
                                            <polyline
                                                points={cumulativeLinePoints(aiSpendRows)}
                                                fill="none"
                                                stroke="currentColor"
                                                class="text-brand-600 dark:text-brand-300"
                                                stroke-width="1.4"
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                            ></polyline>
                                        </svg>
                                        <div class="mt-2 flex items-center justify-between text-[10px] text-slate-500">
                                            <span>Start {formatUSDCompact(aiSpendRows[0]?.cumulativeCostUsd)}</span>
                                            <span class="font-bold">Total {formatUSDCompact(aiSpendRows[aiSpendRows.length - 1]?.cumulativeCostUsd)}</span>
                                        </div>
                                    </div>
                                </div>
                            {:else}
                                <p class="text-[11px] text-slate-500">Configure <span class="font-semibold">AI Pricing JSON</span> to render daily and cumulative spend history. Token history is already being tracked.</p>
                            {/if}
                        </div>
                    {/if}

                    {#if aiUsage?.pricingError}
                        <p class="text-[11px] text-amber-600 dark:text-amber-300">Pricing config ignored: {aiUsage.pricingError}</p>
                    {/if}

                    {#if aiUsage && aiUsage.breakdown.length > 0}
                        <div class="overflow-x-auto rounded-xl border border-slate-200 dark:border-slate-700">
                            <table class="min-w-full text-xs">
                                <thead class="bg-slate-50 dark:bg-slate-900/60">
                                    <tr class="text-left text-slate-500 uppercase tracking-wider">
                                        <th class="px-3 py-2 font-black">Provider</th>
                                        <th class="px-3 py-2 font-black">Model</th>
                                        <th class="px-3 py-2 font-black">Feature</th>
                                        <th class="px-3 py-2 font-black text-right">Calls</th>
                                        <th class="px-3 py-2 font-black text-right">Tokens</th>
                                        <th class="px-3 py-2 font-black text-right">Cost</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {#each aiUsage.breakdown as row}
                                        <tr class="border-t border-slate-200 dark:border-slate-700">
                                            <td class="px-3 py-2 font-semibold text-slate-700 dark:text-slate-200">{row.provider}</td>
                                            <td class="px-3 py-2 text-slate-600 dark:text-slate-300">{row.model}</td>
                                            <td class="px-3 py-2 text-slate-600 dark:text-slate-300">{aiFeatureLabel(row.feature)}</td>
                                            <td class="px-3 py-2 text-right text-slate-600 dark:text-slate-300">{formatInteger(row.calls)}</td>
                                            <td class="px-3 py-2 text-right text-slate-600 dark:text-slate-300">{formatInteger(row.totalTokens)}</td>
                                            <td class="px-3 py-2 text-right text-slate-600 dark:text-slate-300">
                                                {#if aiUsage.pricingConfigured}
                                                    {formatUSD(row.estimatedCostUsd)}
                                                {:else}
                                                    -
                                                {/if}
                                            </td>
                                        </tr>
                                    {/each}
                                </tbody>
                            </table>
                        </div>
                    {:else}
                        <p class="text-[11px] text-slate-500 italic">No AI usage recorded for this span yet.</p>
                    {/if}
                </div>

                <div class="space-y-2">
                    <label for="ai-pricing-json" class="text-[10px] font-black uppercase text-slate-400 ml-1">AI Pricing JSON (optional)</label>
                    <textarea
                        id="ai-pricing-json"
                        rows="5"
                        bind:value={settings.aiPricingJson}
                        disabled={isLocked("aiPricingJson")}
                        placeholder={`[\n  { "provider": "openai", "model": "gpt-5.2", "inputPer1M": 1.25, "outputPer1M": 10 },\n  { "provider": "anthropic", "model": "claude-sonnet-4-5", "inputPer1M": 3, "outputPer1M": 15 }\n]`}
                        class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                    ></textarea>
                    <p class="text-[11px] text-slate-500">If empty, HarborWatch displays token usage only. No external pricing lookup is performed.</p>
                </div>

                <div class="space-y-4">
                    <div class="flex items-center justify-between bg-white dark:bg-slate-900/30 p-6 rounded-3xl border border-slate-200 dark:border-slate-700">
                        <div class="flex items-center gap-4">
                            <div class="w-12 h-12 rounded-2xl bg-brand-500/10 flex items-center justify-center text-brand-600">
                                <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                                </svg>
                            </div>
                            <div>
                                <h3 class="text-sm font-black uppercase tracking-wider text-slate-900 dark:text-white">AI Conversation History</h3>
                                <p class="text-[11px] text-slate-500 mt-0.5">Access the full audit trail of all prompts and automated AI decisions.</p>
                            </div>
                        </div>
                        <button
                            onclick={() => onNavigate('ai-history')}
                            class="px-5 py-2.5 rounded-2xl bg-brand-600 hover:bg-brand-700 text-white text-[10px] font-black uppercase tracking-widest transition-all shadow-lg shadow-brand-500/20"
                        >
                            View Full History
                        </button>
                    </div>
                </div>

                <div class="grid grid-cols-1 xl:grid-cols-3 gap-6">
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                        <h3 class="text-sm font-black uppercase tracking-wider text-slate-500">OpenAI</h3>
                        <label for="openai-key" class="text-[10px] font-black uppercase tracking-wider text-slate-400">API Key</label>
                        <input id="openai-key" type="password" bind:value={settings.openaiKey} disabled={isLocked("openaiKey") || !settings.aiEnabled} placeholder="sk-..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <label for="openai-model" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Model</label>
                        <select id="openai-model" bind:value={settings.openaiModel} disabled={isLocked("openaiModel") || !settings.aiEnabled} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                            {#each providerModels("openai") as model}
                                <option value={model.value}>{model.label}</option>
                            {/each}
                        </select>
                        <p class="text-[11px] text-slate-500">Used for release analysis, compose review, and other AI-assisted decision points.</p>
                        <button onclick={() => testProvider("openai", settings.openaiModel || "")} disabled={testingProvider === "openai" || !settings.aiEnabled} class="w-full px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest">{testingProvider === "openai" ? "Testing..." : "Test OpenAI"}</button>
                    </div>

                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                        <h3 class="text-sm font-black uppercase tracking-wider text-slate-500">Anthropic</h3>
                        <label for="anthropic-key" class="text-[10px] font-black uppercase tracking-wider text-slate-400">API Key</label>
                        <input id="anthropic-key" type="password" bind:value={settings.anthropicKey} disabled={isLocked("anthropicKey") || !settings.aiEnabled} placeholder="sk-ant-..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <label for="anthropic-model" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Model</label>
                        <select id="anthropic-model" bind:value={settings.anthropicModel} disabled={isLocked("anthropicModel") || !settings.aiEnabled} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                            {#each providerModels("anthropic") as model}
                                <option value={model.value}>{model.label}</option>
                            {/each}
                        </select>
                        <p class="text-[11px] text-slate-500">Use this when Anthropic should be considered for AI decisions and analysis workflows.</p>
                        <button onclick={() => testProvider("anthropic", settings.anthropicModel || "")} disabled={testingProvider === "anthropic" || !settings.aiEnabled} class="w-full px-4 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest">{testingProvider === "anthropic" ? "Testing..." : "Test Anthropic"}</button>
                    </div>

                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 space-y-3">
                        <h3 class="text-sm font-black uppercase tracking-wider text-slate-500">Gemini</h3>
                        <label for="gemini-key" class="text-[10px] font-black uppercase tracking-wider text-slate-400">API Key</label>
                        <input id="gemini-key" type="password" bind:value={settings.geminiKey} disabled={isLocked("geminiKey") || !settings.aiEnabled} placeholder="AIza..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <label for="gemini-model" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Model</label>
                        <select id="gemini-model" bind:value={settings.geminiModel} disabled={isLocked("geminiModel") || !settings.aiEnabled} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60">
                            {#each providerModels("gemini") as model}
                                <option value={model.value}>{model.label}</option>
                            {/each}
                        </select>
                        <p class="text-[11px] text-slate-500">Configure to allow Google Gemini model usage inside HarborWatch AI integrations.</p>
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
                        <p class="text-[11px] text-slate-500">Incoming webhook used for outbound alerts (detections, automation events, failures).</p>
                    </div>
                    <div class="space-y-2">
                        <label for="portainer-url" class="text-[10px] font-black uppercase text-slate-400 ml-1">Portainer URL</label>
                        <input id="portainer-url" bind:value={settings.portainerUrl} disabled={isLocked("portainerUrl") || !settings.portainerEnabled} placeholder="https://portainer.example.com" class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <p class="text-[11px] text-slate-500">Base URL for Portainer API access. Used to enrich container and stack metadata.</p>
                    </div>
                    <div class="space-y-2 xl:col-span-2">
                        <label for="portainer-api-key" class="text-[10px] font-black uppercase text-slate-400 ml-1">Portainer API Key</label>
                        <input id="portainer-api-key" type="password" bind:value={settings.portainerApiKey} disabled={isLocked("portainerApiKey") || !settings.portainerEnabled} placeholder="ptr_..." class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-4 py-3 text-sm font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        <p class="text-[11px] text-slate-500">Token HarborWatch uses for authenticated Portainer calls. Keep scope limited to required read/write actions.</p>
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

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-3">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                        <div>
                            <p class="text-xs font-black uppercase tracking-wider text-slate-500">ClamAV Signatures</p>
                            <p class="text-[11px] text-slate-500 mt-1">Manage malware signature definition freshness and update cadence.</p>
                        </div>
                        <div class="flex items-center gap-2">
                            <button
                                onclick={loadClamAVStatus}
                                disabled={clamavStatusLoading}
                                class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-60"
                            >
                                {clamavStatusLoading ? "Refreshing..." : "Refresh Status"}
                            </button>
                            <button
                                onclick={updateClamAVSignaturesNow}
                                disabled={clamavUpdating}
                                class="px-3 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-60 text-white text-[10px] font-black uppercase tracking-widest"
                            >
                                {clamavUpdating ? "Updating..." : "Update Now"}
                            </button>
                            {#if scheduleById("clamav_signature_update")}
                                <button
                                    onclick={() => toggleTask("clamav_signature_update", scheduleById("clamav_signature_update")?.enabled ?? false)}
                                    class="w-10 h-5 rounded-full relative transition-colors {(scheduleById('clamav_signature_update')?.enabled ?? false) ? 'bg-brand-600' : 'bg-slate-300'}"
                                    aria-label="Toggle automated signature update"
                                >
                                    <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {(scheduleById('clamav_signature_update')?.enabled ?? false) ? 'right-1' : 'left-1'}"></div>
                                </button>
                            {/if}
                        </div>
                    </div>

                    <div class="grid grid-cols-1 xl:grid-cols-4 gap-3">
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 p-3">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Engine</p>
                            <p class="text-xs font-bold text-slate-700 dark:text-slate-200 mt-1 break-all">{clamavStatus?.engineVersion || "Unavailable"}</p>
                        </div>
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 p-3">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Signature Version</p>
                            <p class="text-xs font-bold text-slate-700 dark:text-slate-200 mt-1">{clamavStatus?.databaseVersion || "Unknown"}</p>
                        </div>
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 p-3">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Published</p>
                            <p class="text-xs font-bold text-slate-700 dark:text-slate-200 mt-1">{clamavStatus?.databasePublished ? new Date(clamavStatus.databasePublished * 1000).toLocaleString() : (clamavStatus?.databaseTimestamp || "Unknown")}</p>
                        </div>
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/50 p-3">
                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Local DB Updated</p>
                            <p class="text-xs font-bold text-slate-700 dark:text-slate-200 mt-1">{clamavStatus?.lastLocalUpdate ? new Date(clamavStatus.lastLocalUpdate * 1000).toLocaleString() : "Unknown"}</p>
                        </div>
                    </div>

                    <p class="text-[11px] text-slate-500">
                        {#if scheduleById("clamav_signature_update")}
                            Automation schedule: <span class="font-bold">{cronLabel(scheduleById("clamav_signature_update")?.cronSpec || "")}</span>.
                        {:else}
                            Signature update scheduler task is not registered.
                        {/if}
                    </p>

                    <div class="space-y-2">
                        <label for="clamav-snapshot-max-bytes" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Container Snapshot Max Bytes</label>
                        <input
                            id="clamav-snapshot-max-bytes"
                            type="number"
                            min="1"
                            bind:value={settings.clamavSnapshotMaxBytes}
                            disabled={isLocked("clamavSnapshotMaxBytes")}
                            class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                        />
                        <p class="text-[11px] text-slate-500">Current cap: <span class="font-bold">{formatBytesCompact(settings.clamavSnapshotMaxBytes)}</span>. Increase if large container mount snapshots are skipped.</p>
                    </div>
                </div>

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                    <div>
                        <p class="text-xs font-black uppercase tracking-wider text-slate-500">Data Lifecycle Retention</p>
                        <p class="text-[11px] text-slate-500 mt-1">Controls how long operational history is retained before automatic cleanup removes old rows.</p>
                    </div>
                    <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
                        <div class="space-y-2">
                            <label for="retention-logs-days" class="text-[10px] font-black uppercase tracking-wider text-slate-400">System Health Logs (days)</label>
                            <input id="retention-logs-days" type="number" min="1" bind:value={settings.retentionLogsDays} disabled={isLocked("retentionLogsDays")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        </div>
                        <div class="space-y-2">
                            <label for="retention-metrics-days" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Container Metrics (days)</label>
                            <input id="retention-metrics-days" type="number" min="1" bind:value={settings.retentionMetricsDays} disabled={isLocked("retentionMetricsDays")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        </div>
                        <div class="space-y-2">
                            <label for="retention-scan-results-days" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Security Scan Results (days)</label>
                            <input id="retention-scan-results-days" type="number" min="1" bind:value={settings.retentionScanResultsDays} disabled={isLocked("retentionScanResultsDays")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        </div>
                        <div class="space-y-2">
                            <label for="retention-scan-jobs-days" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Security Scan Jobs (days)</label>
                            <input id="retention-scan-jobs-days" type="number" min="1" bind:value={settings.retentionScanJobsDays} disabled={isLocked("retentionScanJobsDays")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        </div>
                        <div class="space-y-2">
                            <label for="retention-update-runs-days" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Upgrade Lifecycle Runs (days)</label>
                            <input id="retention-update-runs-days" type="number" min="1" bind:value={settings.retentionUpdateRunsDays} disabled={isLocked("retentionUpdateRunsDays")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        </div>
                        <div class="space-y-2">
                            <label for="retention-compose-audit-days" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Compose Audit History (days)</label>
                            <input id="retention-compose-audit-days" type="number" min="1" bind:value={settings.retentionComposeAuditDays} disabled={isLocked("retentionComposeAuditDays")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        </div>
                        <div class="space-y-2">
                            <label for="retention-ai-usage-days" class="text-[10px] font-black uppercase tracking-wider text-slate-400">AI Usage History (days)</label>
                            <input id="retention-ai-usage-days" type="number" min="1" bind:value={settings.retentionAIUsageDays} disabled={isLocked("retentionAIUsageDays")} class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60" />
                        </div>
                    </div>
                    <p class="text-[11px] text-slate-500">Lifecycle cleanup runs via <span class="font-mono">metrics_prune</span>, <span class="font-mono">diag_log_prune</span>, and <span class="font-mono">history_retention_prune</span>.</p>
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

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 flex items-center justify-between gap-4">
                    <div>
                        <p class="text-xs font-black uppercase tracking-wider text-slate-500">Normalized CPU Metrics</p>
                        <p class="text-[11px] text-slate-500 mt-1">Scale CPU usage to 100% of total system capacity. Disable to see raw per-core values (e.g. 400% for 4 cores).</p>
                    </div>
                    <button
                        onclick={() => settings.metricsNormalized = !settings.metricsNormalized}
                        disabled={isLocked("metricsNormalized")}
                        class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.metricsNormalized ? 'bg-brand-600' : 'bg-slate-300'}"
                        aria-label="Toggle CPU normalization"
                    >
                        <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.metricsNormalized ? 'right-1' : 'left-1'}"></div>
                    </button>
                </div>

                <p class="text-[11px] text-slate-500">Theme changes are applied globally across dashboards, tables, and settings views.</p>
                <ThemeSwitcher />
            </div>
        {/if}

        <div class="px-6 md:px-8 py-4 border-t border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 text-[11px] text-slate-500">
            Some fields may be read-only when defined via environment variables.
        </div>
    </div>
</div>
