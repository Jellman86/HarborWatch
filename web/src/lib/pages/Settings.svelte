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
        defaultValidateMode: "docker",
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
        if (domain === "remediation") return settings.unhealthyAutoRemediationEnabled ?? false;
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
        const defaultValidateMode = "docker";
        const defaultValidateTimeoutSec = Math.max(1, Number(settings.defaultValidateTimeoutSec || 45));
        const defaultValidateIntervalSec = Math.max(1, Number(settings.defaultValidateIntervalSec || 2));
        const defaultRestartCooldown = Math.max(0, Number(settings.unhealthyRestartCooldownSecDefault || 300));

        try {
            // We do this in parallel but with a small limit if there are many, 
            // though for most home labs Discovery list is small enough for Promise.all
            const results = await Promise.allSettled(discoveredContainers.map(async (c) => {
                const containerName = String((c.names || [])[0] || "").replace(/^\//, "").trim();
                const res = await fetch(`/api/docker/${encodeURIComponent(c.id)}/rules`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        containerId: c.id,
                        containerName: containerName || undefined,
                        updatePolicy: "auto",
                        inheritAutomation: true,
                        upgradesAutomation: true,
                        maintenanceAutomation: true,
                        securityAutomation: true,
                        validateMode: defaultValidateMode,
                        validateTimeoutSec: defaultValidateTimeoutSec,
                        validateIntervalSec: defaultValidateIntervalSec,
                        aiValidateLogs: !!settings.defaultAiValidateLogs,
                        autoRollback: !!settings.defaultAutoRollback,
                        restartOnUnhealthy: !!settings.defaultRestartOnUnhealthy,
                        unhealthyRestartCooldownSec: defaultRestartCooldown
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
                toasts.success(`Successfully set all ${success} containers to Automatic with Docker health validation defaults.`);
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

<!-- ============================================================
     SETTINGS PAGE TEMPLATE
     HarborWatch · Svelte 5 · Tailwind dark mode
     ============================================================ -->

<!-- ── Shared snippets ───────────────────────────────────────── -->

{#snippet toggleSwitch(value: boolean, onChange: () => void, locked: boolean)}
    <button
        role="switch"
        aria-checked={value}
        onclick={onChange}
        disabled={locked}
        class="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 focus:outline-none disabled:opacity-50 {value ? 'bg-brand-500' : 'bg-slate-200 dark:bg-slate-700'}"
    >
        <span class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out {value ? 'translate-x-4' : 'translate-x-0'}"></span>
    </button>
{/snippet}

{#snippet settingRow(label: string, description: string, value: boolean, onChange: () => void, lockedKey: string)}
    <div class="flex items-center justify-between gap-4 py-3 border-b border-slate-50 dark:border-slate-800/50 last:border-0">
        <div class="min-w-0">
            <p class="text-sm font-medium text-slate-800 dark:text-slate-200">{label}</p>
            {#if description}
                <p class="text-[11px] text-slate-400 mt-0.5">{description}</p>
            {/if}
            {#if isLocked(lockedKey)}
                <p class="text-[10px] text-amber-500 mt-0.5">Managed by environment variable</p>
            {/if}
        </div>
        <div class="flex-none">
            {@render toggleSwitch(value, onChange, isLocked(lockedKey))}
        </div>
    </div>
{/snippet}

{#snippet sectionCard(title: string, description: string)}
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">{title}</h3>
            {#if description}
                <p class="text-[11px] text-slate-500 mt-0.5">{description}</p>
            {/if}
        </div>
    </div>
{/snippet}

{#snippet taskCard(task: { id: string; cronSpec: string; enabled: boolean; lastRun?: number })}
    {@const draft = draftForTask(task)}
    {@const isUpdateCheck = isUpdateCheckIntervalTask(task.id)}
    {@const dirty = scheduleDirty(task)}
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
        <!-- Task header -->
        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
            <div class="flex items-start justify-between gap-3">
                <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-2">
                        <span class="inline-block w-2 h-2 rounded-full flex-none {task.enabled ? 'bg-emerald-400' : 'bg-slate-300 dark:bg-slate-600'}"></span>
                        <p class="text-sm font-bold text-slate-800 dark:text-slate-100">{taskLabel(task.id)}</p>
                    </div>
                    <p class="text-[11px] text-slate-400 mt-1 ml-4">{taskDescription(task.id)}</p>
                    <p class="text-[10px] text-slate-400 mt-1 ml-4">
                        Schedule: <span class="font-medium text-slate-600 dark:text-slate-300">{taskCronLabel(task)}</span>
                        &nbsp;·&nbsp;Last run: <span class="font-medium text-slate-600 dark:text-slate-300">{formatTime(task.lastRun)}</span>
                    </p>
                </div>
                <div class="flex-none">
                    {@render toggleSwitch(task.enabled, () => toggleTask(task.id, task.enabled), false)}
                </div>
            </div>
        </div>

        <!-- Schedule editor body -->
        <div class="px-5 py-4 space-y-4 bg-slate-50/50 dark:bg-slate-800/20">
            {#if isUpdateCheck}
                <div class="space-y-1.5">
                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Check Interval</label>
                    <select
                        value={updateCheckIntervalDraftForTask(task)}
                        onchange={(e) => handleUpdateCheckIntervalChange(task, (e.target as HTMLSelectElement).value as any)}
                        class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 dark:focus:border-brand-500 transition-colors"
                    >
                        {#each updateCheckIntervalOptions as opt}
                            <option value={opt.value}>{opt.label}</option>
                        {/each}
                    </select>
                </div>
            {:else}
                <div class="space-y-1.5">
                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Cadence</label>
                    <select
                        value={draft.cadence}
                        onchange={(e) => patchScheduleDraft(task.id, { cadence: (e.target as HTMLSelectElement).value as any })}
                        class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors"
                    >
                        <option value="daily">Daily</option>
                        <option value="weekly">Weekly</option>
                        <option value="monthly">Monthly</option>
                    </select>
                </div>
                <div class="space-y-1.5">
                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Time (24h)</label>
                    <input
                        type="time"
                        value={draft.time}
                        oninput={(e) => patchScheduleDraft(task.id, { time: (e.target as HTMLInputElement).value })}
                        class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors"
                    />
                </div>
                {#if draft.cadence === "weekly"}
                    <div class="space-y-2">
                        <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Days of Week</label>
                        <div class="flex flex-wrap gap-1.5">
                            {#each weekdayOptions as day}
                                <button
                                    onclick={() => toggleWeeklyDay(task.id, day.value)}
                                    class="px-2.5 py-1 rounded-lg text-[11px] font-bold transition-colors {draft.weeklyDays.includes(day.value) ? 'bg-brand-500 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-700'}"
                                >{day.label}</button>
                            {/each}
                        </div>
                    </div>
                {/if}
                {#if draft.cadence === "monthly"}
                    <div class="space-y-2">
                        <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Days of Month</label>
                        <div class="flex flex-wrap gap-1">
                            {#each monthDayOptions as day}
                                <button
                                    onclick={() => toggleMonthDay(task.id, day)}
                                    class="w-7 h-7 rounded-lg text-[11px] font-bold transition-colors {draft.monthDays.includes(day) ? 'bg-brand-500 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-700'}"
                                >{day}</button>
                            {/each}
                        </div>
                    </div>
                {/if}
            {/if}
        </div>

        <!-- Footer actions -->
        <div class="px-5 py-3 border-t border-slate-100 dark:border-slate-800/80 flex items-center justify-between gap-3">
            <button
                onclick={() => saveTaskSchedule(task.id)}
                disabled={!dirty || savingScheduleId === task.id}
                class="px-4 py-1.5 rounded-xl bg-brand-500 hover:bg-brand-600 disabled:opacity-40 text-white text-[11px] font-black uppercase tracking-widest transition-colors"
            >
                {savingScheduleId === task.id ? "Saving…" : "Save Schedule"}
            </button>
            <button
                onclick={() => runTask(task.id)}
                class="px-4 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-50 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 text-[11px] font-black uppercase tracking-widest transition-colors"
            >
                Run Now
            </button>
        </div>
    </div>
{/snippet}

<!-- ── Main page wrapper ──────────────────────────────────────── -->
<div class="settings-page w-full">

    <!-- Sticky top bar -->
    <div class="settings-topbar sticky top-0 z-20 flex flex-col sm:flex-row sm:items-center justify-between gap-3 px-4 md:px-6 py-3 md:py-4 border-b border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-950/90 backdrop-blur-md">
        <div>
            <h1 class="text-xl font-black text-slate-900 dark:text-white tracking-tight">Settings</h1>
            <p class="text-[11px] text-slate-500 mt-0.5">Configure automation, AI, integrations and system behaviour</p>
        </div>
        <div class="flex items-center gap-3">
            <span
                class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-[11px] font-bold {indicatorToneClass(settingsIndicatorState())}"
                title={settingsIndicatorState() === 'saving' ? 'Saving…' : settingsIndicatorState() === 'error' ? 'Save failed' : settingsIndicatorState() === 'pending' ? 'Unsaved changes' : 'All changes saved'}
            >
                <span class="inline-flex items-center justify-center w-4 h-4 rounded-full border border-current/20 bg-white/60 dark:bg-slate-900/40 text-[10px] font-black">{indicatorSymbol(settingsIndicatorState())}</span>
                {settingsIndicatorState() === 'saving' ? 'Saving…' : settingsIndicatorState() === 'error' ? 'Error' : settingsIndicatorState() === 'pending' ? 'Unsaved' : 'Saved'}
            </span>
            <button
                onclick={saveSettings}
                disabled={saving || loading || !settingsDirty}
                class="px-5 py-2 bg-brand-500 hover:bg-brand-600 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-xl font-black uppercase tracking-widest text-[10px] shadow-sm transition-colors"
            >
                {saving ? 'Saving…' : settingsDirty ? 'Save Settings' : 'Saved'}
            </button>
        </div>
    </div>

    <!-- Mobile pill nav — full width, above the body -->
    <div class="md:hidden flex gap-2 overflow-x-auto px-4 py-3 border-b border-slate-100 dark:border-slate-800 settings-mobile-nav">
        {#each [
            { id: 'automations', label: 'Automation' },
            { id: 'ai', label: 'AI' },
            { id: 'integrations', label: 'Integrations' },
            { id: 'system', label: 'System' },
            { id: 'backups', label: 'Backups' },
            { id: 'appearance', label: 'Appearance' }
        ] as nav}
            <button
                onclick={() => activeTab = nav.id}
                class="flex-none px-4 py-1.5 rounded-full text-[11px] font-black uppercase tracking-widest whitespace-nowrap transition-colors
                    {activeTab === nav.id ? 'bg-brand-500 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400'}"
            >{nav.label}</button>
        {/each}
    </div>

    <!-- Body -->
    <div class="flex flex-col md:flex-row gap-0 mt-4 md:mt-6 px-4 md:px-6">

        <!-- Left sidebar nav (md+) -->
        <nav class="hidden md:flex flex-col w-52 flex-none gap-0.5 pr-6 pt-1">
            {#each [
                { id: 'automations', label: 'Automation', icon: 'cog' },
                { id: 'ai', label: 'AI & Providers', icon: 'sparkles' },
                { id: 'integrations', label: 'Integrations', icon: 'link' },
                { id: 'system', label: 'System', icon: 'server' },
                { id: 'backups', label: 'Backups', icon: 'archive' },
                { id: 'appearance', label: 'Appearance', icon: 'brush' }
            ] as nav}
                <button
                    onclick={() => activeTab = nav.id}
                    class="flex items-center gap-2.5 px-3 py-2.5 rounded-xl text-left transition-all text-[12px] font-semibold w-full
                        {activeTab === nav.id
                            ? 'bg-brand-50 dark:bg-slate-800 border-l-2 border-brand-500 text-brand-700 dark:text-brand-300 pl-[10px]'
                            : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 border-l-2 border-transparent'}"
                >
                    <span class="flex-none w-4 h-4 text-current opacity-70">
                        {#if nav.icon === 'cog'}
                            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
                        {:else if nav.icon === 'sparkles'}
                            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z" /></svg>
                        {:else if nav.icon === 'link'}
                            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" /></svg>
                        {:else if nav.icon === 'server'}
                            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" /></svg>
                        {:else if nav.icon === 'archive'}
                            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" /></svg>
                        {:else}
                            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" /></svg>
                        {/if}
                    </span>
                    <span class="flex-1 truncate">{nav.label}</span>
                    {#if nav.id === 'automations'}
                        {@const anyEnabled = (['upgrades','maintenance','security','remediation'] as AutomationDomain[]).some(d => domainEnabled(d))}
                        <span class="w-1.5 h-1.5 rounded-full flex-none {anyEnabled ? 'bg-emerald-400' : 'bg-slate-300 dark:bg-slate-600'}"></span>
                    {/if}
                </button>
            {/each}

            <!-- Divider + version info -->
            <div class="mt-4 pt-4 border-t border-slate-100 dark:border-slate-800">
                <p class="text-[10px] text-slate-400 px-3">HarborWatch</p>
            </div>
        </nav>

        <!-- Main content -->
        <div class="flex-1 min-w-0 space-y-6 pb-16">

            {#if loading}
                <!-- Loading skeleton -->
                <div class="space-y-4">
                    {#each [1,2,3] as _}
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 h-32 animate-pulse"></div>
                    {/each}
                </div>

            {:else if activeTab === 'automations'}
                <!-- ================================================
                     AUTOMATIONS TAB
                     ================================================ -->

                <!-- Domain sub-nav pills -->
                <div class="flex flex-wrap gap-2">
                    {#each (['general','upgrades','maintenance','security','remediation'] as AutomationDomain[]) as domain}
                        <button
                            onclick={() => activeAutomationTab = domain}
                            class="flex items-center gap-2 px-4 py-2 rounded-full text-[11px] font-black uppercase tracking-widest transition-colors
                                {activeAutomationTab === domain ? 'bg-brand-500 text-white shadow-sm' : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-700'}"
                        >
                            {#if domain !== 'general'}
                                <span class="w-1.5 h-1.5 rounded-full flex-none {domainEnabled(domain) ? 'bg-emerald-400' : 'bg-slate-400 dark:bg-slate-500'}
                                    {activeAutomationTab === domain ? 'opacity-100' : ''}"></span>
                            {/if}
                            {automationConfig[domain].title.split(' ')[0]}
                        </button>
                    {/each}
                </div>

                <!-- Domain header card -->
                {#if activeAutomationTab !== 'general'}
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm overflow-hidden">
                        <div class="px-5 py-4 flex items-center justify-between gap-4">
                            <div class="min-w-0">
                                <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">{automationConfig[activeAutomationTab].title}</h3>
                                <p class="text-[11px] text-slate-500 mt-0.5">{automationConfig[activeAutomationTab].subtitle}</p>
                            </div>
                            <div class="flex items-center gap-3 flex-none">
                                <span class="text-[11px] text-slate-500">{domainEnabled(activeAutomationTab) ? 'Enabled' : 'Disabled'}</span>
                                {@render toggleSwitch(domainEnabled(activeAutomationTab), () => setDomainEnabled(activeAutomationTab, !domainEnabled(activeAutomationTab)), domainToggleBusy !== '')}
                            </div>
                        </div>
                    </div>
                {/if}

                <!-- Flow chart -->
                <AutomationFlowChart
                    title={automationConfig[activeAutomationTab].title}
                    subtitle={automationConfig[activeAutomationTab].subtitle}
                    steps={flowSteps(activeAutomationTab)}
                    accent={automationConfig[activeAutomationTab].accent}
                />

                <!-- ── GENERAL DOMAIN ── -->
                {#if activeAutomationTab === 'general'}
                    <!-- Capacity -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Capacity</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">Limit parallel upgrade operations to protect host stability.</p>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Max Concurrent Upgrades</label>
                                <input
                                    bind:value={settings.autoUpgradeMaxConcurrency}
                                    type="number" min="1" max="20"
                                    disabled={isLocked('autoUpgradeMaxConcurrency')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 dark:focus:border-brand-500 transition-colors disabled:opacity-50"
                                />
                                {#if isLocked('autoUpgradeMaxConcurrency')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                        </div>
                    </div>

                    <!-- Automation Safety Exclusions -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Automation Safety Exclusions</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">These containers are excluded from all automated upgrade, maintenance, and security pipelines.</p>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            {#if discoveredContainers.length > 0}
                                <div class="space-y-1">
                                    <p class="text-[11px] font-black uppercase tracking-widest text-slate-500 mb-2">Discovered Containers</p>
                                    <div class="max-h-64 overflow-y-auto space-y-1 pr-1">
                                        {#each discoveredContainers as container}
                                            {@const hw = isHarborWatchContainer(container)}
                                            {@const ignored = isContainerIgnored(container)}
                                            <div class="flex items-center justify-between gap-3 px-3 py-2 rounded-xl {ignored ? 'bg-amber-50 dark:bg-amber-900/10 border border-amber-100 dark:border-amber-900/30' : 'bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50'}">
                                                <div class="min-w-0 flex-1">
                                                    <p class="text-[12px] font-semibold text-slate-800 dark:text-slate-200 truncate">{containerDisplayName(container)}</p>
                                                    <p class="text-[10px] text-slate-400 truncate">{container.image || ''}</p>
                                                </div>
                                                <div class="flex items-center gap-2 flex-none">
                                                    {#if hw}
                                                        <span class="text-[10px] text-amber-500 font-bold">Protected</span>
                                                    {/if}
                                                    {@render toggleSwitch(ignored, () => setContainerIgnored(container, !ignored), hw)}
                                                </div>
                                            </div>
                                        {/each}
                                    </div>
                                </div>
                            {/if}

                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Raw Exclusion Tokens</label>
                                <textarea
                                    bind:value={settings.automationIgnoredContainers}
                                    rows="3"
                                    placeholder="harborwatch, my-db, nginx"
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors resize-none font-mono"
                                ></textarea>
                                <p class="text-[10px] text-slate-400">Comma, semicolon, or newline separated. Matches container name, image, or ID substring.</p>
                            </div>
                        </div>
                    </div>

                    <!-- Default Upgrade Policy -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Default Upgrade Policy</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">Applied when creating new container rules without explicit overrides.</p>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Validation Mode</label>
                                <select
                                    bind:value={settings.defaultValidateMode}
                                    disabled={isLocked('defaultValidateMode')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                >
                                    <option value="docker">Docker Health Check</option>
                                    <option value="http">HTTP Probe</option>
                                    <option value="none">None</option>
                                </select>
                            </div>
                            <div class="grid grid-cols-2 gap-4">
                                <div class="space-y-1.5">
                                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Timeout (sec)</label>
                                    <input
                                        bind:value={settings.defaultValidateTimeoutSec}
                                        type="number" min="5" max="300"
                                        disabled={isLocked('defaultValidateTimeoutSec')}
                                        class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                    />
                                </div>
                                <div class="space-y-1.5">
                                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Poll Interval (sec)</label>
                                    <input
                                        bind:value={settings.defaultValidateIntervalSec}
                                        type="number" min="1" max="30"
                                        disabled={isLocked('defaultValidateIntervalSec')}
                                        class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                    />
                                </div>
                            </div>
                            {@render settingRow('AI Log Validation', 'Use AI to analyse container logs after upgrades.', settings.defaultAiValidateLogs ?? false, () => settings.defaultAiValidateLogs = !settings.defaultAiValidateLogs, 'defaultAiValidateLogs')}
                            {@render settingRow('Auto Rollback', 'Automatically rollback on health check failure.', settings.defaultAutoRollback ?? true, () => settings.defaultAutoRollback = !settings.defaultAutoRollback, 'defaultAutoRollback')}
                            {@render settingRow('Restart on Unhealthy', 'Restart container when health check fails post-upgrade.', settings.defaultRestartOnUnhealthy ?? false, () => settings.defaultRestartOnUnhealthy = !settings.defaultRestartOnUnhealthy, 'defaultRestartOnUnhealthy')}

                            <div class="pt-2 border-t border-slate-100 dark:border-slate-800/50">
                                <button
                                    onclick={bulkSetAutoApply}
                                    disabled={bulkUpdating || !discoveredContainers.length}
                                    class="w-full px-4 py-2.5 rounded-xl border border-brand-200 dark:border-brand-800 bg-brand-50 dark:bg-brand-900/20 hover:bg-brand-100 dark:hover:bg-brand-900/40 text-brand-700 dark:text-brand-300 text-[11px] font-black uppercase tracking-widest transition-colors disabled:opacity-50"
                                >
                                    {bulkUpdating ? 'Applying…' : `Apply Defaults to All Containers (${discoveredContainers.length})`}
                                </button>
                            </div>
                        </div>
                    </div>

                <!-- ── UPGRADES DOMAIN ── -->
                {:else if activeAutomationTab === 'upgrades'}
                    {#each schedulesForDomain('upgrades') as task}
                        {@render taskCard(task)}
                    {/each}

                    <!-- Upgrade Runtime -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Upgrade Runtime</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">Retry, AI gating, and health check policy for the upgrade pipeline.</p>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Min Retry Cooldown (minutes)</label>
                                <input
                                    bind:value={settings.autoUpgradeMinRetryMinutes}
                                    type="number" min="1" max="1440"
                                    disabled={isLocked('autoUpgradeMinRetryMinutes')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                />
                                {#if isLocked('autoUpgradeMinRetryMinutes')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                            {@render settingRow('Bypass AI Risk Gate', 'Skip AI risk assessment and always proceed with upgrades.', settings.globalBypassAi ?? false, () => settings.globalBypassAi = !settings.globalBypassAi, 'globalBypassAi')}
                            {@render settingRow('Skip Health Check', 'Proceed with upgrades without post-deploy health verification.', settings.globalSkipHealthCheck ?? false, () => settings.globalSkipHealthCheck = !settings.globalSkipHealthCheck, 'globalSkipHealthCheck')}
                        </div>
                    </div>

                <!-- ── MAINTENANCE DOMAIN ── -->
                {:else if activeAutomationTab === 'maintenance'}
                    <!-- Data Retention -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Data Retention Window</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">How long historical records are kept before automated pruning. Applies to logs, metrics, scans, update runs, and AI usage.</p>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="flex flex-wrap gap-2">
                                {#each (['7d','30d','90d','365d'] as RetentionWindowPreset[]) as preset}
                                    <button
                                        onclick={() => applyRetentionWindowPreset(preset)}
                                        class="px-4 py-2 rounded-xl text-[11px] font-black uppercase tracking-widest transition-colors
                                            {retentionWindowPreset === preset ? 'bg-brand-500 text-white shadow-sm' : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-700'}"
                                    >{preset}</button>
                                {/each}
                            </div>
                            <div class="grid grid-cols-2 gap-3 text-[11px]">
                                <div class="px-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                    <p class="text-slate-500 font-medium">Logs</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5">{settings.retentionLogsDays}d</p>
                                </div>
                                <div class="px-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                    <p class="text-slate-500 font-medium">Metrics</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5">{settings.retentionMetricsDays}d</p>
                                </div>
                                <div class="px-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                    <p class="text-slate-500 font-medium">Scan Results</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5">{settings.retentionScanResultsDays}d</p>
                                </div>
                                <div class="px-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                    <p class="text-slate-500 font-medium">AI Usage</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5">{settings.retentionAIUsageDays}d</p>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Docker Prune task -->
                    {#each schedules.filter(s => s.id === 'docker_system_prune') as task}
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                            <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                                <div class="flex items-start justify-between gap-3">
                                    <div class="min-w-0 flex-1">
                                        <div class="flex items-center gap-2">
                                            <span class="inline-block w-2 h-2 rounded-full flex-none {task.enabled ? 'bg-emerald-400' : 'bg-slate-300 dark:bg-slate-600'}"></span>
                                            <p class="text-sm font-bold text-slate-800 dark:text-slate-100">{taskLabel(task.id)}</p>
                                        </div>
                                        <p class="text-[11px] text-slate-400 mt-1 ml-4">{taskDescription(task.id)}</p>
                                    </div>
                                    {@render toggleSwitch(task.enabled, () => toggleTask(task.id, task.enabled), false)}
                                </div>
                            </div>
                            <div class="px-5 py-4 space-y-4 bg-slate-50/50 dark:bg-slate-800/20">
                                {@render settingRow('Include Unused Tagged Images', 'Also prune tagged images that are not referenced by any container.', settings.dockerPruneIncludeUnusedTaggedImages ?? false, () => settings.dockerPruneIncludeUnusedTaggedImages = !settings.dockerPruneIncludeUnusedTaggedImages, 'dockerPruneIncludeUnusedTaggedImages')}
                                <!-- Schedule editor inline -->
                                <div class="space-y-1.5">
                                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Cadence</label>
                                    <select
                                        value={draftForTask(task).cadence}
                                        onchange={(e) => patchScheduleDraft(task.id, { cadence: (e.target as HTMLSelectElement).value as any })}
                                        class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors"
                                    >
                                        <option value="daily">Daily</option>
                                        <option value="weekly">Weekly</option>
                                        <option value="monthly">Monthly</option>
                                    </select>
                                </div>
                                <div class="space-y-1.5">
                                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Time (24h)</label>
                                    <input
                                        type="time"
                                        value={draftForTask(task).time}
                                        oninput={(e) => patchScheduleDraft(task.id, { time: (e.target as HTMLInputElement).value })}
                                        class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors"
                                    />
                                </div>
                                {#if draftForTask(task).cadence === 'weekly'}
                                    <div class="flex flex-wrap gap-1.5">
                                        {#each weekdayOptions as day}
                                            <button onclick={() => toggleWeeklyDay(task.id, day.value)} class="px-2.5 py-1 rounded-lg text-[11px] font-bold transition-colors {draftForTask(task).weeklyDays.includes(day.value) ? 'bg-brand-500 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-500'}">{day.label}</button>
                                        {/each}
                                    </div>
                                {/if}
                            </div>
                            <div class="px-5 py-3 border-t border-slate-100 dark:border-slate-800/80 flex items-center gap-3">
                                <button onclick={() => saveTaskSchedule(task.id)} disabled={!scheduleDirty(task) || savingScheduleId === task.id} class="px-4 py-1.5 rounded-xl bg-brand-500 hover:bg-brand-600 disabled:opacity-40 text-white text-[11px] font-black uppercase tracking-widest transition-colors">{savingScheduleId === task.id ? 'Saving…' : 'Save Schedule'}</button>
                                <button onclick={() => runTask(task.id)} class="px-4 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-50 text-slate-600 dark:text-slate-300 text-[11px] font-black uppercase tracking-widest transition-colors">Run Now</button>
                            </div>
                        </div>
                    {/each}

                    <!-- History retention task -->
                    {#each schedules.filter(s => s.id === 'history_retention_prune') as task}
                        {@render taskCard(task)}
                    {/each}

                    <!-- Danger zone -->
                    <div class="rounded-2xl border border-rose-200 dark:border-rose-900/50 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-rose-100 dark:border-rose-900/30">
                            <h3 class="text-sm font-bold text-rose-700 dark:text-rose-400">Danger Zone</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">Permanently delete all historical data. This cannot be undone.</p>
                        </div>
                        <div class="px-5 py-4">
                            <button
                                onclick={clearHistory}
                                class="px-5 py-2.5 rounded-xl bg-rose-500 hover:bg-rose-600 text-white text-[11px] font-black uppercase tracking-widest transition-colors"
                            >
                                Clear All Historical Data
                            </button>
                        </div>
                    </div>

                <!-- ── SECURITY DOMAIN ── -->
                {:else if activeAutomationTab === 'security'}
                    <!-- Trivy -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Trivy Configuration</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">Vulnerability scanning scope for Trivy sweeps.</p>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Sweep Mode</label>
                                <select
                                    bind:value={settings.trivySweepMode}
                                    disabled={isLocked('trivySweepMode')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                >
                                    <option value="running-only">Running containers only</option>
                                    <option value="all">All containers</option>
                                    <option value="images-only">Images only</option>
                                </select>
                                {#if isLocked('trivySweepMode')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                        </div>
                    </div>

                    {#each schedules.filter(s => s.id === 'security_sweep_trivy') as task}
                        {@render taskCard(task)}
                    {/each}

                    <!-- ClamAV -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">ClamAV Configuration</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">Malware scan scope and snapshot limits.</p>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Max Snapshot Size (bytes)</label>
                                <input
                                    bind:value={settings.clamavSnapshotMaxBytes}
                                    type="number" min="1"
                                    disabled={isLocked('clamavSnapshotMaxBytes')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50 font-mono"
                                />
                                <p class="text-[10px] text-slate-400">Currently: {formatBytesCompact(settings.clamavSnapshotMaxBytes)}</p>
                                {#if isLocked('clamavSnapshotMaxBytes')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Ignored Mounts</label>
                                <textarea
                                    bind:value={settings.malwareIgnoredMounts}
                                    rows="3"
                                    placeholder="/proc, /sys, /dev"
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors resize-none font-mono"
                                ></textarea>
                                <p class="text-[10px] text-slate-400">Mount paths to exclude from ClamAV scanning.</p>
                            </div>
                        </div>
                    </div>

                    {#each schedules.filter(s => s.id === 'malware_sweep_clamav' || s.id === 'clamav_signature_update') as task}
                        {@render taskCard(task)}
                    {/each}

                    <!-- ClamAV Signature Status -->
                    {#if clamavStatus}
                        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                            <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80 flex items-center justify-between gap-3">
                                <div>
                                    <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">ClamAV Signature Status</h3>
                                    <p class="text-[11px] text-slate-500 mt-0.5">Current signature database information.</p>
                                </div>
                                <button
                                    onclick={updateClamAVSignaturesNow}
                                    disabled={clamavUpdating}
                                    class="px-4 py-1.5 rounded-xl bg-brand-500 hover:bg-brand-600 disabled:opacity-50 text-white text-[11px] font-black uppercase tracking-widest transition-colors"
                                >
                                    {clamavUpdating ? 'Updating…' : 'Update Now'}
                                </button>
                            </div>
                            <div class="px-5 py-4 grid grid-cols-2 gap-3 text-[11px]">
                                <div>
                                    <p class="text-slate-500">Engine Version</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5 font-mono">{clamavStatus.engineVersion || '—'}</p>
                                </div>
                                <div>
                                    <p class="text-slate-500">DB Version</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5 font-mono">{clamavStatus.databaseVersion || '—'}</p>
                                </div>
                                <div>
                                    <p class="text-slate-500">DB Published</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5">{formatTime(clamavStatus.databasePublished)}</p>
                                </div>
                                <div>
                                    <p class="text-slate-500">Last Local Update</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5">{formatTime(clamavStatus.lastLocalUpdate)}</p>
                                </div>
                            </div>
                        </div>
                    {:else if clamavStatusLoading}
                        <div class="h-20 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 flex items-center justify-center text-[11px] text-slate-400">Loading ClamAV status…</div>
                    {/if}

                <!-- ── REMEDIATION DOMAIN ── -->
                {:else if activeAutomationTab === 'remediation'}
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Self-Healing Policy</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">Controls for automatic container restart on health failure events.</p>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="grid grid-cols-2 gap-4">
                                <div class="space-y-1.5">
                                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Restart Cooldown (sec)</label>
                                    <input
                                        bind:value={settings.unhealthyRestartCooldownSecDefault}
                                        type="number" min="0"
                                        disabled={isLocked('unhealthyRestartCooldownSecDefault')}
                                        class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                    />
                                    {#if isLocked('unhealthyRestartCooldownSecDefault')}
                                        <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                    {/if}
                                </div>
                                <div class="space-y-1.5">
                                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Max Restarts per Window</label>
                                    <input
                                        bind:value={settings.maxRestartsPerWindow}
                                        type="number" min="1" max="20"
                                        disabled={isLocked('maxRestartsPerWindow')}
                                        class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                    />
                                    {#if isLocked('maxRestartsPerWindow')}
                                        <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                    {/if}
                                </div>
                            </div>
                            <p class="text-[11px] text-slate-400">Toggle self-healing on/off using the domain header switch above. Changes are applied on Save Settings.</p>
                        </div>
                    </div>
                {/if}

            <!-- ================================================
                 AI TAB
                 ================================================ -->
            {:else if activeTab === 'ai'}
                <!-- AI sub-tabs -->
                <div class="flex gap-2">
                    {#each ([{id:'settings',label:'Settings'},{id:'costs',label:'Usage & Costs'}] as const) as sub}
                        <button
                            onclick={() => activeAITab = sub.id}
                            class="px-4 py-2 rounded-full text-[11px] font-black uppercase tracking-widest transition-colors
                                {activeAITab === sub.id ? 'bg-brand-500 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-700'}"
                        >{sub.label}</button>
                    {/each}
                </div>

                {#if activeAITab === 'settings'}
                    <!-- AI Runtime -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">AI Runtime</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">Master switch and routing policy for all AI-assisted features.</p>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            {@render settingRow('AI Enabled', 'Enable AI-assisted risk analysis, log interpretation, and audit features.', settings.aiEnabled ?? true, () => settings.aiEnabled = !settings.aiEnabled, 'aiEnabled')}

                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Active Provider</label>
                                <select
                                    bind:value={settings.aiProvider}
                                    disabled={isLocked('aiProvider')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                >
                                    <option value="">Auto (first configured)</option>
                                    <option value="openai">OpenAI</option>
                                    <option value="anthropic">Anthropic</option>
                                    <option value="gemini">Google Gemini</option>
                                </select>
                                {#if isLocked('aiProvider')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>

                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Block Risk Threshold (%)</label>
                                <input
                                    bind:value={settings.aiBlockRiskThreshold}
                                    type="number" min="0" max="100"
                                    disabled={isLocked('aiBlockRiskThreshold')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                />
                                <p class="text-[10px] text-slate-400">Upgrades with AI risk score above this value are blocked. 0 = never block, 100 = always allow.</p>
                                {#if isLocked('aiBlockRiskThreshold')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>

                            <div class="pt-2">
                                <button
                                    onclick={() => onNavigate('ai-history')}
                                    class="flex items-center gap-1.5 text-[11px] font-bold text-brand-600 dark:text-brand-400 hover:underline"
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" /></svg>
                                    View AI Conversation History
                                </button>
                            </div>
                        </div>
                    </div>

                    <!-- OpenAI -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80 flex items-center justify-between gap-3">
                            <div>
                                <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">OpenAI</h3>
                                <p class="text-[11px] text-slate-500 mt-0.5">Configure GPT model access.</p>
                            </div>
                            <button
                                onclick={() => testProvider('openai', settings.openaiModel || '')}
                                disabled={testingProvider === 'openai' || !settings.openaiKey}
                                class="px-3 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-50 text-slate-600 dark:text-slate-300 text-[10px] font-black uppercase tracking-widest transition-colors disabled:opacity-50"
                            >
                                {testingProvider === 'openai' ? 'Testing…' : 'Test'}
                            </button>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">API Key</label>
                                <input
                                    bind:value={settings.openaiKey}
                                    type="password" autocomplete="off"
                                    disabled={isLocked('openaiKey')}
                                    placeholder="sk-…"
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50 font-mono"
                                />
                                {#if isLocked('openaiKey')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Model</label>
                                <select
                                    bind:value={settings.openaiModel}
                                    disabled={isLocked('openaiModel')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                >
                                    {#each providerModels('openai') as opt}
                                        <option value={opt.value}>{opt.label}</option>
                                    {/each}
                                </select>
                                {#if isLocked('openaiModel')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                        </div>
                    </div>

                    <!-- Anthropic -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80 flex items-center justify-between gap-3">
                            <div>
                                <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Anthropic</h3>
                                <p class="text-[11px] text-slate-500 mt-0.5">Configure Claude model access.</p>
                            </div>
                            <button
                                onclick={() => testProvider('anthropic', settings.anthropicModel || '')}
                                disabled={testingProvider === 'anthropic' || !settings.anthropicKey}
                                class="px-3 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-50 text-slate-600 dark:text-slate-300 text-[10px] font-black uppercase tracking-widest transition-colors disabled:opacity-50"
                            >
                                {testingProvider === 'anthropic' ? 'Testing…' : 'Test'}
                            </button>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">API Key</label>
                                <input
                                    bind:value={settings.anthropicKey}
                                    type="password" autocomplete="off"
                                    disabled={isLocked('anthropicKey')}
                                    placeholder="sk-ant-…"
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50 font-mono"
                                />
                                {#if isLocked('anthropicKey')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Model</label>
                                <select
                                    bind:value={settings.anthropicModel}
                                    disabled={isLocked('anthropicModel')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                >
                                    {#each providerModels('anthropic') as opt}
                                        <option value={opt.value}>{opt.label}</option>
                                    {/each}
                                </select>
                                {#if isLocked('anthropicModel')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                        </div>
                    </div>

                    <!-- Gemini -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80 flex items-center justify-between gap-3">
                            <div>
                                <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Google Gemini</h3>
                                <p class="text-[11px] text-slate-500 mt-0.5">Configure Gemini model access.</p>
                            </div>
                            <button
                                onclick={() => testProvider('gemini', settings.geminiModel || '')}
                                disabled={testingProvider === 'gemini' || !settings.geminiKey}
                                class="px-3 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-50 text-slate-600 dark:text-slate-300 text-[10px] font-black uppercase tracking-widest transition-colors disabled:opacity-50"
                            >
                                {testingProvider === 'gemini' ? 'Testing…' : 'Test'}
                            </button>
                        </div>
                        <div class="px-5 py-4 space-y-4">
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">API Key</label>
                                <input
                                    bind:value={settings.geminiKey}
                                    type="password" autocomplete="off"
                                    disabled={isLocked('geminiKey')}
                                    placeholder="AIza…"
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50 font-mono"
                                />
                                {#if isLocked('geminiKey')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                            <div class="space-y-1.5">
                                <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Model</label>
                                <select
                                    bind:value={settings.geminiModel}
                                    disabled={isLocked('geminiModel')}
                                    class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                                >
                                    {#each providerModels('gemini') as opt}
                                        <option value={opt.value}>{opt.label}</option>
                                    {/each}
                                </select>
                                {#if isLocked('geminiModel')}
                                    <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                                {/if}
                            </div>
                        </div>
                    </div>

                    <!-- Pricing JSON -->
                    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                        <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Pricing Configuration</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">JSON map of model pricing used to estimate AI usage costs.</p>
                        </div>
                        <div class="px-5 py-4">
                            <textarea
                                bind:value={settings.aiPricingJson}
                                rows="6"
                                placeholder={`{"openai/gpt-4o": {"input": 0.005, "output": 0.015}}`}
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors resize-y font-mono"
                            ></textarea>
                        </div>
                    </div>

                {:else}
                    <!-- Costs sub-tab -->
                    <!-- Span selector -->
                    <div class="flex flex-wrap gap-2">
                        {#each (['24h','7d','30d','90d'] as AIUsageSpan[]) as span}
                            <button
                                onclick={() => { aiUsageSpan = span; loadAIUsage(); }}
                                class="px-4 py-2 rounded-full text-[11px] font-black uppercase tracking-widest transition-colors
                                    {aiUsageSpan === span ? 'bg-brand-500 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-700'}"
                            >{span}</button>
                        {/each}
                        <button
                            onclick={loadAIUsage}
                            disabled={aiUsageLoading}
                            class="px-4 py-2 rounded-full text-[11px] font-black uppercase tracking-widest bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 hover:bg-slate-200 disabled:opacity-50 transition-colors"
                        >
                            {aiUsageLoading ? 'Loading…' : 'Refresh'}
                        </button>
                    </div>

                    {#if aiUsageError}
                        <div class="px-4 py-3 rounded-xl border border-rose-200 dark:border-rose-900/50 bg-rose-50 dark:bg-rose-900/10 text-rose-700 dark:text-rose-300 text-sm">
                            {aiUsageError}
                        </div>
                    {:else if aiUsage}
                        <!-- Summary stats -->
                        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
                            {#each [
                                { label: 'Calls', value: formatInteger(aiUsage.calls) },
                                { label: 'Input Tokens', value: formatInteger(aiUsage.inputTokens) },
                                { label: 'Output Tokens', value: formatInteger(aiUsage.outputTokens) },
                                { label: 'Est. Cost', value: aiUsage.pricingConfigured ? formatUSDCompact(aiUsage.estimatedCostUsd) : 'N/A' }
                            ] as stat}
                                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 px-4 py-3 shadow-sm">
                                    <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">{stat.label}</p>
                                    <p class="text-lg font-black text-slate-800 dark:text-slate-100 mt-1">{stat.value}</p>
                                </div>
                            {/each}
                        </div>

                        {#if !aiUsage.pricingConfigured}
                            <div class="px-4 py-3 rounded-xl border border-amber-200 dark:border-amber-900/50 bg-amber-50 dark:bg-amber-900/10 text-amber-700 dark:text-amber-300 text-[11px]">
                                Cost estimates require pricing configuration. Add model pricing JSON in the Settings tab.
                                {#if aiUsage.pricingError}<span class="block mt-1 opacity-70">{aiUsage.pricingError}</span>{/if}
                            </div>
                        {/if}

                        <!-- Daily spend bar chart -->
                        {#if aiSpendRows.length > 0 && aiUsage.pricingConfigured}
                            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                                <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                                    <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Daily Spend</h3>
                                </div>
                                <div class="px-5 py-4">
                                    <svg viewBox="0 0 100 48" class="w-full h-28" preserveAspectRatio="none">
                                        {#each dailyBars(aiSpendRows) as bar}
                                            <rect
                                                x={bar.x} y={bar.y}
                                                width={bar.width} height={bar.height}
                                                rx="1.5"
                                                class="fill-brand-400 dark:fill-brand-500 opacity-80"
                                            />
                                        {/each}
                                    </svg>
                                    <div class="flex justify-between text-[9px] text-slate-400 mt-1">
                                        <span>{formatDayLabel(aiSpendRows[0]?.day)}</span>
                                        <span>{formatDayLabel(aiSpendRows[aiSpendRows.length - 1]?.day)}</span>
                                    </div>
                                </div>
                            </div>

                            <!-- Cumulative line chart -->
                            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                                <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                                    <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Cumulative Spend</h3>
                                </div>
                                <div class="px-5 py-4">
                                    <svg viewBox="0 0 100 48" class="w-full h-24" preserveAspectRatio="none">
                                        {#if cumulativeLinePoints(aiSpendRows)}
                                            <polyline
                                                points={cumulativeLinePoints(aiSpendRows)}
                                                fill="none"
                                                stroke="currentColor"
                                                stroke-width="1.5"
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                                class="text-brand-500"
                                            />
                                        {/if}
                                    </svg>
                                    <p class="text-right text-[10px] text-slate-500 mt-1">Total: {formatUSDCompact(aiUsage.estimatedCostUsd)}</p>
                                </div>
                            </div>
                        {/if}

                        <!-- Breakdown table -->
                        {#if aiUsage.breakdown?.length > 0}
                            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                                <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                                    <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Usage Breakdown</h3>
                                </div>
                                <div class="overflow-x-auto">
                                    <table class="w-full text-[11px]">
                                        <thead>
                                            <tr class="border-b border-slate-100 dark:border-slate-800">
                                                <th class="text-left px-4 py-2.5 text-[10px] font-black uppercase tracking-widest text-slate-500">Feature</th>
                                                <th class="text-left px-4 py-2.5 text-[10px] font-black uppercase tracking-widest text-slate-500">Provider / Model</th>
                                                <th class="text-right px-4 py-2.5 text-[10px] font-black uppercase tracking-widest text-slate-500">Calls</th>
                                                <th class="text-right px-4 py-2.5 text-[10px] font-black uppercase tracking-widest text-slate-500">Tokens</th>
                                                {#if aiUsage.pricingConfigured}
                                                    <th class="text-right px-4 py-2.5 text-[10px] font-black uppercase tracking-widest text-slate-500">Est. Cost</th>
                                                {/if}
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {#each aiUsage.breakdown as row}
                                                <tr class="border-b border-slate-50 dark:border-slate-800/50 last:border-0 hover:bg-slate-50 dark:hover:bg-slate-800/30">
                                                    <td class="px-4 py-2.5 font-medium text-slate-700 dark:text-slate-200">{aiFeatureLabel(row.feature)}</td>
                                                    <td class="px-4 py-2.5 text-slate-500">
                                                        <span class="font-medium">{row.provider}</span>
                                                        <span class="text-slate-400"> / {row.model}</span>
                                                    </td>
                                                    <td class="px-4 py-2.5 text-right text-slate-600 dark:text-slate-300">{formatInteger(row.calls)}</td>
                                                    <td class="px-4 py-2.5 text-right text-slate-600 dark:text-slate-300">{formatInteger(row.totalTokens)}</td>
                                                    {#if aiUsage.pricingConfigured}
                                                        <td class="px-4 py-2.5 text-right font-mono text-slate-700 dark:text-slate-200">{formatUSD(row.estimatedCostUsd)}</td>
                                                    {/if}
                                                </tr>
                                            {/each}
                                        </tbody>
                                    </table>
                                </div>
                            </div>
                        {/if}
                    {:else if !aiUsageLoading}
                        <div class="h-32 rounded-2xl border border-dashed border-slate-200 dark:border-slate-700 flex items-center justify-center text-[11px] text-slate-400">No AI usage data for this period.</div>
                    {/if}
                {/if}

            <!-- ================================================
                 INTEGRATIONS TAB
                 ================================================ -->
            {:else if activeTab === 'integrations'}
                <!-- Discord -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                    <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Discord</h3>
                        <p class="text-[11px] text-slate-500 mt-0.5">Send upgrade and alert notifications to a Discord channel via webhook.</p>
                    </div>
                    <div class="px-5 py-4 space-y-4">
                        {@render settingRow('Discord Notifications', 'Enable Discord webhook notifications for automation events.', settings.discordEnabled ?? true, () => settings.discordEnabled = !settings.discordEnabled, 'discordEnabled')}
                        <div class="space-y-1.5">
                            <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Webhook URL</label>
                            <input
                                bind:value={settings.discordWebhookUrl}
                                type="url"
                                disabled={isLocked('discordWebhookUrl')}
                                placeholder="https://discord.com/api/webhooks/…"
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                            />
                            {#if isLocked('discordWebhookUrl')}
                                <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                            {/if}
                        </div>
                    </div>
                </div>

                <!-- Portainer -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                    <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Portainer</h3>
                        <p class="text-[11px] text-slate-500 mt-0.5">Provide Portainer context for stack-aware operations and environment discovery.</p>
                    </div>
                    <div class="px-5 py-4 space-y-4">
                        {@render settingRow('Portainer Integration', 'Enable Portainer API integration.', settings.portainerEnabled ?? true, () => settings.portainerEnabled = !settings.portainerEnabled, 'portainerEnabled')}
                        <div class="space-y-1.5">
                            <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Portainer URL</label>
                            <input
                                bind:value={settings.portainerUrl}
                                type="url"
                                disabled={isLocked('portainerUrl')}
                                placeholder="https://portainer.example.com"
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                            />
                            {#if isLocked('portainerUrl')}
                                <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                            {/if}
                        </div>
                        <div class="space-y-1.5">
                            <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">API Key</label>
                            <input
                                bind:value={settings.portainerApiKey}
                                type="password" autocomplete="off"
                                disabled={isLocked('portainerApiKey')}
                                placeholder="ptr_…"
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50 font-mono"
                            />
                            {#if isLocked('portainerApiKey')}
                                <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                            {/if}
                        </div>
                    </div>
                </div>

            <!-- ================================================
                 SYSTEM TAB
                 ================================================ -->
            {:else if activeTab === 'system'}
                <!-- Metrics Collection -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                    <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Metrics Collection</h3>
                        <p class="text-[11px] text-slate-500 mt-0.5">Background task that samples container and host metrics on a fixed schedule.</p>
                    </div>
                    <div class="px-5 py-4">
                        <div class="flex items-center justify-between gap-4">
                            <div>
                                <p class="text-sm font-medium text-slate-800 dark:text-slate-200">Metrics Collector</p>
                                <p class="text-[11px] text-slate-400 mt-0.5">
                                    {scheduleById('metrics_collector') ? `Schedule: ${taskCronLabel(scheduleById('metrics_collector')!)} · Last run: ${formatTime(scheduleById('metrics_collector')!.lastRun)}` : 'Schedule not available'}
                                </p>
                            </div>
                            {#if scheduleById('metrics_collector')}
                                {@render toggleSwitch(scheduleById('metrics_collector')!.enabled, () => toggleMetricsCollector(), false)}
                            {/if}
                        </div>

                        {@render settingRow('Normalized Metrics', 'Store metrics as percentage-normalized values (0–100) rather than raw counts.', settings.metricsNormalized ?? true, () => settings.metricsNormalized = !settings.metricsNormalized, 'metricsNormalized')}
                    </div>
                </div>

                <!-- ClamAV Signatures -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                    <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80 flex items-center justify-between gap-3">
                        <div>
                            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">ClamAV Signatures</h3>
                            <p class="text-[11px] text-slate-500 mt-0.5">Malware signature database status and update controls.</p>
                        </div>
                        <div class="flex items-center gap-2">
                            <button
                                onclick={loadClamAVStatus}
                                disabled={clamavStatusLoading}
                                class="px-3 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-50 text-slate-600 dark:text-slate-300 text-[10px] font-black uppercase tracking-widest disabled:opacity-50 transition-colors"
                            >{clamavStatusLoading ? 'Loading…' : 'Refresh'}</button>
                            <button
                                onclick={updateClamAVSignaturesNow}
                                disabled={clamavUpdating}
                                class="px-3 py-1.5 rounded-xl bg-brand-500 hover:bg-brand-600 disabled:opacity-50 text-white text-[10px] font-black uppercase tracking-widest transition-colors"
                            >{clamavUpdating ? 'Updating…' : 'Update Now'}</button>
                        </div>
                    </div>
                    <div class="px-5 py-4">
                        {#if clamavStatus}
                            <div class="grid grid-cols-2 gap-3 text-[11px] mb-4">
                                <div class="px-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                    <p class="text-slate-500">Engine Version</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5 font-mono">{clamavStatus.engineVersion || '—'}</p>
                                </div>
                                <div class="px-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                    <p class="text-slate-500">DB Version</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5 font-mono">{clamavStatus.databaseVersion || '—'}</p>
                                </div>
                                <div class="px-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                    <p class="text-slate-500">Published</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5">{formatTime(clamavStatus.databasePublished)}</p>
                                </div>
                                <div class="px-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-700/50">
                                    <p class="text-slate-500">Last Updated</p>
                                    <p class="font-bold text-slate-700 dark:text-slate-200 mt-0.5">{formatTime(clamavStatus.lastLocalUpdate)}</p>
                                </div>
                            </div>
                        {:else if clamavStatusLoading}
                            <div class="h-16 flex items-center justify-center text-[11px] text-slate-400">Loading signature status…</div>
                        {:else}
                            <p class="text-[11px] text-slate-400">Signature status unavailable. ClamAV may not be installed.</p>
                        {/if}

                        <!-- Auto-update schedule toggle -->
                        {#each schedules.filter(s => s.id === 'clamav_signature_update') as task}
                            <div class="flex items-center justify-between gap-4 pt-3 border-t border-slate-100 dark:border-slate-800/50 mt-3">
                                <div>
                                    <p class="text-sm font-medium text-slate-800 dark:text-slate-200">Auto-Update Signatures</p>
                                    <p class="text-[11px] text-slate-400 mt-0.5">Schedule: {taskCronLabel(task)} · Last: {formatTime(task.lastRun)}</p>
                                </div>
                                {@render toggleSwitch(task.enabled, () => toggleTask(task.id, task.enabled), false)}
                            </div>
                        {/each}
                    </div>
                </div>

                <!-- Instance -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                    <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Instance Identity</h3>
                        <p class="text-[11px] text-slate-500 mt-0.5">Public URL and validation probe pattern for this HarborWatch instance.</p>
                    </div>
                    <div class="px-5 py-4 space-y-4">
                        <div class="space-y-1.5">
                            <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Instance URL</label>
                            <input
                                bind:value={settings.instanceUrl}
                                type="url"
                                disabled={isLocked('instanceUrl')}
                                placeholder="https://harborwatch.example.com"
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50"
                            />
                            {#if isLocked('instanceUrl')}
                                <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                            {/if}
                        </div>
                        <div class="space-y-1.5">
                            <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Validate URL Pattern</label>
                            <input
                                bind:value={settings.validateUrlPattern}
                                type="text"
                                disabled={isLocked('validateUrlPattern')}
                                placeholder="https://HOSTNAME/health"
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50 font-mono"
                            />
                            {#if isLocked('validateUrlPattern')}
                                <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                            {/if}
                        </div>
                    </div>
                </div>

            <!-- ================================================
                 BACKUPS TAB
                 ================================================ -->
            {:else if activeTab === 'backups'}
                <!-- Storage Paths -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                    <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Storage Paths</h3>
                        <p class="text-[11px] text-slate-500 mt-0.5">Host filesystem locations for compose snapshots and GitOps working directory.</p>
                    </div>
                    <div class="px-5 py-4 space-y-4">
                        <div class="space-y-1.5">
                            <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">Compose Snapshot Root</label>
                            <input
                                bind:value={settings.composeSnapshotRootPath}
                                type="text"
                                disabled={isLocked('composeSnapshotRootPath')}
                                placeholder="/data/snapshots"
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50 font-mono"
                            />
                            {#if isLocked('composeSnapshotRootPath')}
                                <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                            {/if}
                        </div>
                        <div class="space-y-1.5">
                            <label class="text-[11px] font-black uppercase tracking-widest text-slate-500">GitOps Master Directory</label>
                            <input
                                bind:value={settings.gitOpsMasterDirectory}
                                type="text"
                                disabled={isLocked('gitOpsMasterDirectory')}
                                placeholder="/data/gitops"
                                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 outline-none focus:border-brand-400 transition-colors disabled:opacity-50 font-mono"
                            />
                            {#if isLocked('gitOpsMasterDirectory')}
                                <p class="text-[10px] text-amber-500">Managed by environment variable</p>
                            {/if}
                        </div>
                    </div>
                </div>

                <!-- Compose Snapshot Sweep -->
                {#each schedules.filter(s => s.id === 'compose_snapshot_on_change') as task}
                    {@render taskCard(task)}
                {/each}

            <!-- ================================================
                 APPEARANCE TAB
                 ================================================ -->
            {:else if activeTab === 'appearance'}
                <!-- Motion & Animations -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                    <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Motion & Animations</h3>
                        <p class="text-[11px] text-slate-500 mt-0.5">Control transition and animation behaviour across all views.</p>
                    </div>
                    <div class="px-5 py-4">
                        {@render settingRow('UI Animations', 'Enable CSS transitions and motion effects throughout the interface.', settings.uiAnimationsEnabled ?? true, () => settings.uiAnimationsEnabled = !settings.uiAnimationsEnabled, 'uiAnimationsEnabled')}
                    </div>
                </div>

                <!-- Metrics Display -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                    <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Metrics Display</h3>
                        <p class="text-[11px] text-slate-500 mt-0.5">How metrics values are presented in charts and tables.</p>
                    </div>
                    <div class="px-5 py-4">
                        {@render settingRow('Normalized Metrics', 'Display metrics as 0–100% normalized values rather than raw counts.', settings.metricsNormalized ?? true, () => settings.metricsNormalized = !settings.metricsNormalized, 'metricsNormalized')}
                    </div>
                </div>

                <!-- Theme -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden shadow-sm">
                    <div class="px-5 py-4 border-b border-slate-100 dark:border-slate-800/80">
                        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">Theme</h3>
                        <p class="text-[11px] text-slate-500 mt-0.5">Interface colour mode and brand palette selection.</p>
                    </div>
                    <div class="px-5 py-4">
                        <ThemeSwitcher />
                    </div>
                </div>
            {/if}

        </div><!-- /main content -->
    </div><!-- /body flex -->
</div><!-- /settings-page -->

<style>
    .settings-page {
        min-height: 100vh;
    }

    .settings-mobile-nav {
        scrollbar-width: none;
        -ms-overflow-style: none;
    }

    .settings-mobile-nav::-webkit-scrollbar {
        display: none;
    }

    :global(.no-motion) .settings-page *,
    :global(.no-motion) .settings-page *::before,
    :global(.no-motion) .settings-page *::after {
        transition: none !important;
        animation: none !important;
    }
</style>
