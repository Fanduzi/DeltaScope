#!/usr/bin/env python3
"""Pin the rendered cask's declarative Homebrew postflight contract."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parent
STEPS = '''  postflight_steps do
    on_macos do
      run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "{{staged_path}}/deltascope"]
    end
  end'''


class HomebrewCaskTest(unittest.TestCase):
    def setUp(self):
        self.env = dict(os.environ, VERSION="v0.514.0",
                        DARWIN_AMD64_SHA256="a" * 64, DARWIN_ARM64_SHA256="b" * 64)
        self.cask = subprocess.check_output(
            ["bash", str(SCRIPTS / "render_homebrew_cask.sh")],
            env=self.env, text=True,
        )

    def verify(self, content):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "deltascope.rb"
            path.write_text(content)
            return subprocess.run(
                ["bash", str(SCRIPTS / "verify_homebrew_cask.sh")],
                env=dict(self.env, CASK=str(path)), capture_output=True, text=True,
            )

    def test_rendered_steps(self):
        self.assertIn(STEPS, self.cask)
        result = self.verify(self.cask)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_rejects_legacy_step_constructs(self):
        for old, new in [
            ("postflight_steps do", "postflight do"),
            ("    on_macos do", "    if OS.mac?"),
            ('run "/usr/bin/xattr"', 'system_command "/usr/bin/xattr"'),
            ("{{staged_path}}", "#{staged_path}"),
        ]:
            with self.subTest(construct=new):
                self.assertIn(old, self.cask)
                result = self.verify(self.cask.replace(old, new))
                self.assertNotEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
