import os
import json
import time
from playwright.sync_api import sync_playwright

PLAYWRIGHT_WS = "ws://playwright-service:3000/"
BASE_URL = "http://harborwatch-dev:8000" 
RESULTS_DIR = "/config/workspace/playwright-results/harborwatch-audit"

os.makedirs(RESULTS_DIR, exist_ok=True)

def run_diagnostics(context, device_name):
    page = context.new_page()
    logs = []
    
    # Listeners
    page.on("console", lambda msg: logs.append(f"[{device_name}][CONSOLE] {msg.type}: {msg.text}"))
    page.on("pageerror", lambda err: logs.append(f"[{device_name}][PAGE_ERROR] {err.message}"))
    page.on("requestfailed", lambda req: logs.append(f"[{device_name}][REQ_FAIL] {req.method} {req.url} - {req.failure.error_text}"))

    print(f"--- Starting Detailed Diagnostics for {device_name} ---")
    
    try:
        # 1. Initial Load
        print(f"  [{device_name}] Loading Dashboard...")
        page.goto(BASE_URL, wait_until="networkidle", timeout=30000)
        page.screenshot(path=os.path.join(RESULTS_DIR, f"diag_{device_name}_1_load.png"))
        
        # 2. Navigate to Containers
        print(f"  [{device_name}] Navigating to Fleet...")
        # Click sidebar button if visible or goto
        page.goto(f"{BASE_URL}/containers", wait_until="networkidle", timeout=30000)
        page.wait_for_timeout(2000) # Wait for hydration
        page.screenshot(path=os.path.join(RESULTS_DIR, f"diag_{device_name}_2_fleet.png"))

        # 3. Interaction: Toggle View (List/Cards)
        print(f"  [{device_name}] Attempting View Toggle...")
        toggle_selector = 'button[title="Card View"], button[title="List View"]'
        try:
            # Check if elements are present at all
            count = page.locator(toggle_selector).count()
            print(f"  [{device_name}] Found {count} toggle buttons.")
            
            if count > 0:
                btn = page.locator(toggle_selector).first
                print(f"  [{device_name}] Clicking toggle...")
                btn.click(timeout=5000)
                page.wait_for_timeout(1000)
                page.screenshot(path=os.path.join(RESULTS_DIR, f"diag_{device_name}_3_toggle.png"))
            else:
                print(f"  [{device_name}] WARNING: No toggle buttons found. Checking for empty state text...")
                if "No containers found" in page.inner_text("body"):
                    print(f"  [{device_name}] Confirmed: 'No containers found' empty state is visible.")
                else:
                    print(f"  [{device_name}] ALERT: Neither buttons nor empty state found. Page might be hung or broken.")
                body_text = page.inner_text("body")
                logs.append(f"[{device_name}] Body Text Snippet: {body_text[:200]}")
        except Exception as e:
            print(f"  [{device_name}] Interaction failed: {str(e)}")
            logs.append(f"[{device_name}] INTERACTION_ERROR: {str(e)}")

        # 4. Interaction: Expand Details
        print(f"  [{device_name}] Attempting Expand Details...")
        expand_selector = 'button[title="Expand details"]'
        try:
            if page.locator(expand_selector).count() > 0:
                btn = page.locator(expand_selector).first
                print(f"  [{device_name}] Clicking Expand...")
                btn.click(timeout=5000)
                print(f"  [{device_name}] Click successful, waiting for metrics...")
                page.wait_for_timeout(3000)
                page.screenshot(path=os.path.join(RESULTS_DIR, f"diag_{device_name}_4_expand.png"))
            else:
                print(f"  [{device_name}] No expand buttons found.")
        except Exception as e:
            print(f"  [{device_name}] Expand failed: {str(e)}")
            logs.append(f"[{device_name}] EXPAND_ERROR: {str(e)}")

    except Exception as e:
        print(f"  [{device_name}] Critical error: {e}")
        logs.append(f"[{device_name}] CRITICAL: {str(e)}")

    return logs

def run_audit():
    with sync_playwright() as p:
        print(f"Connecting to Playwright at {PLAYWRIGHT_WS}...")
        browser = p.chromium.connect(PLAYWRIGHT_WS)
        
        all_logs = []

        # Desktop
        desktop_context = browser.new_context(viewport={"width": 1440, "height": 900})
        all_logs.extend(run_diagnostics(desktop_context, "desktop"))
        
        # Mobile
        mobile_device = p.devices["Pixel 5"]
        mobile_context = browser.new_context(**mobile_device)
        all_logs.extend(run_diagnostics(mobile_context, "mobile"))

        # Final Report
        report_path = os.path.join(RESULTS_DIR, "detailed_diag_report.txt")
        with open(report_path, "w") as f:
            for log in all_logs:
                f.write(log + "\n")

        print("\n" + "="*40)
        print("DIAGNOSTIC LOG SUMMARY")
        print("="*40)
        for log in all_logs:
            if "ERROR" in log or "FAIL" in log:
                print(f"  !! {log}")
        
        print(f"\nDiagnostics complete. Results saved to {RESULTS_DIR}")
        browser.close()

if __name__ == "__main__":
    run_audit()
