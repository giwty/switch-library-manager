"""Publish generated title data whose contents differ from the release."""

import hashlib
import json
import os
import re
import subprocess
from pathlib import Path
from tempfile import TemporaryDirectory
from urllib.error import HTTPError
from urllib.request import Request, urlopen


FILENAMES = ("titles.json", "versions.json")
RELEASE_TAG = "data"
RELEASE_NOTES = (
    "Rolling US English titles.json and versions.json generated from "
    "blawar/titledb using blawar/nut."
)


def get_release():
    api = os.environ["GITHUB_API_URL"]
    repo = os.environ["GH_REPO"]
    request = Request(
        f"{api}/repos/{repo}/releases/tags/{RELEASE_TAG}",
        headers={
            "Accept": "application/vnd.github+json",
            "Authorization": f"Bearer {os.environ['GH_TOKEN']}",
            "X-GitHub-Api-Version": "2022-11-28",
        },
    )
    try:
        with urlopen(request, timeout=60) as response:
            return json.load(response)
    except HTTPError as error:
        if error.code == 404:
            return None
        raise


def uploaded_assets(release):
    return {
        asset["name"]: asset
        for asset in release["assets"]
        if asset["name"] in FILENAMES and asset["state"] == "uploaded" and asset["size"] > 0
    }


def file_digest(path):
    digest = hashlib.sha256()
    with path.open("rb") as file:
        for chunk in iter(lambda: file.read(1024 * 1024), b""):
            digest.update(chunk)
    return f"sha256:{digest.hexdigest()}"


def asset_digest(asset):
    digest = asset.get("digest")
    if re.fullmatch(r"sha256:[0-9a-f]{64}", digest or ""):
        return digest
    with TemporaryDirectory() as directory:
        path = Path(directory) / asset["name"]
        subprocess.run(
            ["gh", "release", "download", RELEASE_TAG, "--pattern", asset["name"], "--output", str(path)],
            check=True,
        )
        return file_digest(path)


def publish(data_dir):
    digests = {filename: file_digest(data_dir / filename) for filename in FILENAMES}
    release = get_release()
    if release is None:
        subprocess.run(
            [
                "gh", "release", "create", RELEASE_TAG, "--prerelease", "--latest=false",
                "--title", "Switch Library Manager title data", "--notes", RELEASE_NOTES,
            ],
            check=True,
        )
    assets = uploaded_assets(release) if release is not None else {}
    for filename, digest in digests.items():
        asset = assets.get(filename)
        if asset is not None and asset_digest(asset) == digest:
            print(f"Skipping upload for {filename} because the published file matches.", flush=True)
            continue
        print(f"Uploading {filename} because the published file is missing or differs.", flush=True)
        subprocess.run(
            ["gh", "release", "upload", RELEASE_TAG, str(data_dir / filename), "--clobber"],
            check=True,
        )


if __name__ == "__main__":
    publish(Path("generated-data"))
