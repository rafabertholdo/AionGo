import unittest
from unittest.mock import patch
import urllib.error
from pathlib import Path
import images


class ImageTests(unittest.TestCase):
    def test_every_role_has_a_unique_version_and_source_tag(self):
        manifest = images.load_manifest()
        references = [reference for role in manifest["images"]
                      for reference in images.references(manifest, role, "abcdef0123456789")]
        self.assertEqual(len(references), len(set(references)))
        self.assertTrue(all(reference.startswith("docker.io/rafabertholdo/aiongo:1.9-") for reference in references))
        self.assertTrue(all(not reference.endswith(":1.9") for reference in references))

    def test_go_and_java_have_separate_release_tags(self):
        manifest = images.load_manifest()
        self.assertEqual(images.references(manifest, "game-go", "abcdef0123456789")[0],
                         "docker.io/rafabertholdo/aiongo:1.9-game-go-v0.1.0")
        self.assertEqual(images.references(manifest, "game-java21", "abcdef0123456789")[0],
                         "docker.io/rafabertholdo/aiongo:1.9-game-java21-v0.1.0")

    def test_build_contexts_and_targets_match_each_role(self):
        manifest = images.load_manifest()
        for role, spec in manifest["images"].items():
            command = images.build_command(manifest, role, "abcdef", Path("/source"))
            self.assertEqual(command[-1], str(Path("/source") / spec["context"]))
            self.assertEqual(command[command.index("-f") + 1], str(Path("/source") / spec["dockerfile"]))
            if "target" in spec:
                self.assertEqual(command[command.index("--target") + 1], spec["target"])
            else:
                self.assertNotIn("--target", command)

    def test_published_tags_cannot_be_overwritten(self):
        with patch("images.urllib.request.urlopen"):
            with self.assertRaises(RuntimeError):
                images.require_new_tag("docker.io/rafabertholdo/aiongo:1.9-game-go-v0.1.0")

    def test_missing_tags_are_publishable_but_registry_errors_are_not_ignored(self):
        reference = "docker.io/rafabertholdo/aiongo:1.9-game-go-v0.1.0"
        with patch("images.urllib.request.urlopen", side_effect=urllib.error.HTTPError("url", 404, "missing", {}, None)):
            images.require_new_tag(reference)
        with patch("images.urllib.request.urlopen", side_effect=urllib.error.HTTPError("url", 403, "denied", {}, None)):
            with self.assertRaises(urllib.error.HTTPError):
                images.require_new_tag(reference)


if __name__ == "__main__":
    unittest.main()
