#!/usr/bin/env python3
"""Build and publish versioned AionGo images without changing legacy tags."""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def load_manifest():
    manifest = json.loads((ROOT / "image-manifest.json").read_text())
    if not re.fullmatch(r"\d+\.\d+\.\d+", manifest["release"]):
        raise ValueError("Use a three-component release version")
    if manifest["repository"] != "docker.io/rafabertholdo/aiongo":
        raise ValueError("Publishing is isolated to the AionGo image repository")
    return manifest


def references(manifest, role, revision):
    prefix = f'{manifest["repository"]}:{manifest["clientVersion"]}-{manifest["images"][role]["variant"]}'
    return f'{prefix}-v{manifest["release"]}', f'{prefix}-sha-{revision[:12]}'


def build_command(manifest, role, revision, context):
    spec = manifest["images"][role]
    command = ["container", "build", "--platform", manifest["platform"],
               "--progress", "plain", "--cpus", "2", "--memory", "4G",
               "-f", str(context / spec["dockerfile"]),
               "-t", references(manifest, role, revision)[0],
               "--label", "org.opencontainers.image.source=https://github.com/rafabertholdo/AionGo",
               "--label", f"org.opencontainers.image.revision={revision}",
               "--label", f'org.opencontainers.image.version={manifest["release"]}']
    if "target" in spec:
        command += ["--target", spec["target"]]
    return command + [str(context / spec["context"])]


def require_new_tag(reference):
    tag = reference.rsplit(":", 1)[1]
    url = f"https://hub.docker.com/v2/repositories/rafabertholdo/aiongo/tags/{tag}"
    try:
        with urllib.request.urlopen(url, timeout=30):
            raise RuntimeError(f"Refusing to overwrite published tag: {reference}")
    except urllib.error.HTTPError as error:
        if error.code != 404:
            raise


def validate_image_pairs(manifest, roles, revision, local_images):
    digests = {row["reference"]: row["descriptor"]["digest"] for row in local_images}
    for role in roles:
        versioned, source = references(manifest, role, revision)
        if versioned not in digests or source not in digests:
            raise ValueError(f"Build is missing for {role} at revision {revision}")
        if digests[versioned] != digests[source]:
            raise ValueError(f"Version and source tags identify different builds for {role}")


def source_revision(requested, action):
    head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    revision = head if requested is None else subprocess.check_output(
        ["git", "rev-parse", "--verify", f"{requested}^{{commit}}"], cwd=ROOT, text=True).strip()
    if action == "build" and revision != head:
        raise ValueError("Build from the checked-out revision; --revision selects an existing build for push/list")
    return revision


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("build", "push", "list"))
    parser.add_argument("roles", nargs="*")
    parser.add_argument("--revision", help="Source revision of an existing build (push/list)")
    args = parser.parse_args()
    manifest = load_manifest()
    roles = args.roles or list(manifest["images"])
    for role in roles:
        if role not in manifest["images"]:
            parser.error(f"Unknown image role: {role}")
    revision = source_revision(args.revision, args.action)
    if args.action == "list":
        for role in roles:
            print(role, *references(manifest, role, revision))
        return
    if args.action == "push":
        local_images = json.loads(subprocess.check_output(
            ["container", "image", "list", "--format", "json"], text=True))
        validate_image_pairs(manifest, roles, revision, local_images)
        # Check every tag before publishing anything; a version must identify one
        # source revision for all roles. A partial upload can be resumed manually.
        for role in roles:
            for reference in references(manifest, role, revision):
                require_new_tag(reference)
        for role in roles:
            for reference in references(manifest, role, revision):
                subprocess.run(["container", "image", "push", "--disable-progress-updates", reference], check=True)
        return
    subprocess.run(["git", "diff", "--exit-code", "HEAD"], cwd=ROOT, check=True, stdout=subprocess.DEVNULL)
    output = ROOT / ".build"
    output.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="images-", dir=output) as directory:
        context = Path(directory)
        ignore = shutil.ignore_patterns("target", ".git", ".quest-debug", ".quest-claims", "__pycache__", "*.log")
        for name in ("go", "java", "docker"):
            shutil.copytree(ROOT / name, context / name, ignore=ignore)
        assets = Path(os.environ.get("AION_PANEL_ASSETS_PATH", ROOT / ".build/panel-assets"))
        if assets.is_dir():
            shutil.copytree(assets, context / "panel-assets")
        else:
            (context / "panel-assets").mkdir()
        for role in roles:
            # Reset before every build: Apple's persistent builder can retain
            # stale contexts or crash on the next role. Game containers are kept.
            subprocess.run(["pkill", "-f", "container-runtime-linux.*containers/buildkit"], check=False)
            import time
            time.sleep(2)
            subprocess.run(["container", "delete", "--force", "buildkit"], check=False,
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            subprocess.run(build_command(manifest, role, revision, context), check=True)
            versioned, source = references(manifest, role, revision)
            subprocess.run(["container", "image", "tag", versioned, source], check=True)


if __name__ == "__main__":
    main()
