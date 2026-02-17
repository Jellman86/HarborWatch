import json
import os
import re
import time
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

from playwright.sync_api import Browser, BrowserContext, Error, Page, sync_playwright

PLAYWRIGHT_WS = os.getenv("PLAYWRIGHT_WS", "ws://playwright-service:3000/")
BASE_URL = os.getenv("BASE_URL", "http://harborwatch-dev:8000")
RUN_ID = datetime.utcnow().strftime("%Y%m%d-%H%M%S")
RESULTS_DIR = Path(os.getenv("RESULTS_DIR", f"/config/workspace/playwright-results/harborwatch-full-audit/{RUN_ID}"))


@dataclass
class AuditState:
    device: str
    step: str = "init"
    screenshot_index: int = 0
    events: List[Dict[str, Any]] = field(default_factory=list)

    def log_event(self, kind: str, message: str, **extra: Any) -> None:
        self.events.append(
            {
                "ts": time.time(),
                "device": self.device,
                "step": self.step,
                "kind": kind,
                "message": message,
                **extra,
            }
        )


def safe_name(value: str) -> str:
    return re.sub(r"[^a-zA-Z0-9_-]+", "-", value).strip("-").lower()[:80]


def attach_page_listeners(page: Page, state: AuditState) -> None:
    def on_console(msg: Any) -> None:
        location = msg.location
        state.log_event(
            "console",
            msg.text,
            level=msg.type,
            url=location.get("url") if location else "",
            line=location.get("lineNumber") if location else None,
            col=location.get("columnNumber") if location else None,
        )

    def on_page_error(err: Exception) -> None:
        state.log_event("pageerror", str(err))

    def on_request_failed(req: Any) -> None:
        failure = req.failure
        if isinstance(failure, str):
            error_text = failure
        elif isinstance(failure, dict):
            error_text = str(failure.get("errorText", ""))
        elif failure is not None:
            error_text = str(getattr(failure, "error_text", "") or failure)
        else:
            error_text = ""
        state.log_event(
            "requestfailed",
            f"{req.method} {req.url}",
            error_text=error_text,
        )

    def on_response(resp: Any) -> None:
        status = resp.status
        if status >= 400:
            state.log_event("response_error", f"{resp.request.method} {resp.url}", status=status)

    page.on("console", on_console)
    page.on("pageerror", on_page_error)
    page.on("requestfailed", on_request_failed)
    page.on("response", on_response)


def set_step(state: AuditState, name: str) -> None:
    state.step = name


def screenshot(page: Page, state: AuditState, label: str) -> str:
    state.screenshot_index += 1
    filename = f"{state.screenshot_index:03d}_{safe_name(state.device)}_{safe_name(label)}.png"
    out = RESULTS_DIR / "screenshots" / filename
    out.parent.mkdir(parents=True, exist_ok=True)
    page.screenshot(path=str(out), full_page=True)
    state.log_event("screenshot", str(out))
    return str(out)


def click_if_exists(page: Page, locator: str, state: AuditState, name: str, timeout: int = 2500) -> bool:
    loc = page.locator(locator)
    if loc.count() == 0:
        state.log_event("action_skip", f"{name}: locator not found", locator=locator)
        return False
    try:
        loc.first.click(timeout=timeout)
        state.log_event("action_ok", f"{name}: clicked", locator=locator)
        return True
    except Exception as exc:
        state.log_event("action_err", f"{name}: click failed", locator=locator, error=str(exc))
        return False


def click_sidebar(page: Page, state: AuditState, label: str) -> bool:
    # Open mobile sidebar if needed
    if page.locator("button[aria-label='Open Navigation']").count() > 0:
        nav_button = page.locator("aside nav button", has_text=label)
        title_button = page.locator(f"aside nav button[title='{label}']")
        if nav_button.count() == 0 and title_button.count() == 0:
            click_if_exists(page, "button[aria-label='Open Navigation']", state, f"open-nav-{label}")
            page.wait_for_timeout(150)

    candidates = [
        page.locator("aside nav button", has_text=label),
        page.locator(f"aside nav button[title='{label}']"),
    ]
    for cand in candidates:
        if cand.count() > 0:
            try:
                cand.first.click(timeout=4000)
                page.wait_for_timeout(700)
                state.log_event("action_ok", f"nav:{label}")
                return True
            except Exception as exc:
                state.log_event("action_err", f"nav:{label} failed", error=str(exc))
                return False

    state.log_event("action_skip", f"nav:{label} not found")
    return False


