import importlib.util
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[3]
SPEC = importlib.util.spec_from_file_location("dark_poeta_evidence", ROOT / "scripts/aion/dark-poeta-evidence.py")
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class DarkPoetaEvidenceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.evidence = json.loads((ROOT / "docs/aion-dungeon-fixes/dark-poeta-evidence.json").read_text())

    def test_repeated_ai_actions_are_preserved(self):
        import xml.etree.ElementTree as ET
        source = ET.fromstring("<actions><spawn><id>1</id></spawn><spawn><id>2</id></spawn></actions>")
        self.assertEqual(MODULE.xml_record(source), {"spawn": [{"id": "1"}, {"id": "2"}]})

    def test_vine_skill_and_client_spawn_agree(self):
        vine = next(x for x in self.evidence["client_19_gatherables"] if x["id"] == "401111")
        self.assertEqual(vine["skill_level"], "300")
        marker = next(x for x in self.evidence["client_19_markers"] if x["npc"] == vine["name"])
        self.assertEqual(marker["Type"], "HSP")
        self.assertEqual(marker["Pos"], "663.97913,1169.7542,143.60455")

    def test_scar_route_covers_all_ambush_indices(self):
        ai = self.evidence["reference_46_ai"]["IDLF1_Sca_NoAction"]
        patterns = ai["on_arrived_at_waypoint"]["pattern"]
        indices = [int(x["conditions"]["is_waypoint_index"]["index"]) for x in patterns]
        self.assertEqual(indices, [7, 14, 24, 25, 32])
        path = next(x for x in self.evidence["reference_46_routes"] if x["name"] == "IDLF1_E_Path_SKA_50")
        self.assertGreater(len(path["points"]["data"]), max(indices))
        self.assertEqual(patterns[-1]["actions"]["spawn_on_target"]["npc_nameid"], "BIDLF1_NM_CrazySca_50_An")

    def test_mine_wall_requires_the_bomb_spell(self):
        ai = self.evidence["reference_46_ai"]["IDLF1_BombWall"]
        self.assertIn("Q_IDLF1_BrownieBomb", json.dumps(ai["on_spelled"]))
        self.assertIn("despawn_self", json.dumps(ai["on_see_spell"]))

    def test_patrol_bosses_resolve_to_recorded_routes(self):
        names = {x["name"] for x in self.evidence["reference_46_routes"]}
        checked = set()
        for territory in self.evidence["reference_46_territories"]:
            npcs = territory["npcs"]["npc"]
            for npc in [npcs] if isinstance(npcs, dict) else npcs:
                if "GhostElim" in npc["name"] or "_Spaller" in npc["name"]:
                    self.assertIn(npc["way_point_name"], names)
                    checked.add(npc["name"])
        self.assertEqual(len(checked), 5)

    def test_generator_housing_and_cores_are_distinct(self):
        npcs = {x["id"]: x for x in self.evidence["client_19_npcs"]}
        for npc_id in range(214895, 214898):
            self.assertEqual(npcs[str(npc_id)]["mesh"], "SpaceDynamo")
        for npc_id in range(214898, 214904):
            self.assertEqual(npcs[str(npc_id)]["mesh"], "DynamoNucleus")

    def test_vine_spawn_preserves_one_harvest_requirement(self):
        import xml.etree.ElementTree as ET
        data = ROOT / "java/AL-Game/data/static_data"
        spawns = ET.parse(data / "spawns/Gather/300040000.xml").getroot()
        vines = [s for s in spawns if s.get("npcid") == "401111"]
        self.assertEqual(len(vines), 1)
        self.assertEqual(vines[0].get("pool"), "1")
        self.assertEqual(len(vines[0].findall("object")), 1)


if __name__ == "__main__":
    unittest.main()
