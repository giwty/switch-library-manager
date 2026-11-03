import hashlib
import io
import json
import os
import subprocess
import tempfile
import unittest
from contextlib import ExitStack
from pathlib import Path
from unittest.mock import patch
from urllib.error import HTTPError

import update_title_data as task


class TitleDataTests(unittest.TestCase):
    def setUp(self):
        context = ExitStack()
        self.addCleanup(context.close)
        self.directory = Path(context.enter_context(tempfile.TemporaryDirectory()))
        self.payloads = {name: b'{"old": true}\n' for name in task.FILENAMES}
        for name, payload in self.payloads.items():
            (self.directory / name).write_bytes(payload)
        self.release = {"assets": []}
        for name in task.FILENAMES:
            self.add_asset(name)
        self.uploads = []
        self.downloads = []
        self.fail_upload = None
        context.enter_context(patch.dict(os.environ, {
            "GITHUB_API_URL": "https://api.example.invalid",
            "GH_REPO": "example/switch-library-manager",
            "GH_TOKEN": "test-token",
        }))
        context.enter_context(patch.object(task, "urlopen", side_effect=self.release_response))
        context.enter_context(patch.object(task.subprocess, "run", side_effect=self.gh))

    def add_asset(self, name):
        payload = self.payloads[name]
        self.release["assets"].append({
            "name": name,
            "state": "uploaded",
            "size": len(payload),
            "digest": f"sha256:{hashlib.sha256(payload).hexdigest()}",
        })

    def release_response(self, request, *, timeout):
        self.assertEqual(
            request.full_url,
            "https://api.example.invalid/repos/example/switch-library-manager/releases/tags/data",
        )
        self.assertEqual(request.get_method(), "GET")
        self.assertGreater(timeout, 0)
        if self.release is None:
            raise HTTPError(request.full_url, 404, "Not found", {}, None)
        return io.BytesIO(json.dumps(self.release).encode())

    def gh(self, args, *, check):
        self.assertTrue(check)
        self.assertEqual(args[:2], ["gh", "release"])
        self.assertEqual(args[3], "data")
        command = args[2]
        if command == "upload":
            path = Path(args[4])
            self.assertEqual(args[5:], ["--clobber"])
            self.uploads.append(path.name)
            self.release["assets"] = [a for a in self.release["assets"] if a["name"] != path.name]
            if self.fail_upload == path.name:
                raise subprocess.CalledProcessError(1, args)
            self.payloads[path.name] = path.read_bytes()
            self.add_asset(path.name)
        elif command == "download":
            name = args[5]
            self.downloads.append(name)
            Path(args[7]).write_bytes(self.payloads[name])
        elif command == "create":
            self.assertIsNone(self.release)
            self.assertIn("--prerelease", args)
            self.release = {"assets": []}
        else:
            self.fail(f"Unexpected GitHub CLI call: {args}")

    def test_unchanged_files_do_not_require_upload_or_download(self):
        task.publish(self.directory)
        task.subprocess.run.assert_not_called()

    def test_only_changed_asset_is_uploaded(self):
        (self.directory / "versions.json").write_bytes(b'{"new": true}\n')
        task.publish(self.directory)
        self.assertEqual(self.uploads, ["versions.json"])

    def test_incomplete_asset_is_replaced(self):
        self.release["assets"][1]["state"] = "starter"
        task.publish(self.directory)
        self.assertEqual(self.uploads, ["versions.json"])

    def test_missing_digest_downloads_only_the_affected_asset(self):
        self.release["assets"][0]["digest"] = None
        task.publish(self.directory)
        self.assertEqual(self.downloads, ["titles.json"])
        self.assertEqual(self.uploads, [])

    def test_retry_after_partial_upload_only_repairs_missing_asset(self):
        for name in task.FILENAMES:
            (self.directory / name).write_bytes(b'{"new": true}\n')
        self.fail_upload = "versions.json"
        with self.assertRaises(subprocess.CalledProcessError):
            task.publish(self.directory)

        self.fail_upload = None
        task.publish(self.directory)
        self.assertEqual(self.uploads, ["titles.json", "versions.json", "versions.json"])

    def test_missing_release_is_created(self):
        self.release = None
        task.publish(self.directory)
        self.assertEqual(self.uploads, list(task.FILENAMES))

    def test_api_failure_does_not_create_or_upload(self):
        error = HTTPError("tags/data", 503, "Unavailable", {}, None)
        with patch.object(task, "urlopen", side_effect=error), self.assertRaises(HTTPError):
            task.publish(self.directory)
        task.subprocess.run.assert_not_called()


if __name__ == "__main__":
    unittest.main()