def goto_home(page: Page, state: AuditState) -> None:
    set_step(state, "goto-home")
    page.goto(BASE_URL, wait_until="domcontentloaded", timeout=45000)
    page.wait_for_timeout(1000)
    screenshot(page, state, "dashboard-home")


def fleet_and_nginx_security(page: Page, state: AuditState) -> None:
    set_step(state, "fleet-open")
    click_sidebar(page, state, "Fleet")
    page.wait_for_timeout(1200)
    screenshot(page, state, "fleet-default")

    set_step(state, "fleet-toggle-view")
    click_if_exists(page, "button[title='Card View']", state, "to-card-view")
    page.wait_for_timeout(600)
    screenshot(page, state, "fleet-card-view")
    click_if_exists(page, "button[title='List View']", state, "to-list-view")
    page.wait_for_timeout(600)
    screenshot(page, state, "fleet-list-view")

    set_step(state, "fleet-expand-details")
    click_if_exists(page, "button[title='Expand details']", state, "expand-first-row")
    page.wait_for_timeout(1000)
    screenshot(page, state, "fleet-expanded")

    # Navigate to nginx-rp manage
    set_step(state, "fleet-nginx-manage")
    managed = False
    row = page.locator("tr", has_text="nginx-rp")
    if row.count() > 0:
        # Click the container name button if present
        name_btn = row.first.locator("button", has_text="nginx-rp")
        if name_btn.count() > 0:
            try:
                name_btn.first.click(timeout=5000)
                managed = True
            except Exception as exc:
                state.log_event("action_err", "click nginx name failed", error=str(exc))
        if not managed:
            manage_btn = row.first.locator("button[title='Manage Asset']")
            if manage_btn.count() > 0:
                try:
                    manage_btn.first.click(timeout=5000)
                    managed = True
                except Exception as exc:
                    state.log_event("action_err", "click manage asset failed", error=str(exc))

    if not managed:
        card = page.locator("div", has_text="nginx-rp")
        if card.count() > 0:
            btn = card.first.locator("button", has_text="Manage")
            if btn.count() > 0:
                try:
                    btn.first.click(timeout=5000)
                    managed = True
                except Exception as exc:
                    state.log_event("action_err", "click card manage failed", error=str(exc))

    if not managed:
        state.log_event("action_err", "Unable to open nginx-rp manage page")
        screenshot(page, state, "fleet-nginx-manage-not-found")
        return

    page.wait_for_timeout(1200)
    screenshot(page, state, "container-nginx-manage")

    # Required path: Manage -> Security
    set_step(state, "container-security-tab")
    clicked_security = click_if_exists(page, "button:has-text('security')", state, "open-security-tab")
    if not clicked_security:
        click_if_exists(page, "button", state, "open-security-fallback")
    page.wait_for_timeout(1200)
    screenshot(page, state, "container-nginx-security")

    # Click through all container tabs for broader coverage
    set_step(state, "container-tabs-sweep")
    for tab in ["insights", "security", "lifecycle", "configuration"]:
        click_if_exists(page, f"button:has-text('{tab}')", state, f"container-tab-{tab}")
        page.wait_for_timeout(700)
        screenshot(page, state, f"container-tab-{tab}")

    set_step(state, "container-back-to-fleet")
    click_if_exists(page, "button:has-text('Back to Fleet')", state, "back-to-fleet", timeout=5000)
    page.wait_for_timeout(1000)
    screenshot(page, state, "fleet-after-back")


def visit_main_nav(page: Page, state: AuditState) -> None:
    # Cover all top-level sections shown in sidebar
    nav_labels = [
        "Dashboard",
        "Fleet",
        "Stacks",
        "Images",
        "Audit Log",
        "System Health",
        "Settings",
    ]
    for label in nav_labels:
        set_step(state, f"nav-{safe_name(label)}")
        click_sidebar(page, state, label)
        page.wait_for_timeout(900)
        screenshot(page, state, f"nav-{label}")

        # settings deep interactions
        if label == "Settings":
            settings_interactions(page, state)


def settings_interactions(page: Page, state: AuditState) -> None:
    set_step(state, "settings-top-tabs")
    for tab in ["Automations", "AI", "Integrations", "System", "Appearance"]:
        click_if_exists(page, f"button:has-text('{tab}')", state, f"settings-tab-{tab}", timeout=4500)
        page.wait_for_timeout(700)
        screenshot(page, state, f"settings-tab-{tab}")

    set_step(state, "settings-automation-subtabs")
    click_if_exists(page, "button:has-text('Automations')", state, "settings-tab-Automations", timeout=4500)
    page.wait_for_timeout(600)
    for sub in ["Upgrades", "Maintenance", "Security"]:
        click_if_exists(page, f"button:has-text('{sub}')", state, f"automation-subtab-{sub}", timeout=3500)
        page.wait_for_timeout(700)
        screenshot(page, state, f"automation-subtab-{sub}")


