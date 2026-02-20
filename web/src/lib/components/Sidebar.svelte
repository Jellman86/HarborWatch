<script lang="ts">
    import { themeStore } from '../stores/theme.svelte';
    import { layoutStore } from '../stores/layout.svelte';

    let { currentRoute, onNavigate, hasActiveJobs = false } = $props<{
        currentRoute: string;
        onNavigate: (path: string) => void;
        hasActiveJobs?: boolean;
    }>();

    let collapsed = $derived(layoutStore.sidebarCollapsed);

    const navItems = [
        { path: 'dashboard', label: 'Dashboard', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6' },
        { path: 'containers', label: 'Fleet', icon: 'M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10' },
        { path: 'stacks', label: 'Stacks', icon: 'M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z' },
        { path: 'images', label: 'Images', icon: 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4' },
        { path: 'diagnostics', label: 'System Health', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z' },
        { path: 'about', label: 'About', icon: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z' },
        { path: 'settings', label: 'Settings', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z' },
    ];

    function handleNavClick(path: string) {
        onNavigate(path);
        layoutStore.closeMobileSidebar();
    }
</script>

<aside class="fixed left-0 top-0 h-full bg-white dark:bg-[#020617] shadow-2xl border-r border-slate-200 dark:border-cyan-500/10 transition-all duration-300 flex flex-col z-50 {collapsed ? 'w-20' : 'w-64'} {layoutStore.mobileSidebarOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0 pointer-events-none md:pointer-events-auto'}">
    <div class="flex-1 flex flex-col min-h-0 pointer-events-auto">
        <!-- Logo -->        <div class="flex items-center justify-between p-6 h-auto border-b border-slate-100 dark:border-cyan-500/5">
            <div class="flex flex-col items-center w-full gap-4 text-center">
                <div class="relative">
                    <img 
                        src="/logo-64.png" 
                        alt="HarborWatch" 
                        class="h-20 w-20 flex-shrink-0 transition-all duration-500 {hasActiveJobs ? 'ring-4 ring-cyan-500/30 animate-pulse rounded-full' : ''}" 
                    />
                    {#if hasActiveJobs}
                        <span class="absolute -top-1 -right-1 flex h-3 w-3">
                            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                            <span class="relative inline-flex rounded-full h-3 w-3 bg-emerald-500"></span>
                        </span>
                    {/if}
                </div>
                {#if !collapsed}
                    <div class="flex flex-col overflow-hidden items-center">
                        <h1 class="text-xl font-black text-slate-900 dark:text-white leading-tight truncate uppercase tracking-[0.2em]">HarborWatch</h1>
                        <span class="text-[9px] font-black text-cyan-600 dark:text-cyan-400 uppercase tracking-widest leading-relaxed mt-1 opacity-80 px-2">Simple Container Maintenance & Management</span>
                    </div>
                {/if}
            </div>
                
                <button 
                    onclick={() => layoutStore.closeMobileSidebar()}
                    class="md:hidden absolute top-4 right-4 p-2 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors"
                    aria-label="Close Navigation"
                >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
        </button>
    </div>

    <!-- Nav Items -->
    <nav class="flex-1 overflow-y-auto p-4 space-y-1.5">
        {#each navItems as item}
            <button
                class="w-full flex items-center gap-3.5 p-3 rounded-xl transition-all duration-300 group nav-item-hover {currentRoute === item.path ? 'nav-item-active' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
                onclick={() => handleNavClick(item.path)}
                title={collapsed ? item.label : ''}
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 flex-shrink-0 transition-all duration-300 group-hover:scale-110 {currentRoute === item.path ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-400 group-hover:text-cyan-500'}" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d={item.icon} />
                </svg>
                {#if !collapsed}
                    <span class="text-[13px] font-black uppercase tracking-wider">{item.label}</span>
                {/if}
            </button>
        {/each}
    </nav>

    <!-- Bottom Actions -->
    <div class="p-4 border-t border-slate-100 dark:border-cyan-500/5 space-y-1.5">
        <button
            class="w-full flex items-center gap-3.5 p-3 rounded-xl text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-cyan-500/5 hover:text-slate-900 dark:hover:text-slate-100 transition-all duration-300 group"
            onclick={() => themeStore.toggle()}
            title={collapsed ? 'Toggle Theme' : ''}
        >
            <div class="h-6 w-6 flex items-center justify-center transition-transform group-hover:rotate-12">
                {#if themeStore.isDark}
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                    </svg>
                {:else}
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                    </svg>
                {/if}
            </div>
            {#if !collapsed}
                <span class="text-[13px] font-black uppercase tracking-wider">{themeStore.isDark ? 'Light Mode' : 'Dark Mode'}</span>
            {/if}
        </button>

        <button
            class="w-full hidden md:flex items-center justify-center p-3 rounded-xl text-slate-400 hover:text-cyan-600 dark:hover:text-cyan-400 transition-all duration-300 hover:bg-slate-50 dark:hover:bg-cyan-500/5"
            onclick={() => layoutStore.toggleSidebar()}
            aria-label={collapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
            title={collapsed ? 'Expand Sidebar' : 'Collapse Sidebar'}
        >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 transition-transform duration-500 {collapsed ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
            </svg>
        </button>
    </div>
    </div>
</aside>
