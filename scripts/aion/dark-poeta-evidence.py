#!/usr/bin/env python3
"""Record local 1.9 identities and 4.6 behavior leads; never alter server data."""
import argparse
import ast
import hashlib
import json
from pathlib import Path
import xml.etree.ElementTree as ET


def load_client_reader(source):
    tree = ast.parse(source.read_text())
    names = {"TABLE1", "TABLE2", "pak", "unpack", "bxml", "fields"}
    nodes = [node for node in tree.body if
             isinstance(node, ast.FunctionDef) and node.name in names or
             isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and
             target.id in names for target in node.targets)]
    context = {}
    exec("import struct,zlib,binascii", context)
    exec(compile(ast.Module(body=nodes, type_ignores=[]), str(source), "exec"), context)
    return context


def xml_record(element):
    if not len(element):
        return (element.text or "").strip()
    record = {}
    for child in element:
        record.setdefault(child.tag, []).append(xml_record(child))
    return {key: values[0] if len(values) == 1 else values for key, values in record.items()}


def extract(client, reference, reader_source):
    reader = load_client_reader(reader_source)
    archives = ["Data/npcs/npcs.pak", "Data/Gather/Gather.pak", "Levels/idlf1/level.pak"]
    sources = {name: hashlib.sha256((client / name).read_bytes()).hexdigest()
               for name in archives}
    sources["go/scripts/extract-panel-assets.py"] = hashlib.sha256(reader_source.read_bytes()).hexdigest()
    def records(archive, entry):
        raw = reader["unpack"](reader["pak"](client / archive)[entry])
        return [reader["fields"](node) for node in reader["bxml"](raw)[3]]
    ids = {214871, 214864, 215387, 214880, 215388, 215389, 730185, 730186, 700516,
           700517, 700556, 700558, *range(700439, 700448), *range(214895, 214904)}
    keys = {"id", "name", "desc", "dir", "mesh", "ai_name", "quest_ai_name", "npc_type",
            "cursor_type", "talking_distance", "scale", "altitude"}
    npcs = [{key: value for key, value in record.items() if key in keys}
            for record in records(archives[0], "client_npcs.xml") if int(record["id"]) in ids]
    gather = [record for record in records(archives[1], "gather_src.xml")
              if record["id"] in {"401111", "401112"}]
    mission = ET.fromstring(reader["unpack"](reader["pak"](client / archives[2])["mission_mission0.xml"]))
    names = {record["name"] for record in npcs + gather}
    markers = [dict(element.attrib) for element in mission.iter("Object")
               if element.get("npc") in names]
    ai_path = reference / "XML/NpcAIPatterns.xml"
    world_path = reference / "Worlds/idlf1/world.xml"
    template_path = reference / "XML/npcs.xml"
    for path in [ai_path, world_path, template_path]:
        sources[str(path.relative_to(reference))] = hashlib.sha256(path.read_bytes()).hexdigest()
    ai_names = {record.get("ai_name") for record in npcs} | {"IDLF1_Sca"}
    patterns = {element.findtext("name"): xml_record(element.find("event_handlers"))
                for element in ET.parse(ai_path).getroot() if element.findtext("name") in ai_names}
    world = ET.parse(world_path).getroot()
    territories = [xml_record(element) for element in world.find("npc_spawn")
                   if any(npc.findtext("name") in names for npc in element.findall("npcs/npc"))]
    paths = {npc.get("way_point_name") for territory in territories
             for npc in ([territory["npcs"]["npc"]] if isinstance(territory["npcs"]["npc"], dict)
                         else territory["npcs"]["npc"])}
    # Keep AI-spawned core routes as well as routes attached to static bosses.
    routes = [xml_record(element) for element in world.find("way_point_list")
              if element.findtext("name") in paths or element.findtext("name", "").startswith("NPCPath_GCore")]
    template_keys = {"id", "name", "max_hp", "hp_regen", "physical_defend", "physical_damage_trim",
                     "magical_damage_trim", "skills", "ai_name", "idle_name"}
    templates = [{child.tag: xml_record(child) for child in element if child.tag in template_keys}
                 for element in ET.parse(template_path).getroot() if element.findtext("name") in names]
    return {"recorded": "2026-10-05", "status": "evidence; no gameplay validation",
            "caution": "4.6 routes, stats and AI are leads, not verified 1.9 constants. Editor marker Z requires ground validation.",
            "sources": sources, "client_19_npcs": npcs, "client_19_gatherables": gather,
            "client_19_markers": markers, "reference_46_ai": patterns,
            "reference_46_territories": territories, "reference_46_routes": routes,
            "reference_46_templates": templates}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client", type=Path, required=True)
    parser.add_argument("--reference", type=Path, required=True)
    parser.add_argument("--reader", type=Path, default=Path(__file__).resolve().parents[2] / "go/scripts/extract-panel-assets.py")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    result = extract(args.client, args.reference, args.reader)
    args.output.write_text(json.dumps(result, indent=2) + "\n")
    print(f"Recorded {len(result['client_19_npcs'])} NPCs, {len(result['reference_46_ai'])} AI patterns and {len(result['reference_46_routes'])} route leads.")


if __name__ == "__main__":
    main()
