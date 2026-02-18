<script lang="ts">
    import { onMount } from "svelte";
    import type { HealthResponse } from "../api-types";

    let version = $state("Unknown");
    let serviceStatus = $state("Unknown");

    onMount(async () => {
        try {
            const res = await fetch("/health");
            if (!res.ok) return;
            const payload = await res.json() as HealthResponse;
            version = payload.version || "Unknown";
            serviceStatus = payload.status || "Unknown";
        } catch {
            // Keep fallback values on transient connectivity issues.
        }
    });
</script>

<div class="w-full space-y-6">
    <section class="rounded-3xl border border-slate-200 dark:border-slate-700 bg-white/80 dark:bg-slate-900/45 p-6 md:p-8">
        <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-6">
            <div class="space-y-3 max-w-3xl">
                <p class="text-[10px] font-black uppercase tracking-[0.2em] text-brand-600 dark:text-brand-300">About HarborWatch</p>
                <h2 class="text-3xl md:text-4xl font-black text-slate-900 dark:text-white leading-tight">A modern control plane for container maintenance, upgrade safety, and security automation.</h2>
                <p class="text-sm text-slate-600 dark:text-slate-300">HarborWatch centralizes container inventory, update policy gates, image lifecycle intelligence, malware/vulnerability scanning, and AI-assisted upgrade analysis in a single operational UI.</p>
                <div class="flex flex-wrap gap-2 pt-1">
                    <span class="px-3 py-1 rounded-full text-[10px] font-black uppercase tracking-wider bg-brand-100 text-brand-800 dark:bg-brand-900/40 dark:text-brand-200">Version {version}</span>
                    <span class="px-3 py-1 rounded-full text-[10px] font-black uppercase tracking-wider bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200">Status {serviceStatus}</span>
                </div>
            </div>
            <div class="w-28 h-28 md:w-32 md:h-32 rounded-3xl bg-white/90 dark:bg-slate-800/80 ring-1 ring-brand-200 dark:ring-brand-700 shadow-xl shadow-brand-900/10 flex items-center justify-center">
                <img src="/logo-128.png" alt="HarborWatch icon" class="w-24 h-24 md:w-28 md:h-28" />
            </div>
        </div>
    </section>

    <section class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <article class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white/70 dark:bg-slate-900/30 p-4">
            <h3 class="text-sm font-black uppercase tracking-wider text-slate-700 dark:text-slate-200">Upgrade Orchestration</h3>
            <p class="mt-2 text-xs text-slate-600 dark:text-slate-300">Detects candidate updates, applies policy gates, and supports controlled rollout with validation and rollback context.</p>
        </article>
        <article class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white/70 dark:bg-slate-900/30 p-4">
            <h3 class="text-sm font-black uppercase tracking-wider text-slate-700 dark:text-slate-200">Security Intelligence</h3>
            <p class="mt-2 text-xs text-slate-600 dark:text-slate-300">Runs Trivy and ClamAV workflows, tracks scan state, and surfaces findings in container and image-level views.</p>
        </article>
        <article class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white/70 dark:bg-slate-900/30 p-4">
            <h3 class="text-sm font-black uppercase tracking-wider text-slate-700 dark:text-slate-200">Operational Telemetry</h3>
            <p class="mt-2 text-xs text-slate-600 dark:text-slate-300">Maintains system health history, scheduler outcomes, lifecycle events, and AI usage/cost tracking for auditability.</p>
        </article>
    </section>

    <section class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white/70 dark:bg-slate-900/30 p-5 md:p-6 space-y-3">
        <h3 class="text-sm font-black uppercase tracking-wider text-slate-700 dark:text-slate-200">Platform Details</h3>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3">
                <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Backend</p>
                <p class="text-xs text-slate-700 dark:text-slate-200 mt-1">Go API service with scheduler, persistence, update orchestration, scan orchestration, and integration pipelines.</p>
            </div>
            <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3">
                <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Frontend</p>
                <p class="text-xs text-slate-700 dark:text-slate-200 mt-1">Svelte 5 + Tailwind UI with real-time status surfacing, policy controls, and operator-focused workflows.</p>
            </div>
            <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3">
                <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Data Ownership</p>
                <p class="text-xs text-slate-700 dark:text-slate-200 mt-1">Operational metadata, logs, and analysis history are stored by HarborWatch and survive container updates when persistent volumes are retained.</p>
            </div>
            <div class="rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-900/40 p-3">
                <p class="text-[10px] font-black uppercase tracking-wider text-slate-500">Automation Model</p>
                <p class="text-xs text-slate-700 dark:text-slate-200 mt-1">Domain-based automation (Upgrades, Maintenance, Security) with per-task scheduling, run-now triggers, and centralized exclusions.</p>
            </div>
        </div>
    </section>

    <section class="rounded-2xl border border-slate-200 dark:border-slate-700 bg-white/70 dark:bg-slate-900/30 p-4">
        <p class="text-[10px] uppercase tracking-wider font-black text-slate-500">Icon Attribution</p>
        <p class="mt-1 text-[11px] text-slate-600 dark:text-slate-300">
            <a href="https://www.flaticon.com/free-icons/harbour" title="harbour icons" target="_blank" rel="noreferrer noopener" class="underline decoration-brand-400/60 underline-offset-2 hover:text-brand-700 dark:hover:text-brand-300">Harbour icons created by Freepik - Flaticon</a>
        </p>
    </section>
</div>