def run_device(browser: Browser, device_name: str, context_kwargs: Dict[str, Any]) -> AuditState:
    context: BrowserContext = browser.new_context(**context_kwargs)
    state = AuditState(device=device_name)
    page = context.new_page()
    attach_page_listeners(page, state)

    try:
        goto_home(page, state)
        fleet_and_nginx_security(page, state)
        visit_main_nav(page, state)

        set_step(state, "complete")
        screenshot(page, state, "final-state")
    except Exception as exc:
        state.log_event("critical", f"Unhandled audit failure: {exc}")
        try:
            screenshot(page, state, "critical-failure")
        except Exception:
            pass
    finally:
        context.close()

    return state


def write_reports(states: List[AuditState]) -> None:
    RESULTS_DIR.mkdir(parents=True, exist_ok=True)

    all_events: List[Dict[str, Any]] = []
    for s in states:
        all_events.extend(s.events)

    logs_path = RESULTS_DIR / "console-network-pageerror-events.jsonl"
    with logs_path.open("w", encoding="utf-8") as f:
        for evt in all_events:
            f.write(json.dumps(evt, ensure_ascii=True) + "\n")

    # Summaries
    errors = [
        e
        for e in all_events
        if (
            e.get("kind") in {"pageerror", "requestfailed", "critical", "action_err", "response_error"}
            or (e.get("kind") == "console" and str(e.get("level", "")).lower() in {"error", "warning"})
        )
    ]

    by_kind: Dict[str, int] = {}
    by_message: Dict[str, int] = {}
    for e in errors:
        kind = str(e.get("kind"))
        by_kind[kind] = by_kind.get(kind, 0) + 1
        msg = str(e.get("message", "")).strip()
        if msg:
            by_message[msg] = by_message.get(msg, 0) + 1

    top_messages = sorted(by_message.items(), key=lambda x: x[1], reverse=True)[:40]
    summary = {
        "run_id": RUN_ID,
        "base_url": BASE_URL,
        "playwright_ws": PLAYWRIGHT_WS,
        "devices": [s.device for s in states],
        "event_count": len(all_events),
        "error_event_count": len(errors),
        "error_kinds": by_kind,
        "top_error_messages": [{"message": m, "count": c} for m, c in top_messages],
        "screenshots_dir": str(RESULTS_DIR / "screenshots"),
        "logs_file": str(logs_path),
    }

    with (RESULTS_DIR / "summary.json").open("w", encoding="utf-8") as f:
        json.dump(summary, f, indent=2)

    with (RESULTS_DIR / "summary.md").open("w", encoding="utf-8") as f:
        f.write(f"# HarborWatch UI Full Audit\n\n")
        f.write(f"- Run ID: `{RUN_ID}`\n")
        f.write(f"- Base URL: `{BASE_URL}`\n")
        f.write(f"- Playwright WS: `{PLAYWRIGHT_WS}`\n")
        f.write(f"- Events: `{len(all_events)}`\n")
        f.write(f"- Error Events: `{len(errors)}`\n\n")
        f.write("## Error Kinds\n")
        for k, v in sorted(by_kind.items(), key=lambda x: x[1], reverse=True):
            f.write(f"- {k}: {v}\n")
        f.write("\n## Top Error Messages\n")
        for item in top_messages:
            f.write(f"- ({item[1]}) {item[0]}\n")


def main() -> None:
    print(f"Running HarborWatch full UI audit")
    print(f"BASE_URL={BASE_URL}")
    print(f"PLAYWRIGHT_WS={PLAYWRIGHT_WS}")
    print(f"RESULTS_DIR={RESULTS_DIR}")

    with sync_playwright() as p:
        browser = p.chromium.connect(PLAYWRIGHT_WS)
        states: List[AuditState] = []

        desktop = run_device(
            browser,
            "desktop",
            {"viewport": {"width": 1512, "height": 982}},
        )
        states.append(desktop)

        mobile = run_device(
            browser,
            "mobile_pixel5",
            dict(p.devices["Pixel 5"]),
        )
        states.append(mobile)

        browser.close()

    write_reports(states)
    print("Audit complete")


if __name__ == "__main__":
    main()
