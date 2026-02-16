class LayoutStore {
    sidebarCollapsed = $state(false);
    mobileSidebarOpen = $state(false);

    constructor() {
        if (typeof localStorage !== 'undefined') {
            const stored = localStorage.getItem('sidebarCollapsed');
            if (stored) {
                this.sidebarCollapsed = stored === 'true';
            }
        }

        $effect.root(() => {
            $effect(() => {
                if (typeof localStorage !== 'undefined') {
                    localStorage.setItem('sidebarCollapsed', String(this.sidebarCollapsed));
                }
            });
        });
    }

    toggleSidebar() {
        this.sidebarCollapsed = !this.sidebarCollapsed;
    }

    toggleMobileSidebar() {
        this.mobileSidebarOpen = !this.mobileSidebarOpen;
    }

    closeMobileSidebar() {
        this.mobileSidebarOpen = false;
    }
}

export const layoutStore = new LayoutStore();
