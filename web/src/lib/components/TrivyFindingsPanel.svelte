<script lang="ts">
    import type { TrivyScanDetails, TrivyVulnerability } from "../api-types";

    type TrivyFindingRow = TrivyVulnerability & {
        key: string;
        resultType: string;
        resultTarget: string;
        resultClass: string;
        severityNorm: string;
    };

    let { details, loading = false } = $props<{
        details: TrivyScanDetails | null;
        loading?: boolean;
    }>();

    let search = $state("");
    let severity = $state("all");
    let showRaw = $state(false);

    const severityOrder: Record<string, number> = {
        CRITICAL: 0,
        HIGH: 1,
        MEDIUM: 2,
        LOW: 3,
        UNKNOWN: 4
    };

    const severityClasses: Record<string, string> = {
        CRITICAL: "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300",
        HIGH: "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300",
        MEDIUM: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300",
        LOW: "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300",
        UNKNOWN: "bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-200"
    };

    function normalizeSeverity(value?: string): string {
        const sev = String(value || "UNKNOWN").toUpperCase();
        return severityOrder[sev] === undefined ? "UNKNOWN" : sev;
    }

    function flattenFindings(input: TrivyScanDetails | null): TrivyFindingRow[] {
        if (!input?.results || input.results.length === 0) return [];
        const out: TrivyFindingRow[] = [];
        for (const group of input.results) {
            const vulns = group.vulnerabilities || [];
            for (const vuln of vulns) {
                const sev = normalizeSeverity(vuln.severity);
                out.push({
                    ...vuln,
                    key: `${vuln.id || "UNKNOWN"}:${vuln.pkgName || "pkg"}:${vuln.installedVersion || ""}:${vuln.fixedVersion || ""}:${group.target || "target"}:${group.type || ""}`,
                    resultType: group.type || "",
                    resultTarget: group.target || "",
                    resultClass: group.class || "",
                    severityNorm: sev
                });
            }
        }
        return out;
    }

    function filteredFindings(): TrivyFindingRow[] {
        const all = flattenFindings(details);
        const q = search.trim().toLowerCase();
        const scoped = all.filter((item) => {
            if (severity !== "all" && item.severityNorm !== severity) return false;
            if (!q) return true;
            const haystack = [
                item.id,
                item.pkgName,
                item.title,
                item.description,
                item.installedVersion,
                item.fixedVersion,
                item.resultTarget
            ].join(" ").toLowerCase();
            return haystack.includes(q);
        });
        scoped.sort((a, b) => {
            const left = severityOrder[a.severityNorm] ?? 9;
            const right = severityOrder[b.severityNorm] ?? 9;
            if (left !== right) return left - right;
            return (a.id || "").localeCompare(b.id || "");
        });
        return scoped;
    }

    function severityPill(sev: string): string {
        return severityClasses[sev] || severityClasses.UNKNOWN;
    }

    function findingCount(sev: string): number {
        return flattenFindings(details).filter((v) => v.severityNorm === sev).length;
    }

    function formatWhen(value?: string): string {
        if (!value) return "n/a";
        const d = new Date(value);
        return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
    }
</script>

