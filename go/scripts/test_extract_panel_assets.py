#!/usr/bin/env python3
"""Tests the gear-set manifest written for character admin grants."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).with_name('extract-panel-assets.py')
SPEC = importlib.util.spec_from_file_location('extract_panel_assets', SCRIPT)
ASSETS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ASSETS)


class ExtractGearSets(unittest.TestCase):
    def test_curated_sets_and_slain_archon_variants(self):
        with tempfile.TemporaryDirectory() as directory:
            static = Path(directory) / 'static'
            output = Path(directory) / 'output'
            (static / 'item_sets').mkdir(parents=True)
            output.mkdir()
            (static / 'item_sets' / 'item_sets.xml').write_text(
                '<item_sets>'
                '<itemset id="14" name="Miragent Cloth Set"><itempart itemid="110100917"/></itemset>'
                '<itemset id="81" name="Steel Beard Pirate Plate Set"><itempart itemid="110600840"/></itemset>'
                '<itemset id="129" name="Unrelated Set"><itempart itemid="100000001"/></itemset>'
                '</item_sets>',
                encoding='utf-8',
            )
            ASSETS.STATIC = str(static)
            ASSETS.OUT = str(output)

            self.assertEqual(6, ASSETS.extract_gear_sets())
            sets = json.loads((output / 'gear-sets.json').read_text(encoding='utf-8'))
            by_key = {item_set['k']: item_set for item_set in sets}
            self.assertEqual([110100917], by_key['set-14']['i'])
            self.assertEqual([110600840], by_key['set-81']['i'])
            self.assertEqual(5, len(by_key['slain-archon-cloth']['i']))
            self.assertEqual(5, len(by_key['slain-archon-leather']['i']))
            self.assertEqual(5, len(by_key['slain-archon-chain']['i']))
            self.assertEqual(5, len(by_key['slain-archon-plate']['i']))
            self.assertNotIn('set-129', by_key)


class ExtractSkills(unittest.TestCase):
    def test_classes_renamed_and_stigmas_dropped(self):
        with tempfile.TemporaryDirectory() as directory:
            static = Path(directory) / 'static'
            output = Path(directory) / 'output'
            (static / 'skill_tree').mkdir(parents=True)
            output.mkdir()
            (static / 'skill_tree' / 'skill_tree.xml').write_text(
                '<skill_tree>'
                '<skill skillId="1" skillLevel="2" minLevel="3" race="ALL" classId="PRIEST" autolearn="true"/>'
                '<skill skillId="4" skillLevel="1" minLevel="9" race="ELYOS" classId="CLERIC"/>'
                '<skill skillId="5" skillLevel="1" minLevel="20" race="ALL" classId="FIGHTER" stigma="true"/>'
                '</skill_tree>',
                encoding='utf-8',
            )
            ASSETS.STATIC = str(static)
            ASSETS.OUT = str(output)

            self.assertEqual(2, ASSETS.extract_skills())
            skills = json.loads((output / 'skills.json').read_text(encoding='utf-8'))
            self.assertEqual({'CLERIC': [[1, 2, 3, 'ALL']], 'PRIEST': [[4, 1, 9, 'ELYOS']]}, skills)


if __name__ == '__main__':
    unittest.main()
