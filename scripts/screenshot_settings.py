import os
from playwright.sync_api import sync_playwright

PLAYWRIGHT_WS = os.getenv("PLAYWRIGHT_WS", "ws://playwright-service:3000/")
BASE_URL = os.getenv("BASE_URL", "http://harborwatch-dev:8000")

with sync_playwright() as p:
    browser = p.chromium.connect(PLAYWRIGHT_WS)
    page = browser.new_page(viewport={"width": 1440, "height": 900})
    
    # Go to settings page
    print("Navigating to Settings...")
    page.goto(f"{BASE_URL}/settings")
    page.wait_for_load_state("domcontentloaded")
    
    # Give it a bit of time to render Svelte components
    page.wait_for_timeout(2000)
    
    # Take full page screenshot
    page.screenshot(path="settings_main.png", full_page=True)
    print("Saved main settings screenshot.")

    browser.close()
