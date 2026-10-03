"""Keep the command website catalog complete against the Java reference."""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]

class CommandCatalogTests(unittest.TestCase):
    def test_catalog_covers_every_java_admin_command(self):
        java = set()
        for path in (ROOT / "java/AL-Game/data/scripts/system/handlers/admincommands").glob("*.java"):
            source = path.read_text()
            match = re.search(r'super\("(\w+)"\)', source)
            if match is None:
                match = re.search(r'String\s+COMMAND\s*=\s*"(\w+)"', source)
            self.assertIsNotNone(match, path.name)
            java.add(match[1])
        source = (ROOT / "go/commands/catalog.go").read_text().split("var Admin =", 1)[1]
        catalog = re.findall(r'\{"(\w+)",', source)
        self.assertEqual(len(catalog), len(set(catalog)), "duplicate website command")
        self.assertEqual(java, set(catalog))

if __name__ == "__main__":
    unittest.main()
