from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
global_progress = (ROOT / "web/src/lib/components/GlobalProgress.svelte").read_text()
app = (ROOT / "web/src/App.svelte").read_text()


def assert_contains(haystack: str, needle: str, message: str) -> None:
    if needle not in haystack:
        raise AssertionError(message)


assert_contains(
    global_progress,
    "sticky top-[var(--global-progress-sticky-top)]",
    "GlobalProgress should render the visible bar inside a sticky shell anchored by the shared top offset variable.",
)
assert_contains(
    app,
    "--global-progress-sticky-top",
    "App shell should define the shared sticky offset variable used by the global progress bar.",
)

print("ok")
