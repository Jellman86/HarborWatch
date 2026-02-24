<script lang="ts">
    let {
        summaryText,
        pageText,
        canPrev = false,
        canNext = false,
        onPrev = () => {},
        onNext = () => {},
    } = $props<{
        summaryText: string;
        pageText: string;
        canPrev?: boolean;
        canNext?: boolean;
        onPrev?: () => void;
        onNext?: () => void;
    }>();
</script>

<div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900/30 px-4 py-3">
    <p class="text-[10px] font-black uppercase tracking-widest text-slate-500">
        {summaryText}
    </p>
    <div class="flex items-center gap-2">
        <span class="inline-flex min-h-[34px] min-w-[108px] items-center justify-center px-3 py-1.5 bg-white dark:bg-slate-900 rounded-xl text-[10px] font-black text-slate-500 uppercase tracking-widest border border-slate-200 dark:border-slate-700">
            {pageText}
        </span>
        <button
            type="button"
            onclick={onPrev}
            disabled={!canPrev}
            class="inline-flex min-h-[34px] min-w-[68px] items-center justify-center px-3 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 disabled:opacity-50 hover:border-brand-300"
        >
            Prev
        </button>
        <button
            type="button"
            onclick={onNext}
            disabled={!canNext}
            class="inline-flex min-h-[34px] min-w-[68px] items-center justify-center px-3 py-1.5 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-[10px] font-black uppercase tracking-widest text-slate-600 dark:text-slate-300 disabled:opacity-50 hover:border-brand-300"
        >
            Next
        </button>
    </div>
</div>
