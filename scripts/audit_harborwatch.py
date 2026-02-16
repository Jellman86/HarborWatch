import os
import json
import time
from playwright.sync_api import sync_playwright

PLAYWRIGHT_WS = "ws://playwright-service:3000/"
# Using the container name as per the guide
BASE_URL = "http://harborwatch-dev:8000" 
RESULTS_DIR = "/config/workspace/playwright-results/harborwatch-audit"

os.makedirs(RESULTS_DIR, exist_ok=True)

def run_audit():
    with sync_playwright() as p:
        print(f"Connecting to Playwright at {PLAYWRIGHT_WS}...")
        browser = p.chromium.connect(PLAYWRIGHT_WS)
        context = browser.new_context(viewport={"width": 1440, "height": 900})
        
        page = context.new_page()
        
        console_logs = []
        page.on("console", lambda msg: console_logs.append({
            "type": msg.type,
            "text": msg.text,
            "location": msg.location
        }))

        # List of routes to audit
        routes = [
            "/",
            "/containers",
            "/stacks",
            "/images",
            "/audit",
            "/diagnostics",
            "/settings"
        ]

        audit_results = {}

        for route in routes:
            url = f"{BASE_URL}{route}"
            print(f"Auditing {url}...")
            
            try:
                page.goto(url, wait_until="domcontentloaded", timeout=10000)
                time.sleep(2) # Give dynamic content (sparklines, etc) a moment to settle
                
                name = route.replace("/", "") or "dashboard"
                screenshot_path = os.path.join(RESULTS_DIR, f"{name}.png")
                page.screenshot(path=screenshot_path, full_page=True)
                
                audit_results[route] = {
                    "status": "success",
                    "screenshot": screenshot_path,
                    "title": page.title()
                }
            except Exception as e:
                print(f"Failed to audit {route}: {e}")
                audit_results[route] = {"status": "failed", "error": str(e)}

        # Capture console logs
        with open(os.path.join(RESULTS_DIR, "console_logs.json"), "w") as f:
            json.dump(console_logs, f, indent=2)

        print(f"Audit complete. Results saved to {RESULTS_DIR}")
        browser.close()

if __name__ == "__main__":
    run_audit()
