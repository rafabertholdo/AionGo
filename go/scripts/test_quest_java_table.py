#!/usr/bin/env python3
"""Checks quest-java-table.py against handlers with existing Go ports (needs the AL-Game tree; skipped without it)."""
from pathlib import Path
import os
import subprocess
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
JAVA = os.environ.get("AION_JAVA", str(Path(__file__).resolve().parents[2] / "java/AL-Game"))


def table(quest, *flags):
    return subprocess.run([sys.executable, f"{HERE}/quest-java-table.py", str(quest), *flags], capture_output=True, text=True, check=True,
                          env={**os.environ, "AION_JAVA": JAVA}).stdout


@unittest.skipUnless(os.path.isdir(JAVA), "AL-Game tree not available")
class QuestJavaTable(unittest.TestCase):
    def check(self, quest, *needles):
        out = table(quest)
        for needle in needles:
            self.assertIn(needle, out, f"quest {quest}")

    def test_1001_kerub_threat(self):
        self.check(1001, "starts  : ON LEVEL-UP", "levelUp", "talk                       203071",
                   "status==START > npc==203071 > dialog=1012                  -> MOVIE 15",
                   "dialog=25 > var==0           -> window 1011", "dialog=10002|33 > var==7", "REMOVE ALL of item 182200001",
                   "else status==REWARD > npc==203067                          -> defaultQuestEndDialog")

    def test_1002_lines_are_java_line_numbers(self):
        out = table(1002)
        self.assertRegex(out, r"L101\s+status==REWARD > npc==203067 > dialog==-1\s+-> window 2716")

    def test_2001_fallthrough_and_movie_break(self):
        self.check(2001, "MOVIE 51", "break -> leaves the switch", "FALLTHROUGH: the previous case (10000|10002)", "collectItemCheck")

    def test_4939_prerequisites_and_default_case(self):
        self.check(4939, "prerequisites=[4938]", "questStart(npc offers it)  204053", "npc=default", "count(186000079) >= 30")
        self.assertIn('"prerequisites": [4938]', table(4939, "--json"))

    def test_3934_many_npcs(self):
        out = table(3934)
        for npc in (798359, 798366, 203752, 203701):
            self.assertIn(str(npc), out)

    def test_xml_template_quest(self):
        out = table(1102)
        self.assertIn("XML template (poeta.xml)", out)
        self.assertIn('npc_id="210133"', out)

    def test_shell_output_is_evalable_numbers(self):
        out = table(1001, "--shell")
        self.assertIn('q_levelup="1"', out)
        self.assertIn('q_npc="203071"', out)


if __name__ == "__main__":
    unittest.main()
