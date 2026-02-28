<script lang="ts">
    import { onMount } from "svelte";
    import AutomationFlowChart from "../components/AutomationFlowChart.svelte";
    import ThemeSwitcher from "../components/ThemeSwitcher.svelte";
    import type { ContainerSummary, Settings } from "../api-types";
    import { toasts } from "../stores/ToastStore";
    import { configStore } from "../stores/config.svelte";

    let { onNavigate } = $props<{
        onNavigate: (route: string, params?: any) => void;
    }>();

    interface Schedule {
        id: string;
        cronSpec: string;
        enabled: boolean;
        lastRun?: number;
    }

    type ScheduleCadence = "daily" | "weekly" | "monthly";
    type UpdateCheckIntervalOption = "10m" | "30m" | "60m" | "5h" | "12h" | "24h";

    interface ScheduleDraft {
        cadence: ScheduleCadence;
        time: string;
        weeklyDays: number[];
        monthDays: number[];
    }

    type AutomationDomain = "general" | "upgrades" | "maintenance" | "security" | "remediation";
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
    type RetentionWindowPreset = "7d" | "30d" | "90d" | "365d";

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
    const updateCheckIntervalOptions: Array<{ value: UpdateCheckIntervalOption; label: string }> = [
        { value: "10m", label: "Every 10 minutes" },
        { value: "30m", label: "Every 30 minutes" },
        { value: "60m", label: "Every 60 minutes" },
        { value: "5h", label: "Every 5 hours" },
        { value: "12h", label: "Every 12 hours" },
        { value: "24h", label: "Every 24 hours" }
    ];

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
        trivySweepMode: "running-only",
        dockerPruneIncludeUnusedTaggedImages: false,
        composeSnapshotRootPath: "",
        gitOpsMasterDirectory: "/data/gitops",
        clamavSnapshotMaxBytes: 2147483648,
        dataRetentionDays: 30,
        retentionLogsDays: 30,
        retentionMetricsDays: 14,
        retentionScanResultsDays: 30,
        retentionScanJobsDays: 30,
        retentionUpdateRunsDays: 90,
        retentionComposeAuditDays: 90,
        retentionAIUsageDays: 180,
        metricsNormalized: true,
        globalBypassAi: false,
        globalSkipHealthCheck: false,
        defaultValidateMode: "both",
        defaultValidateTimeoutSec: 45,
        defaultValidateIntervalSec: 2,
        defaultAiValidateLogs: false,
        defaultAutoRollback: true,
        defaultRestartOnUnhealthy: false,
        unhealthyAutoRemediationEnabled: true,
        unhealthyRestartCooldownSecDefault: 300,
        maxRestartsPerWindow: 3,
        environmentOverrides: {}
    };

    let settings = $state<Settings>({ ...defaultSettings });
    let settingsSavedSignature = $state("");
    let settingsSaveErrorSignature = $state("");
    let schedules = $state<Schedule[]>([]);
    let scheduleDrafts = $state<Record<string, ScheduleDraft>>({});
    let updateCheckIntervalDrafts = $state<Record<string, UpdateCheckIntervalOption>>({});
    let discoveredContainers = $state<ContainerSummary[]>([]);
    let activeTab = $state("automations");
    let activeAutomationTab = $state<AutomationDomain>("general");
    let activeAITab = $state<"settings" | "costs">("settings");

    let loading = $state(false);
    let saving = $state(false);
    let bulkUpdating = $state(false);
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
    let retentionWindowPreset = $state<RetentionWindowPreset>("30d");

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
        },
        remediation: {
            title: "Self-Healing & Remediation",
            subtitle: "Automatically recover from container health failures",
            accent: "#ec4899",
            tasks: [],
            flow: ["Listen for Events", "Verify Rules", "Cooldown Check", "Restart Container", "Log Outcome"]
        }
    };

    const settingsTabChrome: Record<string, { title: string; subtitle: string; accentClass: string; badge: string }> = {
        automations: {
            title: "Automation Control Plane",
            subtitle: "Orchestrate upgrade, maintenance, security, and remediation behavior with explicit scheduling and safety guardrails.",
            accentClass: "from-sky-500/20 via-brand-500/10 to-transparent border-sky-200/60 dark:border-sky-900/40",
            badge: "Pipelines"
        },
        ai: {
            title: "AI Runtime & Provider Policy",
            subtitle: "Provider configuration, risk thresholds, usage analytics, and audit visibility for all AI-assisted workflows.",
            accentClass: "from-brand-500/20 via-emerald-500/10 to-transparent border-brand-200/60 dark:border-brand-900/40",
            badge: "AI Ops"
        },
        integrations: {
            title: "External Integrations",
            subtitle: "Configure connected systems and credentials for notifications and Portainer-backed orchestration context.",
            accentClass: "from-cyan-500/20 via-blue-500/10 to-transparent border-cyan-200/60 dark:border-cyan-900/40",
            badge: "Connectors"
        },
        system: {
            title: "System Runtime Controls",
            subtitle: "Operational settings for metrics, ClamAV signatures, instance identity, and validation defaults.",
            accentClass: "from-amber-500/20 via-orange-500/10 to-transparent border-amber-200/60 dark:border-amber-900/40",
            badge: "Runtime"
        },
        backups: {
            title: "Backup & Snapshot Controls",
            subtitle: "Configure compose snapshot storage used before compose-managed upgrades and redeploy operations.",
            accentClass: "from-emerald-500/20 via-teal-500/10 to-transparent border-emerald-200/60 dark:border-emerald-900/40",
            badge: "Backups"
        },
        appearance: {
            title: "Interface Presentation",
            subtitle: "Adjust motion and visual interpretation defaults for day-to-day operation across all HarborWatch views.",
            accentClass: "from-fuchsia-500/20 via-indigo-500/10 to-transparent border-fuchsia-200/60 dark:border-fuchsia-900/40",
            badge: "UX"
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

    const retentionPresetToDays: Record<RetentionWindowPreset, number> = {
        "7d": 7,
        "30d": 30,
        "90d": 90,
        "365d": 365
    };

    function detectRetentionWindowPreset(): RetentionWindowPreset {
        const current = Number(settings.dataRetentionDays || 30);
        for (const [preset, days] of Object.entries(retentionPresetToDays)) {
            if (days === current) return preset as RetentionWindowPreset;
        }
        return "30d";
    }

    function applyRetentionWindowPreset(preset: RetentionWindowPreset) {
        retentionWindowPreset = preset;
        const days = retentionPresetToDays[preset];
        settings.dataRetentionDays = days;
        settings.retentionLogsDays = days;
        settings.retentionMetricsDays = days;
        settings.retentionScanResultsDays = days;
        settings.retentionScanJobsDays = days;
        settings.retentionUpdateRunsDays = days;
        settings.retentionComposeAuditDays = days;
        settings.retentionAIUsageDays = days;
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

    function cloneSettings(input: Settings): Settings {
        return JSON.parse(JSON.stringify({ ...defaultSettings, ...input }));
    }

    function sanitizedSettingsForSave(input: Settings): Settings {
        const next = cloneSettings(input);
        next.aiBlockRiskThreshold = Math.max(0, Math.min(100, Number(next.aiBlockRiskThreshold || 80)));
        next.autoUpgradeMaxConcurrency = Math.max(1, Math.min(20, Number(next.autoUpgradeMaxConcurrency || 1)));
        next.autoUpgradeMinRetryMinutes = Math.max(1, Math.min(1440, Number(next.autoUpgradeMinRetryMinutes || 60)));
        next.clamavSnapshotMaxBytes = Math.max(1, Number(next.clamavSnapshotMaxBytes || 2147483648));
        next.retentionLogsDays = Math.max(1, Math.min(3650, Number(next.retentionLogsDays || 30)));
        next.retentionMetricsDays = Math.max(1, Math.min(3650, Number(next.retentionMetricsDays || 14)));
        next.retentionScanResultsDays = Math.max(1, Math.min(3650, Number(next.retentionScanResultsDays || 30)));
        next.retentionScanJobsDays = Math.max(1, Math.min(3650, Number(next.retentionScanJobsDays || 30)));
        next.retentionUpdateRunsDays = Math.max(1, Math.min(3650, Number(next.retentionUpdateRunsDays || 90)));
        next.retentionComposeAuditDays = Math.max(1, Math.min(3650, Number(next.retentionComposeAuditDays || 90)));
        next.retentionAIUsageDays = Math.max(1, Math.min(3650, Number(next.retentionAIUsageDays || 180)));
        next.environmentOverrides = next.environmentOverrides || {};
        return next;
    }

    function settingsSignature(input: Settings): string {
        return JSON.stringify(sanitizedSettingsForSave(input));
    }

    const settingsDirty = $derived.by(() => !loading && settingsSavedSignature !== "" && settingsSignature(settings) !== settingsSavedSignature);

    type SaveIndicatorState = "saving" | "error" | "pending" | "saved";

    function indicatorToneClass(state: SaveIndicatorState): string {
        switch (state) {
            case "saving":
                return "border-brand-200 bg-brand-50 text-brand-700 dark:border-brand-900/40 dark:bg-brand-900/20 dark:text-brand-300";
            case "error":
                return "border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900/40 dark:bg-rose-900/20 dark:text-rose-300";
            case "pending":
                return "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-300";
            default:
                return "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-900/20 dark:text-emerald-300";
        }
    }

    function indicatorSymbol(state: SaveIndicatorState): string {
        if (state === "saving") return "…";
        if (state === "error") return "x";
        if (state === "pending") return "!";
        return "✓";
    }

    function settingsIndicatorState(): SaveIndicatorState {
        if (saving) return "saving";
        if (settingsSaveErrorSignature && settingsSaveErrorSignature === settingsSignature(settings)) return "error";
        if (settingsDirty) return "pending";
        return "saved";
    }

    function schedulesForDomain(domain: AutomationDomain): Schedule[] {
        let tasks = automationConfig[domain].tasks;
        if (domain === "maintenance") {
            tasks = tasks.filter((id) => id !== "metrics_prune" && id !== "diag_log_prune" && id !== "history_retention_prune");
        }
        return tasks
            .map((id) => scheduleById(id))
            .filter((v): v is Schedule => !!v);
    }

    function domainEnabled(domain: AutomationDomain): boolean {
        if (domain === "remediation") return settings.unhealthyAutoRemediationEnabled;
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

        if (id === "history_retention_prune") {
            // Also toggle internal sub-tasks
            await Promise.all([
                setScheduleEnabled("metrics_prune", enabled),
                setScheduleEnabled("diag_log_prune", enabled)
            ]).catch(() => {});
        }
    }

    async function setDomainEnabled(domain: AutomationDomain, enabled: boolean) {
        if (domainToggleBusy) return;
        domainToggleBusy = domain;

        if (domain === "remediation") {
            settings.unhealthyAutoRemediationEnabled = enabled;
            toasts.info(`${automationConfig[domain].title} ${enabled ? "enabled" : "disabled"} locally. Click Save Settings to persist.`);
            domainToggleBusy = "";
            return;
        }

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
        const enabled = domainEnabled(domain);
        const scoped = schedulesForDomain(domain);
        const total = Math.max(scoped.length, 1);
        const activeCount = scoped.filter((s) => s.enabled).length;

        return automationConfig[domain].flow.map((label, idx, arr) => {
            if (!enabled) return { label, state: "idle" as const };
            
            // For domains with tasks (upgrades, maintenance, security)
            if (scoped.length > 0) {
                if (activeCount >= total) return { label, state: "active" as const };
                if (idx === 0 || idx === arr.length - 1) return { label, state: "active" as const };
                return { label, state: "warning" as const };
            }

            // For taskless domains (remediation)
            return { label, state: "active" as const };
        });
    }

    function normalizeCronSpecClient(spec: string): string {
        const fields = String(spec || "").trim().split(/\s+/).filter(Boolean);
        if (fields.length === 5) return `0 ${fields.join(" ")}`;
        if (fields.length === 6) return fields.join(" ");
        return String(spec || "").trim();
    }

    function isUpdateCheckIntervalTask(id: string): boolean {
        return id === "container_update_check";
    }

    function updateCheckIntervalToCronSpec(interval: UpdateCheckIntervalOption): string {
        switch (interval) {
            case "10m": return "0 */10 * * * *";
            case "30m": return "0 */30 * * * *";
            case "60m": return "0 0 */1 * * *";
            case "5h": return "0 0 */5 * * *";
            case "12h": return "0 0 */12 * * *";
            case "24h": return "0 0 0 * * *";
            default: return "0 0 0 * * *";
        }
    }

    function updateCheckIntervalFromCronSpec(spec: string): UpdateCheckIntervalOption | null {
        const normalized = normalizeCronSpecClient(spec);
        switch (normalized) {
            case "0 */10 * * * *":
                return "10m";
            case "0 */30 * * * *":
                return "30m";
            case "0 0 */1 * * *":
            case "0 0 * * * *":
                return "60m";
            case "0 0 */5 * * *":
                return "5h";
            case "0 0 */12 * * *":
                return "12h";
            case "0 0 0 * * *":
                return "24h";
            default:
                return null;
        }
    }

    function updateCheckIntervalLabel(interval: UpdateCheckIntervalOption): string {
        return updateCheckIntervalOptions.find((opt) => opt.value === interval)?.label || "Every 24 hours";
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
        const intervalNext: Record<string, UpdateCheckIntervalOption> = {};
        for (const schedule of schedules) {
            next[schedule.id] = parseScheduleDraft(schedule.cronSpec);
            if (isUpdateCheckIntervalTask(schedule.id)) {
                intervalNext[schedule.id] = updateCheckIntervalFromCronSpec(schedule.cronSpec) || "24h";
            }
        }
        scheduleDrafts = next;
        updateCheckIntervalDrafts = intervalNext;
    }

    function draftForTask(task: Schedule): ScheduleDraft {
        return scheduleDrafts[task.id] ?? parseScheduleDraft(task.cronSpec);
    }

    function updateCheckIntervalDraftForTask(task: Schedule): UpdateCheckIntervalOption {
        return updateCheckIntervalDrafts[task.id] ?? updateCheckIntervalFromCronSpec(task.cronSpec) ?? "24h";
    }

    function patchUpdateCheckIntervalDraft(id: string, interval: UpdateCheckIntervalOption) {
        updateCheckIntervalDrafts = { ...updateCheckIntervalDrafts, [id]: interval };
    }

    async function handleUpdateCheckIntervalChange(task: Schedule, interval: UpdateCheckIntervalOption) {
        patchUpdateCheckIntervalDraft(task.id, interval);
        const nextCron = updateCheckIntervalToCronSpec(interval);
        if (nextCron === normalizeCronSpecClient(task.cronSpec)) return;
        await saveTaskSchedule(task.id, interval);
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
        if (isUpdateCheckIntervalTask(task.id)) {
            const interval = updateCheckIntervalDraftForTask(task);
            return updateCheckIntervalToCronSpec(interval) !== normalizeCronSpecClient(task.cronSpec);
        }
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
        settingsSavedSignature = settingsSignature(settings);
        settingsSaveErrorSignature = "";
        retentionWindowPreset = detectRetentionWindowPreset();
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

    async function bulkSetAutoApply() {
        if (!discoveredContainers.length) return;
        if (!confirm(`Are you sure you want to set ALL (${discoveredContainers.length}) containers to "Automatic"? This will enable automated upgrades for every container not explicitly ignored.`)) return;

        bulkUpdating = true;
        let success = 0;
        let fail = 0;

        try {
            // We do this in parallel but with a small limit if there are many, 
            // though for most home labs Discovery list is small enough for Promise.all
            const results = await Promise.allSettled(discoveredContainers.map(async (c) => {
                const res = await fetch(`/api/docker/${encodeURIComponent(c.id)}/rules`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        containerId: c.id,
                        updatePolicy: "auto",
                        inheritAutomation: true,
                        upgradesAutomation: true,
                        maintenanceAutomation: true,
                        securityAutomation: true
                    })
                });
                if (!res.ok) throw new Error("fail");
                return true;
            }));

            results.forEach(r => {
                if (r.status === "fulfilled") success++;
                else fail++;
            });

            if (fail > 0) {
                toasts.warning(`Bulk update partial: ${success} set to Automatic, ${fail} failed.`);
            } else {
                toasts.success(`Successfully set all ${success} containers to Automatic.`);
            }

            await loadContainersForExclusions();

            const updateCheckEnabled = scheduleById("container_update_check")?.enabled ?? false;
            const updateApplyEnabled = scheduleById("container_update_apply")?.enabled ?? false;
            if (!updateCheckEnabled || !updateApplyEnabled) {
                toasts.info("Container policies were updated. Enable the Update Check and Update Apply tasks to run upgrades automatically.");
            }
        } catch (e) {
            toasts.error("Bulk update failed to complete.");
        } finally {
            bulkUpdating = false;
        }
    }

    async function saveSettings() {
        saving = true;
        settingsSaveErrorSignature = "";
        try {
            const payload = sanitizedSettingsForSave(settings);
            settings = payload;
            retentionWindowPreset = detectRetentionWindowPreset();
            const res = await fetch("/api/settings", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(payload)
            });
            if (!res.ok) throw new Error(`settings save failed (${res.status})`);
            toasts.success("Settings saved.");
            await loadSettings();
            await loadAIUsage();
        } catch (e) {
            settingsSaveErrorSignature = settingsSignature(settings);
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

    async function saveTaskSchedule(id: string, updateCheckIntervalOverride?: UpdateCheckIntervalOption) {
        const isUpdateCheckTask = isUpdateCheckIntervalTask(id);
        const draft = scheduleDrafts[id];
        if (!isUpdateCheckTask && !draft) return;
        const cronSpec = isUpdateCheckTask
            ? updateCheckIntervalToCronSpec(updateCheckIntervalOverride ?? updateCheckIntervalDrafts[id] ?? "24h")
            : draftToCronSpec(draft as ScheduleDraft);
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
            if (isUpdateCheckTask) {
                updateCheckIntervalDrafts = {
                    ...updateCheckIntervalDrafts,
                    [id]: updateCheckIntervalFromCronSpec(cronSpec) || "24h"
                };
            }
            
            if (id === "history_retention_prune") {
                // Sync internal sub-tasks to the same schedule
                const updateTask = (subId: string) => fetch("/api/scheduler/update", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ id: subId, cronSpec })
                }).then(async (res) => {
                    if (res.ok) {
                        schedules = schedules.map((s) => (s.id === subId ? { ...s, cronSpec } : s));
                        scheduleDrafts = { ...scheduleDrafts, [subId]: parseScheduleDraft(cronSpec) };
                    }
                });
                await Promise.all([updateTask("metrics_prune"), updateTask("diag_log_prune")]).catch(() => {});
            }

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

    function taskCronLabel(task: Schedule): string {
        if (isUpdateCheckIntervalTask(task.id)) {
            const interval = updateCheckIntervalFromCronSpec(task.cronSpec);
            if (interval) return updateCheckIntervalLabel(interval);
        }
        return cronLabel(task.cronSpec);
    }

    function taskLabel(id: string): string {
        switch (id) {
            case "container_update_check": return "Container Update Check";
            case "container_update_apply": return "Container Auto-Apply";
            case "compose_snapshot_on_change": return "Compose Snapshot Sweep (On Change)";
            case "docker_system_prune": return "Docker System Prune";
            case "history_retention_prune": return "Historical Data Retention";
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
            case "compose_snapshot_on_change":
                return "Scans local Compose projects and writes a snapshot only when Compose files or project .env changed since the last snapshot.";
            case "docker_system_prune":
                return "Reclaims disk by removing eligible Docker artifacts based on prune policy.";
            case "metrics_prune":
                return "Trims old metrics to keep database growth predictable.";
            case "diag_log_prune":
                return "Deletes aged diagnostics logs after retention limits are reached.";
            case "history_retention_prune":
                return "Unified task that prunes logs, metrics, scan history, update runs, and AI records using the configured retention window.";
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

    async function clearHistory() {
        if (!confirm("Are you absolutely sure? This will permanently delete all historical metrics, scan results, update logs, and AI usage events. This cannot be undone.")) {
            return;
        }
        
        try {
            const res = await fetch("/api/system/clear-history", { method: "POST" });
            if (!res.ok) {
                const body = await res.json().catch(() => ({}));
                throw new Error(body?.message || `Clear failed (${res.status})`);
            }
            toasts.success("All historical data has been cleared.");
            await loadAll();
        } catch (e) {
            toasts.error(e instanceof Error ? e.message : "Failed to clear history");
        }
    }

    onMount(() => {
        loadAll();
    });
</script>

<div class="w-full space-y-6 settings-page">
    <div class="settings-topbar flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
            <h2 class="text-3xl font-black text-slate-900 dark:text-white tracking-tight">Settings</h2>
            <p class="text-sm text-slate-500 mt-1">Global configuration and automation policy control plane.</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
            <span
                class="inline-flex items-center justify-center p-2 rounded-xl border {indicatorToneClass(settingsIndicatorState())}"
                title={settingsIndicatorState() === "saving" ? "Saving..." : settingsIndicatorState() === "error" ? "Save Error" : settingsIndicatorState() === "pending" ? "Unsaved Changes" : "All Changes Saved"}
            >
                <span class="inline-flex items-center justify-center w-4 h-4 rounded-full border border-current/20 bg-white/60 dark:bg-slate-900/30 text-[10px] font-black">
                    {indicatorSymbol(settingsIndicatorState())}
                </span>
            </span>
            <button
                onclick={saveSettings}
                disabled={saving || loading || !settingsDirty}
                class="px-6 py-3 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white rounded-2xl font-black uppercase tracking-widest text-[10px] shadow-lg shadow-brand-500/20"
            >
                {saving ? "Saving..." : settingsDirty ? "Save Settings" : "Saved"}
            </button>
        </div>
    </div>

    <div class="settings-tab-strip flex flex-wrap gap-2 bg-slate-100/95 dark:bg-slate-900/80 p-1.5 rounded-2xl border border-slate-200 dark:border-slate-800 w-fit backdrop-blur">
        {#each [
            { id: "automations", label: "Automations" },
            { id: "ai", label: "AI", status: configStore.initialized && !configStore.aiActive ? "Inactive" : "" },
            { id: "integrations", label: "Integrations", status: configStore.initialized && !configStore.portainerActive && settings.portainerEnabled ? "Portainer Error" : "" },
            { id: "system", label: "System" },
            { id: "backups", label: "Backups" },
            { id: "appearance", label: "Appearance" }
        ] as tab}
            <button
                onclick={() => activeTab = tab.id}
                class="px-5 py-2.5 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all flex items-center gap-2 {activeTab === tab.id ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
            >
                <span class="inline-flex items-center justify-center w-4 h-4 rounded-md bg-white/60 dark:bg-slate-800/70">
                    {#if tab.id === "automations"}
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6h10M4 6h2m4 12h10M4 18h2m10-6h4M4 12h8m-2-8v4m0 8v4m4-10v4" /></svg>
                    {:else if tab.id === "ai"}
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2m-9 4h12M7 21h10a2 2 0 002-2V9H5v10a2 2 0 002 2z" /></svg>
                    {:else if tab.id === "integrations"}
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h8m-4-4v8M7 5h10a2 2 0 012 2v10a2 2 0 01-2 2H7a2 2 0 01-2-2V7a2 2 0 012-2z" /></svg>
                    {:else if tab.id === "system"}
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 3a2.25 2.25 0 00-2.122 1.5l-.223.665a2.25 2.25 0 01-1.423 1.423l-.665.223a2.25 2.25 0 000 4.278l.665.223a2.25 2.25 0 011.423 1.423l.223.665a2.25 2.25 0 004.278 0l.223-.665a2.25 2.25 0 011.423-1.423l.665-.223a2.25 2.25 0 000-4.278l-.665-.223a2.25 2.25 0 01-1.423-1.423l-.223-.665A2.25 2.25 0 009.75 3z" /></svg>
                    {:else if tab.id === "backups"}
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7h16v10a2 2 0 01-2 2H6a2 2 0 01-2-2V7zm0 0l2-3h12l2 3M12 11v6m0 0l-3-3m3 3l3-3" /></svg>
                    {:else}
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7h18M6 3h12l1 4H5l1-4zm-1 4h14v14H5V7z" /></svg>
                    {/if}
                </span>
                {tab.label}
                {#if tab.status}
                    <span class="px-1.5 py-0.5 rounded-md bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-400 text-[8px] font-black">{tab.status}</span>
                {/if}
            </button>
        {/each}
    </div>

    <div class="settings-shell w-full bg-white dark:bg-slate-800 rounded-3xl border border-slate-200 dark:border-slate-700 shadow-sm min-h-[620px] overflow-hidden">
        {#if loading}
            <div class="p-10 text-sm text-slate-500">Loading settings...</div>
        {:else if activeTab === "automations"}
            <div class="p-6 md:p-8 space-y-6 settings-pane">
                <div class="settings-pane-hero rounded-2xl border bg-gradient-to-r {settingsTabChrome.automations.accentClass} p-5">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                        <div class="max-w-3xl">
                            <div class="flex items-center gap-2 mb-2">
                                <span class="px-2 py-0.5 rounded-full bg-white/80 dark:bg-slate-900/60 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 border border-white/70 dark:border-slate-700">{settingsTabChrome.automations.badge}</span>
                                <span class="text-[10px] font-black uppercase tracking-widest text-brand-700 dark:text-brand-300">{automationConfig[activeAutomationTab].title}</span>
                            </div>
                            <h3 class="text-lg font-black tracking-tight text-slate-900 dark:text-white">{settingsTabChrome.automations.title}</h3>
                            <p class="text-[12px] text-slate-600 dark:text-slate-300 mt-1">{settingsTabChrome.automations.subtitle}</p>
                        </div>
                        <div class="rounded-xl border border-white/70 dark:border-slate-700 bg-white/80 dark:bg-slate-900/50 px-3 py-2 text-right min-w-[180px]">
                            <p class="text-[9px] font-black uppercase tracking-widest text-slate-500">Active Domain</p>
                            <p class="text-xs font-bold text-slate-800 dark:text-slate-100 mt-1">{automationConfig[activeAutomationTab].title}</p>
                            <p class="text-[10px] text-slate-500 mt-1">{activeAutomationTab === "general" ? "Global automation defaults" : (domainEnabled(activeAutomationTab) ? "Domain enabled" : "Domain disabled")}</p>
                        </div>
                    </div>
                </div>

                <div class="settings-subtab-strip flex flex-wrap gap-2 bg-slate-100/90 dark:bg-slate-900/70 p-1.5 rounded-2xl border border-slate-200 dark:border-slate-700 w-fit backdrop-blur">
                    {#each [
                        { id: "general", label: "General" },
                        { id: "upgrades", label: "Upgrades" },
                        { id: "maintenance", label: "Maintenance" },
                        { id: "security", label: "Security" },
                        { id: "remediation", label: "Remediation" }
                    ] as tab}
                        <button
                            onclick={() => activeAutomationTab = tab.id as AutomationDomain}
                            class="px-4 py-2 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all {activeAutomationTab === tab.id ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
                        >
                            {tab.label}
                        </button>
                    {/each}
                </div>

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 flex flex-wrap items-center justify-between gap-3 shadow-sm">
                    <div>
                        {#if activeAutomationTab !== "general"}
                            <p class="text-xs font-black uppercase tracking-wider text-slate-500">{automationConfig[activeAutomationTab].title}</p>
                            <p class="text-[11px] text-slate-500 mt-1">
                                Domain status: <span class="font-bold">{domainEnabled(activeAutomationTab) ? "Enabled" : "Disabled"}</span>
                            </p>
                        {:else}
                            <p class="text-xs font-black uppercase tracking-wider text-slate-500">Global Configuration</p>
                            <p class="text-[11px] text-slate-500 mt-1">System-wide automation parameters</p>
                        {/if}
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

                <div class="grid grid-cols-1 {activeAutomationTab === 'general' ? '' : 'xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]'} gap-6">
                    {#if activeAutomationTab !== "general"}
                        <div class="space-y-4">
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
                            
                        </div>
                    {/if}

                    <div class="space-y-4">
                        {#if activeAutomationTab !== "general"}
                            <div class="px-2 py-1">
                                <p class="text-xs font-black uppercase tracking-wider text-brand-500 dark:text-brand-400">
                                    Automation Task Controls
                                </p>
                                <p class="text-[11px] text-slate-500 mt-1">
                                    Use toggles to enable schedules, set cadence/time, and run on-demand checks for validation.
                                </p>
                            </div>
                        {/if}

                        {#if activeAutomationTab === "general"}
                            <div class="flex flex-col gap-4">
                                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4">
                                    <div class="mb-4">
                                        <p class="text-sm font-black text-slate-800 dark:text-slate-100">Global Task Concurrency</p>
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
                            </div>

                            <div class="space-y-4">
                                <div class="rounded-2xl border-2 border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4">
                                    <div class="mb-4">
                                        <div class="flex flex-wrap items-center gap-2">
                                            <span class="px-2 py-0.5 rounded-full text-[8px] font-black uppercase tracking-widest bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300">Safety Policy</span>
                                            <p class="text-sm font-black text-slate-800 dark:text-slate-100">Automation Safety Exclusions</p>
                                        </div>
                                        <p class="text-[11px] text-slate-500 mt-2">Ignored containers are excluded from all container-scoped automations. HarborWatch is always protected and cannot be removed.</p>
                                    </div>
                                    <div class="grid grid-cols-1 gap-4">
                                    <div class="space-y-2">
                                        <p class="text-[10px] font-black uppercase tracking-wider text-slate-400">Ignored Containers</p>
                                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/30 max-h-[260px] overflow-y-auto custom-scrollbar">
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
                                </div>
                            </div>
                        </div>
                    {:else}
                            <div class="space-y-4">
                                {#if activeAutomationTab === "upgrades"}
                                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                                        <div>
                                            <div class="flex items-center gap-2">
                                                <span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300 border border-sky-200/70 dark:border-sky-900/40">
                                                    <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
                                                </span>
                                                <p class="text-sm font-black text-slate-800 dark:text-slate-100">Upgrade Runtime Controls</p>
                                            </div>
                                            <p class="text-[11px] text-slate-500 mt-1">Tune how aggressively auto-apply runs and how long failed containers wait before retry.</p>
                                        </div>
                                        <div class="grid grid-cols-1 gap-4">
                                            <div class="space-y-4">
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
                                    </div>

                                {/if}

                                {#if activeAutomationTab === "security"}
                                    {@const trivyTask = scheduleById("security_sweep_trivy")}
                                    {@const trivyDraft = trivyTask ? draftForTask(trivyTask) : null}
                                    {@const malwareTask = scheduleById("malware_sweep_clamav")}
                                    {@const malwareDraft = malwareTask ? draftForTask(malwareTask) : null}
                                    {@const clamSigTask = scheduleById("clamav_signature_update")}
                                    {@const clamSigDraft = clamSigTask ? draftForTask(clamSigTask) : null}
                                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-3">
                                        <div class="flex items-center gap-2">
                                            <span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300 border border-orange-200/70 dark:border-orange-900/40">
                                                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L6 20.75 2.25 17M6 20.75V3m8.25 4h7.5m-7.5 5h5.25m-5.25 5h3" /></svg>
                                            </span>
                                            <div>
                                                <p class="text-sm font-black text-slate-800 dark:text-slate-100">Trivy</p>
                                                <p class="text-[11px] text-slate-500 mt-1">Trivy sweep targeting and runtime preferences for vulnerability automation.</p>
                                            </div>
                                        </div>
                                        <label for="trivy-sweep-mode" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Scan Target Selection</label>
                                        <select
                                            id="trivy-sweep-mode"
                                            bind:value={settings.trivySweepMode}
                                            disabled={isLocked("trivySweepMode")}
                                            class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                        >
                                            <option value="running-only">Running containers only (default)</option>
                                            <option value="all-images">All local images</option>
                                        </select>
                                        <p class="text-[11px] text-slate-500">`running-only` avoids queue inflation by scanning only images currently in use.</p>
                                        {#if trivyTask && trivyDraft}
                                            <div class="mt-2 rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3 space-y-3">
                                                <div class="flex flex-wrap items-center justify-between gap-3">
                                                    <div>
                                                        <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Scheduler Task</p>
                                                        <p class="text-[11px] text-slate-600 dark:text-slate-300 mt-1">{taskCronLabel(trivyTask)} | Last run: {formatTime(trivyTask.lastRun)}</p>
                                                    </div>
                                                    <div class="flex items-center gap-2">
                                                        <button
                                                            onclick={() => runTask(trivyTask.id)}
                                                            class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800"
                                                        >Run Now</button>
                                                        <button
                                                            onclick={() => toggleTask(trivyTask.id, trivyTask.enabled)}
                                                            class="w-10 h-5 rounded-full relative transition-colors {trivyTask.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                                                            aria-label="Toggle Trivy task"
                                                        >
                                                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {trivyTask.enabled ? 'right-1' : 'left-1'}"></div>
                                                        </button>
                                                    </div>
                                                </div>
                                                <div class="grid grid-cols-1 md:grid-cols-[1fr_1fr_auto] gap-3 items-end">
                                                    <div class="space-y-1">
                                                        <label for="cadence-security-sweep-trivy" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Cadence</label>
                                                        <select
                                                            id="cadence-security-sweep-trivy"
                                                            value={trivyDraft.cadence}
                                                            onchange={(e) => patchScheduleDraft(trivyTask.id, { cadence: (e.currentTarget as HTMLSelectElement).value as ScheduleCadence })}
                                                            class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                                        >
                                                            <option value="daily">Daily</option>
                                                            <option value="weekly">Weekly</option>
                                                            <option value="monthly">Monthly</option>
                                                        </select>
                                                    </div>
                                                    <div class="space-y-1">
                                                        <label for="time-security-sweep-trivy" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Run Time</label>
                                                        <input
                                                            id="time-security-sweep-trivy"
                                                            type="time"
                                                            value={trivyDraft.time}
                                                            onchange={(e) => patchScheduleDraft(trivyTask.id, { time: (e.currentTarget as HTMLInputElement).value || "00:00" })}
                                                            class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                                        />
                                                    </div>
                                                    <button
                                                        onclick={() => saveTaskSchedule(trivyTask.id)}
                                                        disabled={!scheduleDirty(trivyTask) || savingScheduleId === trivyTask.id}
                                                        class="px-3 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest"
                                                    >
                                                        {savingScheduleId === trivyTask.id ? "Saving..." : "Save Schedule"}
                                                    </button>
                                                </div>
                                                {#if trivyDraft.cadence === "weekly"}
                                                    <div class="space-y-1">
                                                        <p class="text-[10px] font-black uppercase tracking-wider text-slate-400">Run On Days</p>
                                                        <div class="flex flex-wrap gap-2">
                                                            {#each weekdayOptions as day}
                                                                <button
                                                                    onclick={() => toggleWeeklyDay(trivyTask.id, day.value)}
                                                                    class="px-2.5 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-wider border transition-colors {trivyDraft.weeklyDays.includes(day.value) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                                                >
                                                                    {day.label}
                                                                </button>
                                                            {/each}
                                                        </div>
                                                    </div>
                                                {:else if trivyDraft.cadence === "monthly"}
                                                    <div class="space-y-1">
                                                        <p class="text-[10px] font-black uppercase tracking-wider text-slate-400">Run On Dates</p>
                                                        <div class="flex flex-wrap gap-1.5">
                                                            {#each monthDayOptions as day}
                                                                <button
                                                                    onclick={() => toggleMonthDay(trivyTask.id, day)}
                                                                    class="min-w-8 px-2 py-1 rounded-lg text-[10px] font-black border transition-colors {trivyDraft.monthDays.includes(day) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                                                >
                                                                    {day}
                                                                </button>
                                                            {/each}
                                                        </div>
                                                    </div>
                                                {/if}
                                            </div>
                                        {/if}
                                    </div>

                                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                                        <div class="flex items-center gap-2">
                                            <span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300 border border-emerald-200/70 dark:border-emerald-900/40">
                                                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                                            </span>
                                            <div>
                                                <p class="text-sm font-black text-slate-800 dark:text-slate-100">ClamAV</p>
                                                <p class="text-[11px] text-slate-500 mt-1">Malware sweep exclusions, signature freshness, and snapshot caps for security automation.</p>
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


                                        <div class="flex flex-wrap items-center gap-2">
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
                                                {clamavUpdating ? "Updating..." : "Update Signatures"}
                                            </button>
                                        </div>

                                        <div class="space-y-2">
                                            <label for="clamav-snapshot-max-bytes-security" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Container Snapshot Max Bytes</label>
                                            <input
                                                id="clamav-snapshot-max-bytes-security"
                                                type="number"
                                                min="1"
                                                bind:value={settings.clamavSnapshotMaxBytes}
                                                disabled={isLocked("clamavSnapshotMaxBytes")}
                                                class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                            />
                                            <p class="text-[11px] text-slate-500">Current cap: <span class="font-bold">{formatBytesCompact(settings.clamavSnapshotMaxBytes || 0)}</span>.</p>
                                        </div>

                                        <div class="grid grid-cols-1 gap-4">
                                            {#if malwareTask && malwareDraft}
                                                <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3 space-y-3">
                                                    <div class="flex flex-wrap items-center justify-between gap-2">
                                                        <div>
                                                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">ClamAV Malware Sweep</p>
                                                            <p class="text-[11px] text-slate-600 dark:text-slate-300 mt-1">{taskCronLabel(malwareTask)} | Last run: {formatTime(malwareTask.lastRun)}</p>
                                                        </div>
                                                        <div class="flex items-center gap-2">
                                                            <button onclick={() => runTask(malwareTask.id)} class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800">Run Now</button>
                                                            <button
                                                                onclick={() => toggleTask(malwareTask.id, malwareTask.enabled)}
                                                                class="w-10 h-5 rounded-full relative transition-colors {malwareTask.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                                                                aria-label="Toggle ClamAV malware sweep task"
                                                            >
                                                                <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {malwareTask.enabled ? 'right-1' : 'left-1'}"></div>
                                                            </button>
                                                        </div>
                                                    </div>
                                                    <div class="grid grid-cols-1 md:grid-cols-[1fr_1fr_auto] gap-3 items-end">
                                                        <div class="space-y-1">
                                                            <label for="cadence-malware-sweep-clamav" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Cadence</label>
                                                            <select
                                                                id="cadence-malware-sweep-clamav"
                                                                value={malwareDraft.cadence}
                                                                onchange={(e) => patchScheduleDraft(malwareTask.id, { cadence: (e.currentTarget as HTMLSelectElement).value as ScheduleCadence })}
                                                                class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                                            >
                                                                <option value="daily">Daily</option>
                                                                <option value="weekly">Weekly</option>
                                                                <option value="monthly">Monthly</option>
                                                            </select>
                                                        </div>
                                                        <div class="space-y-1">
                                                            <label for="time-malware-sweep-clamav" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Run Time</label>
                                                            <input
                                                                id="time-malware-sweep-clamav"
                                                                type="time"
                                                                value={malwareDraft.time}
                                                                onchange={(e) => patchScheduleDraft(malwareTask.id, { time: (e.currentTarget as HTMLInputElement).value || "00:00" })}
                                                                class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                                            />
                                                        </div>
                                                        <button
                                                            onclick={() => saveTaskSchedule(malwareTask.id)}
                                                            disabled={!scheduleDirty(malwareTask) || savingScheduleId === malwareTask.id}
                                                            class="px-3 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest"
                                                        >
                                                            {savingScheduleId === malwareTask.id ? "Saving..." : "Save Schedule"}
                                                        </button>
                                                    </div>
                                                    {#if malwareDraft.cadence === "weekly"}
                                                        <div class="flex flex-wrap gap-2">
                                                            {#each weekdayOptions as day}
                                                                <button
                                                                    onclick={() => toggleWeeklyDay(malwareTask.id, day.value)}
                                                                    class="px-2.5 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-wider border transition-colors {malwareDraft.weeklyDays.includes(day.value) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                                                >{day.label}</button>
                                                            {/each}
                                                        </div>
                                                    {:else if malwareDraft.cadence === "monthly"}
                                                        <div class="flex flex-wrap gap-1.5">
                                                            {#each monthDayOptions as day}
                                                                <button
                                                                    onclick={() => toggleMonthDay(malwareTask.id, day)}
                                                                    class="min-w-8 px-2 py-1 rounded-lg text-[10px] font-black border transition-colors {malwareDraft.monthDays.includes(day) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                                                >{day}</button>
                                                            {/each}
                                                        </div>
                                                    {/if}
                                                </div>
                                            {/if}

                                            {#if clamSigTask && clamSigDraft}
                                                <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3 space-y-3">
                                                    <div class="flex flex-wrap items-center justify-between gap-2">
                                                        <div>
                                                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">ClamAV Signature Update</p>
                                                            <p class="text-[11px] text-slate-600 dark:text-slate-300 mt-1">{taskCronLabel(clamSigTask)} | Last run: {formatTime(clamSigTask.lastRun)}</p>
                                                        </div>
                                                        <div class="flex items-center gap-2">
                                                            <button onclick={() => runTask(clamSigTask.id)} class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800">Run Now</button>
                                                            <button
                                                                onclick={() => toggleTask(clamSigTask.id, clamSigTask.enabled)}
                                                                class="w-10 h-5 rounded-full relative transition-colors {clamSigTask.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                                                                aria-label="Toggle ClamAV signature update task"
                                                            >
                                                                <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {clamSigTask.enabled ? 'right-1' : 'left-1'}"></div>
                                                            </button>
                                                        </div>
                                                    </div>
                                                    <div class="grid grid-cols-1 md:grid-cols-[1fr_1fr_auto] gap-3 items-end">
                                                        <div class="space-y-1">
                                                            <label for="cadence-clamav-signature-update" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Cadence</label>
                                                            <select
                                                                id="cadence-clamav-signature-update"
                                                                value={clamSigDraft.cadence}
                                                                onchange={(e) => patchScheduleDraft(clamSigTask.id, { cadence: (e.currentTarget as HTMLSelectElement).value as ScheduleCadence })}
                                                                class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                                            >
                                                                <option value="daily">Daily</option>
                                                                <option value="weekly">Weekly</option>
                                                                <option value="monthly">Monthly</option>
                                                            </select>
                                                        </div>
                                                        <div class="space-y-1">
                                                            <label for="time-clamav-signature-update" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Run Time</label>
                                                            <input
                                                                id="time-clamav-signature-update"
                                                                type="time"
                                                                value={clamSigDraft.time}
                                                                onchange={(e) => patchScheduleDraft(clamSigTask.id, { time: (e.currentTarget as HTMLInputElement).value || "00:00" })}
                                                                class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                                            />
                                                        </div>
                                                        <button
                                                            onclick={() => saveTaskSchedule(clamSigTask.id)}
                                                            disabled={!scheduleDirty(clamSigTask) || savingScheduleId === clamSigTask.id}
                                                            class="px-3 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest"
                                                        >
                                                            {savingScheduleId === clamSigTask.id ? "Saving..." : "Save Schedule"}
                                                        </button>
                                                    </div>
                                                    {#if clamSigDraft.cadence === "weekly"}
                                                        <div class="flex flex-wrap gap-2">
                                                            {#each weekdayOptions as day}
                                                                <button
                                                                    onclick={() => toggleWeeklyDay(clamSigTask.id, day.value)}
                                                                    class="px-2.5 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-wider border transition-colors {clamSigDraft.weeklyDays.includes(day.value) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                                                >{day.label}</button>
                                                            {/each}
                                                        </div>
                                                    {:else if clamSigDraft.cadence === "monthly"}
                                                        <div class="flex flex-wrap gap-1.5">
                                                            {#each monthDayOptions as day}
                                                                <button
                                                                    onclick={() => toggleMonthDay(clamSigTask.id, day)}
                                                                    class="min-w-8 px-2 py-1 rounded-lg text-[10px] font-black border transition-colors {clamSigDraft.monthDays.includes(day) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                                                >{day}</button>
                                                            {/each}
                                                        </div>
                                                    {/if}
                                                </div>
                                            {/if}
                                        </div>

                                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3 space-y-3">
                                            <div>
                                                <label for="malware-ignore-mounts-security" class="text-[10px] font-black uppercase tracking-wider text-slate-900 dark:text-white">Ignored Malware Mount Paths</label>
                                                <p class="text-[11px] text-slate-500 mt-0.5">Skipped during scheduled ClamAV sweeps to avoid scanning very large media mounts.</p>
                                            </div>
                                            <textarea
                                                id="malware-ignore-mounts-security"
                                                rows="2"
                                                bind:value={settings.malwareIgnoredMounts}
                                                disabled={isLocked("malwareIgnoredMounts")}
                                                placeholder="/mnt/media, /srv/plex-library"
                                                class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                            ></textarea>
                                        </div>
                                    </div>
                                {/if}

                                {#if activeAutomationTab === "remediation"}
                                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                                        <div>
                                            <div class="flex items-center gap-2">
                                                <span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-300 border border-pink-200/70 dark:border-pink-900/40">
                                                    <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l5 5m0 0l-5 5m5-5H6" /></svg>
                                                </span>
                                                <p class="text-sm font-black text-slate-800 dark:text-slate-100">Unhealthy Auto-Remediation</p>
                                            </div>
                                            <p class="text-[11px] text-slate-500 mt-1">Listen for Docker health events and automatically restart unhealthy containers (opt-in per container).</p>
                                        </div>
                                        <div class="flex items-center justify-between gap-4">
                                            <div class="flex-1">
                                                <p class="text-[11px] text-slate-500 italic">Enable global monitoring of container health status.</p>
                                            </div>
                                            <button
                                                onclick={() => settings.unhealthyAutoRemediationEnabled = !settings.unhealthyAutoRemediationEnabled}
                                                disabled={isLocked("unhealthyAutoRemediationEnabled")}
                                                class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.unhealthyAutoRemediationEnabled ? 'bg-brand-600' : 'bg-slate-300'}"
                                                aria-label="Toggle unhealthy auto-remediation"
                                            >
                                                <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.unhealthyAutoRemediationEnabled ? 'right-1' : 'left-1'}"></div>
                                            </button>
                                        </div>

                                        {#if settings.unhealthyAutoRemediationEnabled}
                                            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-100 dark:border-slate-800">
                                                <div class="space-y-2">
                                                    <label for="remediation-cooldown-default" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Default Restart Cooldown (seconds)</label>
                                                    <input
                                                        id="remediation-cooldown-default"
                                                        type="number"
                                                        min="0"
                                                        bind:value={settings.unhealthyRestartCooldownSecDefault}
                                                        disabled={isLocked("unhealthyRestartCooldownSecDefault")}
                                                        class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                                    />
                                                </div>
                                                <div class="space-y-2">
                                                    <label for="max-restarts-per-window" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Max Restarts (per hour window)</label>
                                                    <input
                                                        id="max-restarts-per-window"
                                                        type="number"
                                                        min="0"
                                                        bind:value={settings.maxRestartsPerWindow}
                                                        disabled={isLocked("maxRestartsPerWindow")}
                                                        class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                                    />
                                                </div>
                                            </div>
                                        {/if}
                                    </div>
                                {/if}

                                {#if activeAutomationTab === "maintenance"}
                                    {@const retentionTask = scheduleById("history_retention_prune")}
                                    {@const retentionDraft = retentionTask ? draftForTask(retentionTask) : null}
                                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                                        <div>
                                            <p class="text-sm font-black text-slate-800 dark:text-slate-100">Historical Data Retention</p>
                                            <p class="text-[11px] text-slate-500 mt-1">Uses a rolling retention window. Rows older than the selected window are pruned during scheduled cleanup.</p>
                                        </div>
                                        <div class="flex flex-wrap items-center justify-between gap-3">
                                            <div>
                                                {#if retentionTask}
                                                    <p class="text-[10px] uppercase tracking-wider text-slate-500 font-bold mt-1">{cronLabel(retentionTask.cronSpec)} | Last run: {formatTime(retentionTask.lastRun)}</p>
                                                {/if}
                                            </div>
                                            {#if retentionTask}
                                                <div class="flex items-center gap-2">
                                                    <button
                                                        onclick={() => runTask(retentionTask.id)}
                                                        class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800"
                                                    >Run Now</button>
                                                    <button
                                                        onclick={() => toggleTask(retentionTask.id, retentionTask.enabled)}
                                                        class="w-10 h-5 rounded-full relative transition-colors {retentionTask.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                                                        aria-label="Toggle task"
                                                    >
                                                        <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {retentionTask.enabled ? 'right-1' : 'left-1'}"></div>
                                                    </button>
                                                </div>
                                            {/if}
                                        </div>

                                        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-100 dark:border-slate-800">
                                            <div class="space-y-2">
                                                <label for="retention-window-preset" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Retention Window</label>
                                                <select
                                                    id="retention-window-preset"
                                                    bind:value={retentionWindowPreset}
                                                    onchange={(e) => applyRetentionWindowPreset((e.currentTarget as HTMLSelectElement).value as RetentionWindowPreset)}
                                                    class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                                >
                                                    <option value="7d">1 week</option>
                                                    <option value="30d">1 month</option>
                                                    <option value="90d">3 months</option>
                                                    <option value="365d">1 year</option>
                                                </select>
                                            </div>

                                            {#if retentionTask && retentionDraft}
                                                <div class="space-y-2">
                                                    <label for="retention-time" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Daily Cleanup Time</label>
                                                    <div class="flex items-center gap-2">
                                                        <input
                                                            id="retention-time"
                                                            type="time"
                                                            value={retentionDraft.time}
                                                            onchange={(e) => patchScheduleDraft(retentionTask.id, { time: (e.currentTarget as HTMLInputElement).value || "00:00" })}
                                                            class="flex-1 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                                        />
                                                        <button
                                                            onclick={() => saveTaskSchedule(retentionTask.id)}
                                                            disabled={!scheduleDirty(retentionTask) || savingScheduleId === retentionTask.id}
                                                            class="px-3 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest whitespace-nowrap"
                                                        >
                                                            {savingScheduleId === retentionTask.id ? "..." : "Save"}
                                                        </button>
                                                    </div>
                                                </div>
                                            {/if}
                                        </div>

                                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3">
                                            <p class="text-[11px] text-slate-600 dark:text-slate-300">
                                                Unified rolling window applied across metrics, logs, scan history, update lifecycle history, compose audit history, and AI usage history:
                                                <span class="font-bold">{retentionPresetToDays[retentionWindowPreset]} days</span>.
                                            </p>
                                        </div>
                                        <p class="text-[11px] text-slate-500">The policy is a rolling window, not a fixed wipe date. Cleanup runs via <span class="font-mono">metrics_prune</span>, <span class="font-mono">diag_log_prune</span>, and <span class="font-mono">history_retention_prune</span>.</p>
                                    </div>
                                {/if}

                                {#if activeAutomationTab !== "remediation" && activeAutomationTab !== "security"}
                                    {#each schedulesForDomain(activeAutomationTab) as task, i (task.id + i)}
                                        {@const draft = draftForTask(task)}
                                        <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-4 bg-white dark:bg-slate-900/30 space-y-4">
                                            <div class="flex flex-wrap items-center justify-between gap-3">
                                                <div>
                                                    <p class="text-sm font-black text-slate-800 dark:text-slate-100">{taskLabel(task.id)}</p>
                                                    <p class="text-[11px] text-slate-500 mt-1">{taskDescription(task.id)}</p>
                                                    <p class="text-[10px] uppercase tracking-wider text-slate-500 font-bold mt-1">{taskCronLabel(task)} | Last run: {formatTime(task.lastRun)}</p>
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

                                            {#if task.id === "container_update_check"}
                                                {@const intervalDraft = updateCheckIntervalDraftForTask(task)}
                                                <div class="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-3 items-end">
                                                    <div class="space-y-1">
                                                        <label for={"update-check-interval-" + task.id} class="text-[10px] font-black uppercase tracking-wider text-slate-400">Check Interval</label>
                                                        <select
                                                            id={"update-check-interval-" + task.id}
                                                            value={intervalDraft}
                                                            onchange={(e) => handleUpdateCheckIntervalChange(task, (e.currentTarget as HTMLSelectElement).value as UpdateCheckIntervalOption)}
                                                            disabled={savingScheduleId === task.id}
                                                            class="w-full bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                                        >
                                                            {#each updateCheckIntervalOptions as option}
                                                                <option value={option.value}>{option.label}</option>
                                                            {/each}
                                                        </select>
                                                        <p class="text-[11px] text-slate-500">
                                                            Controls how often HarborWatch checks registries for newer container images. Changes save automatically.
                                                        </p>
                                                    </div>

                                                    <div class="flex items-center justify-end">
                                                        <span
                                                            class="inline-flex items-center gap-2 px-3 py-2 rounded-xl border text-[10px] font-black uppercase tracking-widest {savingScheduleId === task.id ? 'border-brand-200 bg-brand-50 text-brand-700 dark:border-brand-900/40 dark:bg-brand-900/20 dark:text-brand-300' : scheduleDirty(task) ? 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-300' : 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-900/20 dark:text-emerald-300'}"
                                                            aria-live="polite"
                                                        >
                                                            <span class="w-1.5 h-1.5 rounded-full {savingScheduleId === task.id ? 'bg-brand-500 animate-pulse' : scheduleDirty(task) ? 'bg-amber-500' : 'bg-emerald-500'}"></span>
                                                            {savingScheduleId === task.id ? "Saving..." : scheduleDirty(task) ? "Pending" : "Saved"}
                                                        </span>
                                                    </div>
                                                </div>
                                            {:else}
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
                                            {/if}

                                            {#if task.id !== "container_update_check" && draft.cadence === "weekly"}
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
                                            {:else if task.id !== "container_update_check" && draft.cadence === "monthly"}
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

                                            {#if task.id === "docker_system_prune"}
                                                <div class="pt-3 border-t border-slate-200/60 dark:border-slate-700/60 mt-2">
                                                    <div class="flex items-center justify-between gap-4 rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 px-3 py-2">
                                                        <div class="flex-1">
                                                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-900 dark:text-white">Include Unused Tagged Images</p>
                                                            <p class="text-[11px] text-slate-500 mt-1">When enabled, scheduled cleanup also deletes unused tagged images (for example orphaned images like old Watchtower deployments).</p>
                                                        </div>
                                                        <button
                                                            onclick={() => { settings.dockerPruneIncludeUnusedTaggedImages = !settings.dockerPruneIncludeUnusedTaggedImages; }}
                                                            disabled={isLocked("dockerPruneIncludeUnusedTaggedImages")}
                                                            class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.dockerPruneIncludeUnusedTaggedImages ? 'bg-amber-600' : 'bg-slate-300 dark:bg-slate-700'}"
                                                            aria-label="Toggle scheduled prune of unused tagged images"
                                                        >
                                                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.dockerPruneIncludeUnusedTaggedImages ? 'right-1' : 'left-1'}"></div>
                                                        </button>
                                                    </div>
                                                </div>
                                            {/if}
                                        </div>
                                    {:else}
                                        <div class="rounded-2xl border border-dashed border-slate-200 dark:border-slate-700 p-6 text-sm text-slate-500 italic">
                                            No scheduler tasks found for this automation domain.
                                        </div>
                                    {/each}
                                {/if}

                                {#if activeAutomationTab === "maintenance"}
                                    <div class="rounded-2xl border border-rose-200 dark:border-rose-900/30 bg-rose-50 dark:bg-rose-900/10 p-4 space-y-3">
                                        <div>
                                            <p class="text-xs font-black uppercase tracking-wider text-rose-600 dark:text-rose-400">Manual Data Wipe</p>
                                            <p class="text-[11px] text-rose-700/70 dark:text-rose-300/60 mt-1">Immediately purge all historical records from the database. Settings and rules are preserved.</p>
                                        </div>
                                        <button
                                            onclick={clearHistory}
                                            class="px-4 py-2 bg-rose-600 hover:bg-rose-700 text-white rounded-xl text-[10px] font-black uppercase tracking-widest transition-all shadow-md shadow-rose-500/20"
                                        >
                                            Clear All History
                                        </button>
                                    </div>
                                {/if}
                            </div>
                        {/if}
                    </div>
                </div>
                {#if activeAutomationTab === 'upgrades'}
                                    <div class="order-last p-4 rounded-2xl border-2 border-amber-200 dark:border-amber-900/40 bg-gradient-to-br from-amber-50/80 via-white to-white dark:from-amber-900/10 dark:via-slate-900/30 dark:to-slate-900/30 space-y-4">
                                                    <div class="flex flex-wrap items-center justify-between gap-3">
                                                        <div class="flex items-center gap-2">
                                                            <span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300 border border-amber-200/70 dark:border-amber-900/40">
                                                                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6M7 4h10a2 2 0 012 2v12a2 2 0 01-2 2H7a2 2 0 01-2-2V6a2 2 0 012-2z" /></svg>
                                                            </span>
                                                            <div>
                                                                <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-wider">Default Policy</p>
                                                                <p class="text-[11px] text-slate-500 mt-0.5">Applied to new containers and containers configured to follow global automation.</p>
                                                            </div>
                                                        </div>
                                                        <span class="px-2 py-1 rounded-full text-[8px] font-black uppercase tracking-widest bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300">Defaults</span>
                                                    </div>
                                                    {#if configStore.aiActive}
                                                        <div class="flex items-center justify-between">
                                                            <div>
                                                                <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-wider">Global AI Bypass</p>
                                                                <p class="text-[11px] text-slate-500 mt-0.5 italic">Skip AI assessment for all update runs.</p>
                                                            </div>
                                                            <button
                                                                onclick={() => settings.globalBypassAi = !settings.globalBypassAi}
                                                                disabled={isLocked("globalBypassAi")}
                                                                class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.globalBypassAi ? 'bg-brand-600' : 'bg-slate-300 dark:bg-slate-700'}"
                                                                aria-label="Toggle Global AI Bypass"
                                                            >
                                                                <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.globalBypassAi ? 'right-1' : 'left-1'}"></div>
                                                            </button>
                                                        </div>
                                                    {/if}

                                                    <div class="flex items-center justify-between pt-3 border-t border-slate-200/50 dark:border-slate-700/50">
                                                        <div>
                                                            <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-wider">Watchtower Mode (Skip Health)</p>
                                                                <p class="text-[11px] text-slate-500 mt-0.5 italic">Disable health checks globally and suppress plain-Docker automatic rollback behavior.</p>
                                                        </div>
                                                        <button
                                                            onclick={() => settings.globalSkipHealthCheck = !settings.globalSkipHealthCheck}
                                                            disabled={isLocked("globalSkipHealthCheck")}
                                                            class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.globalSkipHealthCheck ? 'bg-amber-600' : 'bg-slate-300 dark:bg-slate-700'}"
                                                            aria-label="Toggle Global Health Check Skip"
                                                        >
                                                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.globalSkipHealthCheck ? 'right-1' : 'left-1'}"></div>
                                                        </button>
                                                    </div>

                                                    <div class="pt-3 border-t border-slate-200/50 dark:border-slate-700/50 space-y-3">
                                                        <div>
                                                            <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-wider">Default Upgrade Validation Profile</p>
                                                            <p class="text-[11px] text-slate-500 mt-0.5 italic">
                                                                Applied to containers that do not yet have explicit lifecycle settings. Validation URL still follows container labels / pattern-based derivation.
                                                            </p>
                                                        </div>

                                                        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
                                                            <div class="space-y-2">
                                                                <label for="default-validate-mode" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Validation Mode</label>
                                                                <select
                                                                    id="default-validate-mode"
                                                                    bind:value={settings.defaultValidateMode}
                                                                    disabled={isLocked("defaultValidateMode")}
                                                                    class="w-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                                                >
                                                                    <option value="both">Docker + HTTP</option>
                                                                    <option value="docker">Docker health only</option>
                                                                    <option value="http">HTTP only</option>
                                                                </select>
                                                            </div>
                                                            <div class="space-y-2">
                                                                <label for="default-validate-timeout" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Timeout (seconds)</label>
                                                                <input
                                                                    id="default-validate-timeout"
                                                                    type="number"
                                                                    min="1"
                                                                    max="3600"
                                                                    bind:value={settings.defaultValidateTimeoutSec}
                                                                    disabled={isLocked("defaultValidateTimeoutSec")}
                                                                    class="w-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                                                />
                                                            </div>
                                                            <div class="space-y-2">
                                                                <label for="default-validate-interval" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Probe Interval (seconds)</label>
                                                                <input
                                                                    id="default-validate-interval"
                                                                    type="number"
                                                                    min="1"
                                                                    max="300"
                                                                    bind:value={settings.defaultValidateIntervalSec}
                                                                    disabled={isLocked("defaultValidateIntervalSec")}
                                                                    class="w-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                                                                />
                                                            </div>
                                                        </div>

                                                        <div class="grid grid-cols-1 xl:grid-cols-3 gap-3">
                                                            <div class="flex items-center justify-between rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 px-3 py-2">
                                                                <div class="pr-3">
                                                                    <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-wider">Auto Rollback</p>
                                                                    <p class="text-[10px] text-slate-500">Default for new containers (plain Docker rollback pipeline)</p>
                                                                </div>
                                                                <button
                                                                    onclick={() => settings.defaultAutoRollback = !settings.defaultAutoRollback}
                                                                    disabled={isLocked("defaultAutoRollback")}
                                                                    class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.defaultAutoRollback ? 'bg-emerald-600' : 'bg-slate-300 dark:bg-slate-700'}"
                                                                    aria-label="Toggle Default Auto Rollback"
                                                                >
                                                                    <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.defaultAutoRollback ? 'right-1' : 'left-1'}"></div>
                                                                </button>
                                                            </div>

                                                            <div class="flex items-center justify-between rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 px-3 py-2">
                                                                <div class="pr-3">
                                                                    <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-wider">AI Log Validation</p>
                                                                    <p class="text-[10px] text-slate-500">Default for new containers</p>
                                                                </div>
                                                                <button
                                                                    onclick={() => settings.defaultAiValidateLogs = !settings.defaultAiValidateLogs}
                                                                    disabled={isLocked("defaultAiValidateLogs")}
                                                                    class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.defaultAiValidateLogs ? 'bg-brand-600' : 'bg-slate-300 dark:bg-slate-700'}"
                                                                    aria-label="Toggle Default AI Log Validation"
                                                                >
                                                                    <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.defaultAiValidateLogs ? 'right-1' : 'left-1'}"></div>
                                                                </button>
                                                            </div>

                                                            <div class="flex items-center justify-between rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/40 px-3 py-2">
                                                                <div class="pr-3">
                                                                    <p class="text-[10px] font-black text-slate-900 dark:text-white uppercase tracking-wider">Restart on Unhealthy</p>
                                                                    <p class="text-[10px] text-slate-500">Per-container opt-in default</p>
                                                                </div>
                                                                <button
                                                                    onclick={() => settings.defaultRestartOnUnhealthy = !settings.defaultRestartOnUnhealthy}
                                                                    disabled={isLocked("defaultRestartOnUnhealthy")}
                                                                    class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.defaultRestartOnUnhealthy ? 'bg-brand-600' : 'bg-slate-300 dark:bg-slate-700'}"
                                                                    aria-label="Toggle Default Restart on Unhealthy"
                                                                >
                                                                    <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.defaultRestartOnUnhealthy ? 'right-1' : 'left-1'}"></div>
                                                                </button>
                                                            </div>
                                                        </div>
                                                    </div>

                                                    <div class="pt-3 border-t border-slate-200/60 dark:border-slate-700/60 space-y-2">
                                                        <div class="flex items-center gap-2">
                                                            <span class="inline-flex items-center justify-center w-6 h-6 rounded-lg bg-white dark:bg-slate-900/40 border border-amber-200/70 dark:border-amber-900/40 text-amber-700 dark:text-amber-300">
                                                                <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7h16M4 12h16M4 17h16" /></svg>
                                                            </span>
                                                            <p class="text-[10px] font-black uppercase tracking-wider text-slate-900 dark:text-white">Apply Default Policy</p>
                                                        </div>
                                                        <button
                                                            onclick={bulkSetAutoApply}
                                                            disabled={bulkUpdating || !discoveredContainers.length}
                                                            class="w-full px-4 py-2.5 rounded-xl bg-amber-600 hover:bg-amber-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest transition-all shadow-lg shadow-amber-500/20"
                                                        >
                                                            {bulkUpdating ? "Applying..." : "Set All Containers To Automatic"}
                                                        </button>
                                                        <p class="text-[10px] text-slate-500 italic">Set containers to follow the global automation policy. Scheduler tasks still determine if/when update jobs run.</p>
                                                    </div>
                                    </div>
                {/if}
            </div>
        {:else if activeTab === "ai"}
            <div class="p-6 md:p-8 space-y-8 settings-pane">
                <div class="settings-pane-hero rounded-2xl border bg-gradient-to-r {settingsTabChrome.ai.accentClass} p-5">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                        <div class="max-w-3xl">
                            <div class="flex items-center gap-2 mb-2">
                                <span class="px-2 py-0.5 rounded-full bg-white/80 dark:bg-slate-900/60 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 border border-white/70 dark:border-slate-700">{settingsTabChrome.ai.badge}</span>
                                {#if configStore.aiActive}
                                    <span class="px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300 text-[8px] font-black uppercase">Verified</span>
                                {/if}
                            </div>
                            <h3 class="text-lg font-black tracking-tight text-slate-900 dark:text-white">{settingsTabChrome.ai.title}</h3>
                            <p class="text-[12px] text-slate-600 dark:text-slate-300 mt-1">{settingsTabChrome.ai.subtitle}</p>
                        </div>
                    </div>
                </div>
                <div class="inline-flex flex-wrap gap-1 bg-slate-100/95 dark:bg-slate-900/80 p-1.5 rounded-2xl border border-slate-200 dark:border-slate-800 w-fit backdrop-blur">
                    <button
                        onclick={() => activeAITab = "settings"}
                        class="px-4 py-2 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all flex items-center gap-2 {activeAITab === 'settings' ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7h16M4 12h10M4 17h16" /></svg>
                        Settings
                    </button>
                    <button
                        onclick={() => activeAITab = "costs"}
                        class="px-4 py-2 rounded-xl text-[10px] font-black uppercase tracking-widest transition-all flex items-center gap-2 {activeAITab === 'costs' ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 1.12-3 2.5S10.343 13 12 13s3 1.12 3 2.5S13.657 18 12 18m0-10V6m0 12v-2M5 12a7 7 0 1014 0 7 7 0 10-14 0z" /></svg>
                        Costs
                    </button>
                </div>
                {#if activeAITab === "settings"}
                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 flex items-center justify-between gap-4">
                    <div>
                        <p class="text-xs font-black uppercase tracking-wider text-slate-500">AI Features</p>
                        <p class="text-[11px] text-slate-500 mt-1">Disable this to fully turn off AI analysis and provider usage.</p>
                    </div>
                    <div class="flex items-center gap-3">
                        {#if settings.aiEnabled && !settings.aiTestingPassed}
                            <span class="px-2 py-1 rounded-md bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 text-[9px] font-black uppercase">Configuration Not Verified</span>
                        {/if}
                        {#if configStore.aiActive}
                            <span class="px-2 py-1 rounded-md bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 text-[9px] font-black uppercase">Active & Verified</span>
                        {/if}
                        <button
                            onclick={() => settings.aiEnabled = !settings.aiEnabled}
                            disabled={isLocked("aiEnabled")}
                            class="w-10 h-5 rounded-full relative transition-colors disabled:opacity-50 {settings.aiEnabled ? 'bg-brand-600' : 'bg-slate-300'}"
                            aria-label="Toggle AI features"
                        >
                            <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {settings.aiEnabled ? 'right-1' : 'left-1'}"></div>
                        </button>
                    </div>
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
                {/if}

                {#if activeAITab === "costs"}
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
                {/if}

                {#if activeAITab === "settings"}
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
                {/if}
            </div>

        {:else if activeTab === "integrations"}
            <div class="p-6 md:p-8 space-y-8 settings-pane">
                <div class="settings-pane-hero rounded-2xl border bg-gradient-to-r {settingsTabChrome.integrations.accentClass} p-5">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                        <div class="max-w-3xl">
                            <div class="flex items-center gap-2 mb-2">
                                <span class="px-2 py-0.5 rounded-full bg-white/80 dark:bg-slate-900/60 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 border border-white/70 dark:border-slate-700">{settingsTabChrome.integrations.badge}</span>
                                {#if settings.portainerEnabled && !settings.portainerTestingPassed}
                                    <span class="px-2 py-0.5 rounded-full bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300 text-[8px] font-black uppercase">Portainer Attention</span>
                                {/if}
                            </div>
                            <h3 class="text-lg font-black tracking-tight text-slate-900 dark:text-white">{settingsTabChrome.integrations.title}</h3>
                            <p class="text-[12px] text-slate-600 dark:text-slate-300 mt-1">{settingsTabChrome.integrations.subtitle}</p>
                        </div>
                    </div>
                </div>
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
                        <div class="flex items-center gap-3">
                            {#if settings.portainerEnabled && !settings.portainerTestingPassed}
                                <span class="px-2 py-1 rounded-md bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400 text-[9px] font-black uppercase">Connectivity Error</span>
                            {/if}
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
            <div class="p-6 md:p-8 space-y-6 settings-pane">
                <div class="settings-pane-hero rounded-2xl border bg-gradient-to-r {settingsTabChrome.system.accentClass} p-5">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                        <div class="max-w-3xl">
                            <div class="flex items-center gap-2 mb-2">
                                <span class="px-2 py-0.5 rounded-full bg-white/80 dark:bg-slate-900/60 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 border border-white/70 dark:border-slate-700">{settingsTabChrome.system.badge}</span>
                            </div>
                            <h3 class="text-lg font-black tracking-tight text-slate-900 dark:text-white">{settingsTabChrome.system.title}</h3>
                            <p class="text-[12px] text-slate-600 dark:text-slate-300 mt-1">{settingsTabChrome.system.subtitle}</p>
                        </div>
                    </div>
                </div>
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
                        <p class="text-[11px] text-slate-500">Current cap: <span class="font-bold">{formatBytesCompact(settings.clamavSnapshotMaxBytes || 0)}</span>. Increase if large container mount snapshots are skipped.</p>
                    </div>
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

        {:else if activeTab === "backups"}
            {@const composeAutoApplyTask = scheduleById("container_update_apply")}
            {@const composeBackupSweepTask = scheduleById("compose_snapshot_on_change")}
            {@const composeBackupSweepDraft = composeBackupSweepTask ? draftForTask(composeBackupSweepTask) : null}
            <div class="p-6 md:p-8 space-y-6 settings-pane">
                <div class="settings-pane-hero rounded-2xl border bg-gradient-to-r {settingsTabChrome.backups.accentClass} p-5">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                        <div class="max-w-3xl">
                            <div class="flex items-center gap-2 mb-2">
                                <span class="px-2 py-0.5 rounded-full bg-white/80 dark:bg-slate-900/60 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 border border-white/70 dark:border-slate-700">{settingsTabChrome.backups.badge}</span>
                            </div>
                            <h3 class="text-lg font-black tracking-tight text-slate-900 dark:text-white">{settingsTabChrome.backups.title}</h3>
                            <p class="text-[12px] text-slate-600 dark:text-slate-300 mt-1">
                                {settingsTabChrome.backups.subtitle}
                                Compose snapshots can run automatically before compose-managed upgrades/redeploys, and optionally as a scheduled on-change sweep.
                            </p>
                        </div>
                    </div>
                </div>

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                    <div class="flex items-center gap-2">
                        <span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">
                            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v8m-4-4h8M4 7h16v10a2 2 0 01-2 2H6a2 2 0 01-2-2V7z" /></svg>
                        </span>
                        <div>
                            <p class="text-sm font-black text-slate-800 dark:text-slate-100">When Compose Backups Run</p>
                            <p class="text-[11px] text-slate-500 mt-1">Two independent triggers can create compose snapshots. Use both if you want baseline backups and pre-upgrade protection.</p>
                        </div>
                    </div>

                    <div class="grid grid-cols-1 xl:grid-cols-2 gap-3">
                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3 space-y-2">
                            <div class="flex items-center justify-between gap-2">
                                <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Before Compose Auto-Apply</p>
                                <span class="px-2 py-0.5 rounded-md text-[9px] font-black uppercase tracking-widest {(composeAutoApplyTask?.enabled ?? false) ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-slate-200 text-slate-600 dark:bg-slate-800 dark:text-slate-300'}">
                                    {(composeAutoApplyTask?.enabled ?? false) ? "Enabled" : "Disabled"}
                                </span>
                            </div>
                            <p class="text-[11px] text-slate-600 dark:text-slate-300">Snapshots are created immediately before compose-managed upgrade/redeploy actions run through the upgrade automation pipeline.</p>
                            <p class="text-[10px] uppercase tracking-wider text-slate-500 font-bold">
                                {#if composeAutoApplyTask}
                                    Upgrade schedule: {cronLabel(composeAutoApplyTask.cronSpec)} | Last run: {formatTime(composeAutoApplyTask.lastRun)}
                                {:else}
                                    Upgrade auto-apply scheduler task is not registered.
                                {/if}
                            </p>
                            <button
                                onclick={() => { activeTab = "automations"; activeAutomationTab = "upgrades"; }}
                                class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800"
                            >Open Upgrades Schedule</button>
                        </div>

                        <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3 space-y-2">
                            <div class="flex items-center justify-between gap-2">
                                <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Scheduled Compose Backup Sweep</p>
                                <span class="px-2 py-0.5 rounded-md text-[9px] font-black uppercase tracking-widest {(composeBackupSweepTask?.enabled ?? false) ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-slate-200 text-slate-600 dark:bg-slate-800 dark:text-slate-300'}">
                                    {(composeBackupSweepTask?.enabled ?? false) ? "Enabled" : "Disabled"}
                                </span>
                            </div>
                            <p class="text-[11px] text-slate-600 dark:text-slate-300">Runs on a cron schedule and creates snapshots only when local Compose project files changed (includes project <span class="font-mono">.env</span> when present).</p>
                            <p class="text-[10px] uppercase tracking-wider text-slate-500 font-bold">
                                {#if composeBackupSweepTask}
                                    {cronLabel(composeBackupSweepTask.cronSpec)} | Last run: {formatTime(composeBackupSweepTask.lastRun)}
                                {:else}
                                    Compose backup scheduler task is not registered.
                                {/if}
                            </p>
                        </div>
                    </div>
                </div>

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                    <div class="flex items-center gap-2">
                        <span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300 border border-emerald-200/70 dark:border-emerald-900/40">
                            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7h16v10a2 2 0 01-2 2H6a2 2 0 01-2-2V7zm0 0l2-3h12l2 3" /></svg>
                        </span>
                        <div>
                            <p class="text-sm font-black text-slate-800 dark:text-slate-100">Compose Backups</p>
                            <p class="text-[11px] text-slate-500 mt-1">Configure where HarborWatch stores zipped snapshots of Compose files and project <span class="font-mono">.env</span> before compose upgrades.</p>
                        </div>
                    </div>

                    <div class="space-y-2">
                        <label for="compose-snapshot-root" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Snapshot Archive Path</label>
                        <input
                            id="compose-snapshot-root"
                            type="text"
                            bind:value={settings.composeSnapshotRootPath}
                            disabled={isLocked("composeSnapshotRootPath")}
                            placeholder="/data/compose-snapshots"
                            class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                        />
                        <p class="text-[11px] text-slate-500">Leave blank to use the appliance default snapshot location.</p>
                    </div>
                </div>

                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                    <div class="flex items-center gap-2">
                        <span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300 border border-indigo-200/70 dark:border-indigo-900/40">
                            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 4a3 3 0 00-3 3v4a3 3 0 003 3h4a3 3 0 003-3V7a3 3 0 00-3-3H8zM2 14a2 2 0 012-2h16a2 2 0 012 2v2a2 2 0 01-2 2H4a2 2 0 01-2-2v-2z" /></svg>
                        </span>
                        <div>
                            <p class="text-sm font-black text-slate-800 dark:text-slate-100">GitOps Storage</p>
                            <p class="text-[11px] text-slate-500 mt-1">Configure where HarborWatch stores synchronized Git repositories.</p>
                        </div>
                    </div>

                    <div class="space-y-2">
                        <label for="gitops-master-dir" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Master GitOps Directory</label>
                        <input
                            id="gitops-master-dir"
                            type="text"
                            bind:value={settings.gitOpsMasterDirectory}
                            disabled={isLocked("gitOpsMasterDirectory")}
                            placeholder="/data/gitops"
                            class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs font-mono outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-60"
                        />
                        <p class="text-[11px] text-slate-500">Local path where repositories will be cloned. Default is <span class="font-mono">/data/gitops</span>.</p>
                    </div>
                </div>

                {#if composeBackupSweepTask && composeBackupSweepDraft}
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 p-4 space-y-4">
                        <div class="flex flex-wrap items-center justify-between gap-3">
                            <div>
                                <div class="flex items-center gap-2">
                                    <span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                                        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                                    </span>
                                    <p class="text-sm font-black text-slate-800 dark:text-slate-100">Scheduled Compose Backup Sweep</p>
                                </div>
                                <p class="text-[11px] text-slate-500 mt-1">Mode is <span class="font-bold">on change</span>: unchanged projects are skipped. This task only applies to local Compose projects with readable compose files.</p>
                                <p class="text-[10px] uppercase tracking-wider text-slate-500 font-bold mt-1">{taskCronLabel(composeBackupSweepTask)} | Last run: {formatTime(composeBackupSweepTask.lastRun)}</p>
                            </div>
                            <div class="flex items-center gap-2">
                                <button
                                    onclick={() => runTask(composeBackupSweepTask.id)}
                                    class="px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-700 text-[10px] font-black uppercase tracking-widest hover:bg-slate-50 dark:hover:bg-slate-800"
                                >Run Now</button>
                                <button
                                    onclick={() => toggleTask(composeBackupSweepTask.id, composeBackupSweepTask.enabled)}
                                    class="w-10 h-5 rounded-full relative transition-colors {composeBackupSweepTask.enabled ? 'bg-brand-600' : 'bg-slate-300'}"
                                    aria-label="Toggle scheduled compose backup sweep"
                                >
                                    <div class="absolute top-1 w-3 h-3 rounded-full bg-white transition-all {composeBackupSweepTask.enabled ? 'right-1' : 'left-1'}"></div>
                                </button>
                            </div>
                        </div>

                        <div class="grid grid-cols-1 md:grid-cols-[1fr_1fr_auto] gap-3 items-end">
                            <div class="space-y-1">
                                <label for="cadence-compose-backup-sweep" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Cadence</label>
                                <select
                                    id="cadence-compose-backup-sweep"
                                    value={composeBackupSweepDraft.cadence}
                                    onchange={(e) => patchScheduleDraft(composeBackupSweepTask.id, { cadence: (e.currentTarget as HTMLSelectElement).value as ScheduleCadence })}
                                    class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                >
                                    <option value="daily">Daily</option>
                                    <option value="weekly">Weekly</option>
                                    <option value="monthly">Monthly</option>
                                </select>
                            </div>
                            <div class="space-y-1">
                                <label for="time-compose-backup-sweep" class="text-[10px] font-black uppercase tracking-wider text-slate-400">Run Time</label>
                                <input
                                    id="time-compose-backup-sweep"
                                    type="time"
                                    value={composeBackupSweepDraft.time}
                                    onchange={(e) => patchScheduleDraft(composeBackupSweepTask.id, { time: (e.currentTarget as HTMLInputElement).value || "00:00" })}
                                    class="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
                                />
                            </div>
                            <button
                                onclick={() => saveTaskSchedule(composeBackupSweepTask.id)}
                                disabled={!scheduleDirty(composeBackupSweepTask) || savingScheduleId === composeBackupSweepTask.id}
                                class="px-3 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest"
                            >
                                {savingScheduleId === composeBackupSweepTask.id ? "Saving..." : "Save Schedule"}
                            </button>
                        </div>

                        {#if composeBackupSweepDraft.cadence === "weekly"}
                            <div class="flex flex-wrap gap-2">
                                {#each weekdayOptions as day}
                                    <button
                                        onclick={() => toggleWeeklyDay(composeBackupSweepTask.id, day.value)}
                                        class="px-2.5 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-wider border transition-colors {composeBackupSweepDraft.weeklyDays.includes(day.value) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                    >{day.label}</button>
                                {/each}
                            </div>
                        {:else if composeBackupSweepDraft.cadence === "monthly"}
                            <div class="flex flex-wrap gap-1.5">
                                {#each monthDayOptions as day}
                                    <button
                                        onclick={() => toggleMonthDay(composeBackupSweepTask.id, day)}
                                        class="min-w-8 px-2 py-1 rounded-lg text-[10px] font-black border transition-colors {composeBackupSweepDraft.monthDays.includes(day) ? 'bg-brand-600 text-white border-brand-600' : 'bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 border-slate-200 dark:border-slate-700'}"
                                    >{day}</button>
                                {/each}
                            </div>
                        {/if}
                    </div>
                {/if}
            </div>

        {:else if activeTab === "appearance"}
            <div class="p-6 md:p-8 space-y-6 settings-pane">
                <div class="settings-pane-hero rounded-2xl border bg-gradient-to-r {settingsTabChrome.appearance.accentClass} p-5">
                    <div class="flex flex-wrap items-start justify-between gap-3">
                        <div class="max-w-3xl">
                            <div class="flex items-center gap-2 mb-2">
                                <span class="px-2 py-0.5 rounded-full bg-white/80 dark:bg-slate-900/60 text-[9px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 border border-white/70 dark:border-slate-700">{settingsTabChrome.appearance.badge}</span>
                            </div>
                            <h3 class="text-lg font-black tracking-tight text-slate-900 dark:text-white">{settingsTabChrome.appearance.title}</h3>
                            <p class="text-[12px] text-slate-600 dark:text-slate-300 mt-1">{settingsTabChrome.appearance.subtitle}</p>
                        </div>
                    </div>
                </div>
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

<style>
    .settings-page {
        position: relative;
    }

    .settings-topbar {
        position: static;
        z-index: 30;
        padding: 0.9rem 1rem;
        border-radius: 1rem;
        border: 1px solid rgb(226 232 240 / 0.9);
        background:
            radial-gradient(circle at top right, rgb(14 165 233 / 0.08), transparent 45%),
            radial-gradient(circle at top left, rgb(99 102 241 / 0.08), transparent 50%),
            rgb(255 255 255 / 0.9);
        backdrop-filter: blur(10px);
        box-shadow: 0 12px 30px -24px rgb(15 23 42 / 0.35);
    }

    .settings-tab-strip {
        position: static;
        z-index: 25;
        box-shadow: 0 8px 24px -22px rgb(15 23 42 / 0.4);
    }

    .settings-subtab-strip {
        position: static;
        z-index: 15;
        box-shadow: 0 8px 24px -24px rgb(15 23 42 / 0.35);
    }

    .settings-shell {
        background:
            linear-gradient(to bottom, rgb(248 250 252), rgb(255 255 255) 14rem),
            rgb(255 255 255);
        box-shadow:
            0 18px 50px -32px rgb(15 23 42 / 0.28),
            inset 0 1px 0 rgb(255 255 255 / 0.7);
    }

    .settings-pane {
        position: relative;
    }

    .settings-pane::before {
        content: "";
        position: absolute;
        inset: 0;
        pointer-events: none;
        background-image:
            linear-gradient(rgb(148 163 184 / 0.06) 1px, transparent 1px),
            linear-gradient(90deg, rgb(148 163 184 / 0.06) 1px, transparent 1px);
        background-size: 18px 18px;
        mask-image: linear-gradient(to bottom, rgb(0 0 0 / 0.14), transparent 22%);
    }

    .settings-pane > * {
        position: relative;
        z-index: 1;
    }

    .settings-pane-hero {
        box-shadow:
            inset 0 1px 0 rgb(255 255 255 / 0.7),
            0 10px 30px -24px rgb(15 23 42 / 0.25);
    }

    :global(.dark) .settings-topbar {
        border-color: rgb(51 65 85 / 0.9);
        background:
            radial-gradient(circle at top right, rgb(14 165 233 / 0.12), transparent 45%),
            radial-gradient(circle at top left, rgb(99 102 241 / 0.12), transparent 50%),
            rgb(15 23 42 / 0.9);
        box-shadow: 0 14px 40px -30px rgb(0 0 0 / 0.6);
    }

    :global(.dark) .settings-shell {
        background:
            linear-gradient(to bottom, rgb(15 23 42), rgb(30 41 59) 14rem),
            rgb(30 41 59);
        box-shadow:
            0 20px 48px -28px rgb(0 0 0 / 0.55),
            inset 0 1px 0 rgb(255 255 255 / 0.03);
    }

    :global(.dark) .settings-pane::before {
        background-image:
            linear-gradient(rgb(148 163 184 / 0.05) 1px, transparent 1px),
            linear-gradient(90deg, rgb(148 163 184 / 0.05) 1px, transparent 1px);
    }

    @media (min-width: 768px) {
        .settings-topbar {
            position: sticky;
            top: 0.5rem;
        }

        .settings-tab-strip {
            position: sticky;
            top: 7.1rem;
        }

        .settings-subtab-strip {
            position: sticky;
            top: 10.7rem;
        }
    }
</style>
