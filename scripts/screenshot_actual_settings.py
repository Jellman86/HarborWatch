import os
from playwright.sync_api import sync_playwright

PLAYWRIGHT_WS = os.getenv("PLAYWRIGHT_WS", "ws://playwright-service:3000/")
BASE_URL = os.getenv("BASE_URL", "http://harborwatch-dev:8000")

with sync_playwright() as p:
    browser = p.chromium.connect(PLAYWRIGHT_WS)
    page = browser.new_page(viewport={"width": 1440, "height": 900})
    
    print("Navigating to App...")
    page.goto(BASE_URL)
    page.wait_for_load_state("domcontentloaded")
    
    # Click on the Settings tab in the sidebar
    print("Clicking Settings button...")
    page.locator('button:has-text("Settings")').click()
    
    # Wait for the settings page to render
    page.wait_for_timeout(2000)
    
    # Take full page screenshot
    page.screenshot(path="settings_actual.png", full_page=True)
    print("Saved actual settings screenshot.")

    browser.close()
