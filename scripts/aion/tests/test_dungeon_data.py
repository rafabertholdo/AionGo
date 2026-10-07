from pathlib import Path
import unittest
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[3]
DATA = ROOT / "java/AL-Game/data/static_data"

class DungeonDataTests(unittest.TestCase):
    def test_asmodian_fire_temple_entrance(self):
        portals = ET.parse(DATA / "portals/portal_templates.xml").getroot()
        matches = [p for p in portals if p.get("npcid") == "730047"]
        self.assertEqual(len(matches), 1)
        p = matches[0]
        self.assertEqual(p.get("race"), "ASMODIANS")
        self.assertEqual(p.get("minlevel"), "27")
        self.assertEqual(p.find("entrypoint").get("mapid"), "220020000")
        self.assertEqual(p.find("exitpoint").get("mapid"), "320100000")

    def test_nochsana_exit_preserves_location(self):
        groups = ET.parse(DATA / "spawns/Npcs/300030000.xml").getroot()
        exits = [g for g in groups if g.get("npcid") == "700438"]
        self.assertEqual(len(exits), 1)
        self.assertFalse(any(g.get("npcid") == "700565" for g in groups))
        spot = exits[0].find("object")
        self.assertEqual(spot.get("staticid"), "14")
        self.assertEqual(spot.get("x"), "466.89075")
        self.assertEqual(spot.get("y"), "708.46313")
        self.assertEqual(spot.get("z"), "346.6602")

    def test_xml_line_endings(self):
        for name in ["portals/portal_templates.xml", "spawns/Npcs/300030000.xml", "spawns/Gather/300040000.xml"]:
            with self.subTest(name=name):
                self.assertNotIn(b"\n", (DATA / name).read_bytes().replace(b"\r\n", b""))

    def test_public_evidence_uses_logical_source_names(self):
        import json
        for name in ["dark-poeta-evidence.json", "client-evidence.json"]:
            with self.subTest(name=name):
                text = (ROOT / "docs/aion-dungeon-fixes" / name).read_text()
                self.assertNotIn("/Volumes/", text)
                self.assertNotIn("/Users/", text)
                self.assertIsInstance(json.loads(text), dict)
