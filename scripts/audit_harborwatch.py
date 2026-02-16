import os
import json
import time
from playwright.sync_api import sync_playwright

PLAYWRIGHT_WS = "ws://playwright-service:3000/"
BASE_URL = "http://harborwatch-dev:8000" 
RESULTS_DIR = "/config/workspace/playwright-results/harborwatch-audit"

os.makedirs(RESULTS_DIR, exist_ok=True)

def inject_heartbeat(page):
    """
    Injects a script that monitors the main thread responsiveness.
    If the interval between frames exceeds 250ms, it logs a 'UI_HANG' console message.
    """
    page.add_init_script("""
        (function() {
            let lastTime = performance.now();
            function check() {
                const now = performance.now();
                const delta = now - lastTime;
                if (delta > 250) {
                    console.error('UI_HANG_DETECTED: Main thread blocked for ' + Math.round(delta) + 'ms');
                }
                lastTime = now;
                requestAnimationFrame(check);
            }
            requestAnimationFrame(check);
            
            // Also monitor Long Tasks API
            const observer = new PerformanceObserver((list) => {
                for (const entry of list.getEntries()) {
                    if (entry.duration > 100) {
                        console.warn('LONG_TASK_DETECTED: duration=' + Math.round(entry.duration) + 'ms, name=' + entry.name);
                    }
                }
            });
            observer.observe({entryTypes: ['longtask']});
        })();
    """)

def run_suite(context, device_name, routes):
    console_logs = []
    
    page = context.new_page()
    page.on("console", lambda msg: console_logs.append({
        "device": device_name,
        "type": msg.type,
        "text": msg.text,
        "location": msg.location
    }))
    inject_heartbeat(page)

    print(f"--- Auditing {device_name} ---")
    
    # 1. Route Audit
    for route in routes:
        url = f"{BASE_URL}{route}"
        print(f"  [{device_name}] Auditing {url}...")
        try:
            page.goto(url, wait_until="domcontentloaded", timeout=15000)
            time.sleep(2)
            name = route.replace("/", "") or "dashboard"
            page.screenshot(path=os.path.join(RESULTS_DIR, f"{device_name}_{name}.png"), full_page=True)
        except Exception as e:
            print(f"  [{device_name}] Failed to audit {route}: {e}")

    # 2. Interaction Stress Test
    print(f"  [{device_name}] Starting Interaction Stress Test...")
    try:
        page.goto(f"{BASE_URL}/containers", wait_until="domcontentloaded")
        # On mobile, the expand button might be different or hidden in cards
        expand_buttons = page.locator('button[title="Expand details"]').all()
        if not expand_buttons:
            # Fallback for cards view manage buttons if expand isn't available
            expand_buttons = page.locator('button:has-text("Manage")').all()
            
        print(f"  [{device_name}] Found {len(expand_buttons)} interactive elements.")
        
        for i, btn in enumerate(expand_buttons[:3]): 
            print(f"  [{device_name}] Interacting with element {i+1}...")
            btn.click()
            page.wait_for_timeout(2000)
            page.screenshot(path=os.path.join(RESULTS_DIR, f"interaction_{device_name}_{i+1}.png"))
            # If it was an expand, click again to collapse
            if "Expand" in (btn.get_attribute("title") or ""):
                btn.click()
                page.wait_for_timeout(500)
                
    except Exception as e:
        print(f"  [{device_name}] Interaction test failed: {e}")

    return console_logs

def run_audit():
    with sync_playwright() as p:
        print(f"Connecting to Playwright at {PLAYWRIGHT_WS}...")
        browser = p.chromium.connect(PLAYWRIGHT_WS)
        
        routes = ["/", "/containers", "/stacks", "/images", "/audit", "/diagnostics", "/settings"]
        all_logs = []

        # Desktop Context
        desktop_context = browser.new_context(viewport={"width": 1440, "height": 900})
        all_logs.extend(run_suite(desktop_context, "desktop", routes))
        
        # Mobile Context (Pixel 5)
        mobile_device = p.devices["Pixel 5"]
        mobile_context = browser.new_context(**mobile_device)
        all_logs.extend(run_suite(mobile_context, "mobile", routes))

        # Capture all logs
        with open(os.path.join(RESULTS_DIR, "full_audit_report.json"), "w") as f:
            json.dump(all_logs, f, indent=2)

        hangs = [log for log in all_logs if "UI_HANG_DETECTED" in log["text"]]
        long_tasks = [log for log in all_logs if "LONG_TASK_DETECTED" in log["text"]]
        
        print("\n" + "="*40)
        print("FINAL AUDIT PERFORMANCE SUMMARY")
        print("="*40)
        print(f"Total UI Hangs (>250ms): {len(hangs)}")
        print(f"Total Long Tasks (>100ms): {len(long_tasks)}")
        
        if hangs:
            print("\nHANG DETAILS:")
            for h in hangs:
                print(f"  - [{h['device']}] {h['text']}")
        else:
            print("\n✅ NO UI HANGS DETECTED DURING INTERACTIONS")
        
        print(f"\nAudit complete. Results saved to {RESULTS_DIR}")
        browser.close()

if __name__ == "__main__":
    run_audit()
