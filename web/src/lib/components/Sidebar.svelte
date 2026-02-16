<script lang="ts">
    import { themeStore } from '../stores/theme.svelte';
    import { layoutStore } from '../stores/layout.svelte';

    let { currentRoute, onNavigate } = $props<{
        currentRoute: string;
        onNavigate: (path: string) => void;
    }>();

    let collapsed = $derived(layoutStore.sidebarCollapsed);

    const navItems = [
        { path: 'dashboard', label: 'Dashboard', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6' },
        { path: 'containers', label: 'Containers', icon: 'M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10' },
        { path: 'images', label: 'Images', icon: 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4' },
        { path: 'security', label: 'Security', icon: 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z' },
        { path: 'automation', label: 'Automation', icon: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z' },
        { path: 'doctor', label: 'Compose Doctor', icon: 'M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.67.335a2 2 0 01-1.797 0l-.67-.335a6 6 0 00-3.86-.517l-2.387.477a2 2 0 00-1.022.547l-1.162 1.162a2 2 0 00.597 3.301l1.557.519a8.001 8.001 0 0011.965 0l1.557-.519a2 2 0 00.597-3.301l-1.162-1.162z' },
        { path: 'intelligence', label: 'Intelligence', icon: 'M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z' },
        { path: 'updates', label: 'Updates', icon: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15' },
        { path: 'audit', label: 'Audit Log', icon: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z' },
        { path: 'settings', label: 'Settings', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z' },
    ];

    function handleNavClick(path: string) {
        onNavigate(path);
        layoutStore.closeMobileSidebar();
    }
</script>

<aside class="fixed left-0 top-0 h-full bg-white dark:bg-slate-900 shadow-xl border-r border-slate-200 dark:border-slate-800 transition-all duration-300 flex flex-col z-50 {collapsed ? 'w-20' : 'w-64'} {layoutStore.mobileSidebarOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}">
    <!-- Logo -->
    <div class="flex items-center gap-3 p-4 h-16 border-b border-slate-100 dark:border-slate-800">
        <div class="w-10 h-10 rounded-xl bg-brand-600 flex items-center justify-center flex-shrink-0 shadow-lg shadow-brand-500/20 text-white">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
        </div>
        {#if !collapsed}
            <div class="flex flex-col overflow-hidden">
                <h1 class="text-sm font-bold text-slate-900 dark:text-white leading-tight truncate uppercase tracking-wider">HarborWatch</h1>
                <span class="text-[10px] font-bold text-brand-600 dark:text-brand-400 uppercase tracking-tighter">Container Sentry</span>
            </div>
        {/if}
    </div>

    <!-- Nav Items -->
    <nav class="flex-1 overflow-y-auto p-3 space-y-1">
        {#each navItems as item}
            <button
                class="w-full flex items-center gap-3 p-3 rounded-xl transition-all duration-200 group {currentRoute === item.path ? 'bg-brand-50 dark:bg-brand-900/20 text-brand-600 dark:text-brand-400' : 'text-slate-500 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-slate-200'}"
                onclick={() => handleNavClick(item.path)}
                title={collapsed ? item.label : ''}
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 flex-shrink-0 transition-transform group-hover:scale-110" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d={item.icon} />
                </svg>
                {#if !collapsed}
                    <span class="text-sm font-semibold">{item.label}</span>
                {/if}
            </button>
        {/each}
    </nav>

    <!-- Bottom Actions -->
    <div class="p-3 border-t border-slate-100 dark:border-slate-800 space-y-1">
        <button
            class="w-full flex items-center gap-3 p-3 rounded-xl text-slate-500 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 transition-all duration-200"
            onclick={() => themeStore.toggle()}
            title={collapsed ? 'Toggle Theme' : ''}
        >
            {#if themeStore.isDark}
                <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                </svg>
            {:else}
                <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                </svg>
            {/if}
            {#if !collapsed}
                <span class="text-sm font-semibold">{themeStore.isDark ? 'Light Mode' : 'Dark Mode'}</span>
            {/if}
        </button>

        <button
            class="w-full flex items-center justify-center p-3 rounded-xl text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-all duration-200"
            onclick={() => layoutStore.toggleSidebar()}
            aria-label={collapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
            title={collapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
        >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 transition-transform duration-300 {collapsed ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
            </svg>
        </button>
    </div>
</aside>
