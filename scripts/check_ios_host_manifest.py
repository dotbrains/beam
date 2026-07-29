#!/usr/bin/env python3
import json
import re
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "ios" / "host-manifest.json"
SOURCES = ROOT / "ios" / "Sources" / "BeamAppCore"
IOS_ROOT = ROOT / "ios"


def fail(message: str) -> None:
    print(message, file=sys.stderr)
    raise SystemExit(1)


def require_nonempty_string(section: dict, field: str, path: str) -> str:
    value = section.get(field)
    if not isinstance(value, str) or not value.strip():
        fail(f"{path}.{field} must be a non-empty string")
    return value


def require_nonempty_list(section: dict, field: str, path: str) -> list[str]:
    value = section.get(field)
    if not isinstance(value, list) or not value:
        fail(f"{path}.{field} must be a non-empty list")
    for item in value:
        if not isinstance(item, str) or not item.strip():
            fail(f"{path}.{field} must contain only non-empty strings")
    return value


def validate_bundle_id(bundle_id: str, path: str) -> None:
    if not re.fullmatch(r"[A-Za-z0-9]+(?:\.[A-Za-z0-9-]+)+", bundle_id):
        fail(f"{path}.bundleIdentifier is not a valid reverse-DNS identifier")


def validate_minimum_os(version: str, path: str) -> None:
    if not re.fullmatch(r"\d+\.\d+", version):
        fail(f"{path}.minimumOSVersion must use major.minor format")
    major, minor = (int(part) for part in version.split("."))
    if (major, minor) < (17, 0):
        fail(f"{path}.minimumOSVersion must be at least 17.0")


def swift_symbols() -> set[str]:
    symbols: set[str] = set()
    for path in SOURCES.glob("*.swift"):
        text = path.read_text()
        symbols.update(re.findall(r"public\s+(?:final\s+)?(?:class|struct|enum|protocol)\s+([A-Za-z_][A-Za-z0-9_]*)", text))
    return symbols


def validate_target(name: str, target: dict, symbols: set[str]) -> None:
    path = name
    bundle_id = require_nonempty_string(target, "bundleIdentifier", path)
    validate_bundle_id(bundle_id, path)
    require_nonempty_string(target, "displayName", path)
    minimum_os = require_nonempty_string(target, "minimumOSVersion", path)
    validate_minimum_os(minimum_os, path)
    entry_point = require_nonempty_string(target, "entryPoint", path)
    if not entry_point.endswith(".swift"):
        fail(f"{path}.entryPoint must be a Swift file")
    entry_path = IOS_ROOT / entry_point
    if not entry_path.is_file():
        fail(f"{path}.entryPoint does not exist at ios/{entry_point}")
    entry_text = entry_path.read_text()
    if "import BeamAppCore" not in entry_text:
        fail(f"{path}.entryPoint must import BeamAppCore")
    capabilities = require_nonempty_list(target, "requiredCapabilities", path)
    entitlements = require_nonempty_list(target, "entitlements", path)
    for capability in capabilities:
        if capability not in entitlements:
            fail(f"{path}.requiredCapabilities contains {capability} without matching entitlement")
    for type_name in require_nonempty_list(target, "usesPackageTypes", path):
        if type_name not in symbols:
            fail(f"{path}.usesPackageTypes references unknown BeamAppCore type {type_name}")
        if not re.search(rf"\b{re.escape(type_name)}\b", entry_text):
            fail(f"{path}.entryPoint does not reference required type {type_name}")


def parse_host_entry_points(manifest: dict) -> None:
    subprocess.run(["swift", "build", "--package-path", str(IOS_ROOT)], cwd=ROOT, check=True)
    module_dirs = sorted((IOS_ROOT / ".build").glob("*/debug/Modules"))
    if not module_dirs:
        fail("ios package build did not produce a debug Modules directory")
    entry_points = [
        str(IOS_ROOT / manifest["app"]["entryPoint"]),
        str(IOS_ROOT / manifest["widgetExtension"]["entryPoint"]),
    ]
    subprocess.run(["swiftc", "-parse", "-I", str(module_dirs[-1]), *entry_points], cwd=ROOT, check=True)


def main() -> None:
    if not MANIFEST.exists():
        fail("ios/host-manifest.json is missing")
    manifest = json.loads(MANIFEST.read_text())
    if not isinstance(manifest, dict):
        fail("ios/host-manifest.json must contain a JSON object")
    app = manifest.get("app")
    widget = manifest.get("widgetExtension")
    if not isinstance(app, dict):
        fail("app target manifest is missing")
    if not isinstance(widget, dict):
        fail("widgetExtension target manifest is missing")
    symbols = swift_symbols()
    validate_target("app", app, symbols)
    validate_target("widgetExtension", widget, symbols)
    if app["bundleIdentifier"] == widget["bundleIdentifier"]:
        fail("app and widgetExtension bundle identifiers must differ")
    if not widget["bundleIdentifier"].startswith(app["bundleIdentifier"] + "."):
        fail("widgetExtension bundle identifier must be nested under the app bundle identifier")
    parse_host_entry_points(manifest)


if __name__ == "__main__":
    main()