<div class="space-y-4">
    {#if loading}
        <div class="py-6 text-sm text-slate-500">Loading vulnerability findings...</div>
    {:else if !details}
        <div class="py-6 text-sm text-slate-400 italic">No Trivy findings available yet for this image.</div>
    {:else}
        <div class="grid grid-cols-2 md:grid-cols-5 gap-2 text-xs">
            <div class="p-2 rounded-xl bg-rose-50 dark:bg-rose-900/10 border border-rose-100 dark:border-rose-900/30">
                <div class="text-[10px] font-black uppercase text-rose-500">Critical</div>
                <div class="text-lg font-black text-rose-700 dark:text-rose-300">{details.summary.critical}</div>
            </div>
            <div class="p-2 rounded-xl bg-orange-50 dark:bg-orange-900/10 border border-orange-100 dark:border-orange-900/30">
                <div class="text-[10px] font-black uppercase text-orange-500">High</div>
                <div class="text-lg font-black text-orange-700 dark:text-orange-300">{details.summary.high}</div>
            </div>
            <div class="p-2 rounded-xl bg-amber-50 dark:bg-amber-900/10 border border-amber-100 dark:border-amber-900/30">
                <div class="text-[10px] font-black uppercase text-amber-500">Medium</div>
                <div class="text-lg font-black text-amber-700 dark:text-amber-300">{details.summary.medium}</div>
            </div>
            <div class="p-2 rounded-xl bg-emerald-50 dark:bg-emerald-900/10 border border-emerald-100 dark:border-emerald-900/30">
                <div class="text-[10px] font-black uppercase text-emerald-500">Low</div>
                <div class="text-lg font-black text-emerald-700 dark:text-emerald-300">{details.summary.low}</div>
            </div>
            <div class="p-2 rounded-xl bg-slate-50 dark:bg-slate-900/30 border border-slate-100 dark:border-slate-700">
                <div class="text-[10px] font-black uppercase text-slate-500">Total</div>
                <div class="text-lg font-black text-slate-700 dark:text-slate-200">{details.summary.total}</div>
            </div>
        </div>

        {#if details.parseError}
            <div class="text-xs rounded-xl border border-amber-200 bg-amber-50 text-amber-700 px-3 py-2 dark:bg-amber-900/10 dark:text-amber-300 dark:border-amber-900/30">
                Some fields could not be parsed from raw Trivy output: {details.parseError}
            </div>
        {/if}

        <div class="flex flex-col md:flex-row gap-2">
            <input
                bind:value={search}
                placeholder="Search CVE, package, version, description..."
                class="flex-1 bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs outline-none focus:ring-2 focus:ring-brand-500"
            />
            <select
                bind:value={severity}
                class="bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-xs font-bold outline-none focus:ring-2 focus:ring-brand-500"
            >
                <option value="all">All severities</option>
                <option value="CRITICAL">Critical ({findingCount("CRITICAL")})</option>
                <option value="HIGH">High ({findingCount("HIGH")})</option>
                <option value="MEDIUM">Medium ({findingCount("MEDIUM")})</option>
                <option value="LOW">Low ({findingCount("LOW")})</option>
                <option value="UNKNOWN">Unknown ({findingCount("UNKNOWN")})</option>
            </select>
            <button
                onclick={() => showRaw = !showRaw}
                class="px-3 py-2 rounded-xl text-xs font-black uppercase tracking-wider border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-900"
            >
                {showRaw ? "Hide Raw JSON" : "Show Raw JSON"}
            </button>
        </div>

        {#if showRaw}
            <pre class="max-h-[280px] overflow-auto text-[10px] leading-relaxed bg-slate-950 text-slate-100 rounded-xl p-3">{details.rawJson}</pre>
        {/if}

        <div class="space-y-2 max-h-[560px] overflow-auto pr-1">
            {#each filteredFindings() as finding (finding.key)}
                <div class="rounded-2xl border border-slate-200 dark:border-slate-700 p-3 bg-white dark:bg-slate-900/30">
                    <div class="flex flex-wrap items-center gap-2">
                        <span class="px-2 py-1 rounded-lg text-[10px] font-black uppercase {severityPill(finding.severityNorm)}">{finding.severityNorm}</span>
                        <span class="font-mono text-xs text-slate-700 dark:text-slate-200">{finding.id || "unknown-id"}</span>
                    </div>

                    <div class="mt-2 grid grid-cols-1 md:grid-cols-2 gap-2 text-xs">
                        <div><span class="text-slate-400 uppercase text-[10px] font-black">Package</span> <span class="font-mono">{finding.pkgName || "n/a"}</span></div>
                        <div><span class="text-slate-400 uppercase text-[10px] font-black">Location</span> <span class="font-mono">{finding.resultTarget || "n/a"}</span></div>
                        <div><span class="text-slate-400 uppercase text-[10px] font-black">Installed</span> <span class="font-mono">{finding.installedVersion || "n/a"}</span></div>
                        <div><span class="text-slate-400 uppercase text-[10px] font-black">Fixed</span> <span class="font-mono">{finding.fixedVersion || "not fixed"}</span></div>
                        <div><span class="text-slate-400 uppercase text-[10px] font-black">CVSS</span> <span class="font-mono">{finding.cvssScore ? `${finding.cvssScore.toFixed(1)}${finding.cvssSource ? ` (${finding.cvssSource})` : ""}` : "n/a"}</span></div>
                        <div><span class="text-slate-400 uppercase text-[10px] font-black">Type</span> <span class="font-mono">{finding.resultType || finding.resultClass || "n/a"}</span></div>
                    </div>

                    {#if finding.title}
                        <p class="mt-2 text-xs font-semibold text-slate-700 dark:text-slate-200">{finding.title}</p>
                    {/if}

                    <details class="mt-2 text-xs">
                        <summary class="cursor-pointer text-brand-600 font-semibold">Inspect full finding</summary>
                        <div class="mt-2 space-y-2">
                            {#if finding.description}
                                <p class="text-slate-600 dark:text-slate-300 whitespace-pre-wrap">{finding.description}</p>
                            {/if}
                            <div class="text-slate-500">
                                Published: <span class="font-mono">{formatWhen(finding.publishedDate)}</span>
                                <span class="mx-1">|</span>
                                Updated: <span class="font-mono">{formatWhen(finding.lastModifiedDate)}</span>
                            </div>
                            {#if finding.primaryUrl}
                                <div>
                                    <a href={finding.primaryUrl} target="_blank" rel="noopener noreferrer" class="text-brand-600 hover:underline break-all">{finding.primaryUrl}</a>
                                </div>
                            {/if}
                            {#if finding.references && finding.references.length > 0}
                                <div class="space-y-1">
                                    {#each finding.references as ref}
                                        <a href={ref} target="_blank" rel="noopener noreferrer" class="block text-brand-600 hover:underline break-all">{ref}</a>
                                    {/each}
                                </div>
                            {/if}
                        </div>
                    </details>
                </div>
            {:else}
                <div class="py-8 text-center text-slate-400 text-sm italic">No findings match your current filters.</div>
            {/each}
        </div>
    {/if}
</div>
