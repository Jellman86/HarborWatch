<script lang="ts">
    interface FlowStep {
        label: string;
        state?: "active" | "idle" | "warning";
    }

    let { title, subtitle = "", steps = [], accent = "#0ea5e9" } = $props<{
        title: string;
        subtitle?: string;
        steps: FlowStep[];
        accent?: string;
    }>();

    function nodeClasses(state?: string): string {
        switch (state) {
            case "active":
                return "border-transparent text-white shadow-lg";
            case "warning":
                return "border-amber-200 bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:border-amber-900/40 dark:text-amber-300";
            default:
                return "border-slate-200 bg-white text-slate-500 dark:bg-slate-900/40 dark:border-slate-700 dark:text-slate-300";
        }
    }

    function badgeClasses(state?: string): string {
        switch (state) {
            case "active":
                return "bg-white/20 text-white";
            case "warning":
                return "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300";
            default:
                return "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300";
        }
    }

    function statusLabel(state?: string): string {
        if (state === "active") return "active";
        if (state === "warning") return "partial";
        return "idle";
    }
</script>

<div class="space-y-3">
    <div>
        <h4 class="text-sm font-black text-slate-800 dark:text-slate-100 tracking-tight uppercase">{title}</h4>
        {#if subtitle}
            <p class="text-xs text-slate-500 mt-1">{subtitle}</p>
        {/if}
    </div>

    {#if steps.length > 0}
        <div class="overflow-x-auto pb-2">
            <div class="flex items-center min-w-max">
                <div class="shrink-0 rounded-full border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-900 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-500">
                    Start
                </div>
                {#each steps as step, idx}
                    <div class="mx-2 w-12 flex items-center justify-center" aria-hidden="true">
                        <div class="relative h-[2px] w-10 bg-slate-300 dark:bg-slate-700">
                            <span class="absolute -right-1 -top-[3px] h-0 w-0 border-y-[4px] border-y-transparent border-l-[6px] border-l-slate-400 dark:border-l-slate-500"></span>
                        </div>
                    </div>
                    <div
                        class="min-w-[164px] rounded-xl border px-3 py-3 transition-all {nodeClasses(step.state)}"
                        style={step.state === "active" ? `background:${accent};` : ""}
                    >
                        <p class="text-[9px] font-black uppercase tracking-[0.18em] opacity-80">Step {idx + 1}</p>
                        <p class="mt-1 text-[11px] font-bold uppercase tracking-wide">{step.label}</p>
                        <span class="mt-2 inline-block px-2 py-0.5 rounded-full text-[9px] font-black uppercase tracking-wider {badgeClasses(step.state)}">
                            {statusLabel(step.state)}
                        </span>
                    </div>
                {/each}
                <div class="mx-2 w-12 flex items-center justify-center" aria-hidden="true">
                    <div class="relative h-[2px] w-10 bg-slate-300 dark:bg-slate-700">
                        <span class="absolute -right-1 -top-[3px] h-0 w-0 border-y-[4px] border-y-transparent border-l-[6px] border-l-slate-400 dark:border-l-slate-500"></span>
                    </div>
                </div>
                <div class="shrink-0 rounded-full border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-900 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-slate-500">
                    End
                </div>
            </div>
        </div>
    {:else}
        <div class="h-[120px] rounded-xl border border-dashed border-slate-200 dark:border-slate-700 flex items-center justify-center text-xs italic text-slate-400">
            No flow steps configured.
        </div>
    {/if}
</div>
