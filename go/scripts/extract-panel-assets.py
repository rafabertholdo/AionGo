#!/usr/bin/env python3
"""Extracts what the panel needs to draw inventories like the 1.9 client does.

Reads the client's .pak archives (ZIPs with scrambled signatures; the first 32
bytes of each entry are XORed with one of two tables picked by compressed size)
and writes, into the output folder:

  items.json   item id -> name, icon, quality, level, type, stats, bonuses
  gear-sets.json curated gear sets and the item ids each grants
  exp.json     total experience at the start of each level (AL-Game's table)
  skills.json  class -> [skill id, skill level, min level, race] of the skill tree, without stigmas
  icons/       item icons as PNG
  skins/       every UI skin of the client's atlases, cut out as PNG
  skins.css    .s-<skin> and .p-<preset> classes drawing them (9-slices as border-image)
  font.ttf     the client's UI font

Usage: extract-panel-assets.py [client folder] [static_data folder] [output folder]
Needs Pillow (DDS decoding). The output is game content: keep it out of the repository.
"""
from pathlib import Path
import binascii, io, json, os, re, struct, sys, zlib
import xml.etree.ElementTree as ET
from PIL import Image

CLIENT = sys.argv[1] if len(sys.argv) > 1 else '/Volumes/acasis/games/aion/clients/aion1.9'
STATIC = sys.argv[2] if len(sys.argv) > 2 else str(Path(__file__).resolve().parents[2] / "java/AL-Game/data/static_data")
OUT = sys.argv[3] if len(sys.argv) > 3 else str(Path(__file__).resolve().parents[2] / ".build/panel-assets")

# The client's XOR tables (from aion-pak-manager's pak-codec.ts, as AIONencdec has them).
TABLE1 = bytes.fromhex(
    '2f5d51f701e9b4934e51813eaf3fdf99805e13839b4657b51b5cecb1297ca93168e5daa7f64fae169a7f03cf1d5ed0515ae502d911d0fbf4f87ca28826d81fa2'
    '43da33a9ac4e5a0ded78862db26ac49baa7785576aa6d835d8976b1724b77a1dd33b9e79f2ae9f01e69d2940ed2f9c16da18d1990ed40a632d92d7ebb4a75021'
    'd80f45d6c6bfcc47cc59ed3e71fea026fcd107858aee1236115a60e18fbd9ef7b66439cd495a9af7901cc1a20bb381f7cab82a4b9513dc2e4ae564169499c9b1'
    '7b5376aec4df26f7c85f7831aeaf5a7fa4e7295e0ee2bb9141322cf0ce609e27dcfadc13ac37f7f1b4a4cdf47adca97b9582da7dfb8d6b6e0c43e7236cc053f9'
    '398238de9bd0fe573d756543b0ae5a6e4eb3fbae8cc40f9b6527afa2c6f18491941a393953a59064f062ccb5bf1ebca728ae333f16c630b7b1f283b15eb03720'
    '9df77b95be356e1b070577323aae8a3925af10c51856c22bf9c44bd6dc44d79da85c7fadef88bc465ffec0e3de69e303edf8061f38c12223f4c1d7e1117b3ccb'
    'b48daf8223300d7882f9ed3e91e152a7d5d5757146da1197fb16dfeaf3aba03266db5e5eb943550e9ea52afd5e31c693d49aa22b3700b94613f70551a7b2aa22'
    '0c9dc5d23d62f4288cbc892579fa9afd8da1bc022b15b0b6e6a4cdbc72f868b49a3308ba62b7b1b1ca00080140688ee1494fd8f26785f037c961ab1ec66a4dca'
    'af032f3602f0bc5e81398a25382cca04f90df6445b46dbdeb77bf4ac3b7f360d907c2cb02048aba97f39db6d0b80e2f13750fa839dd33e8c5448ebe792346aeb'
    '2b18dadae57c7ed33dd9b1fd9028cd004593b386eb324be6ea24b697b41194a01653fbaea6d79ae9d9fba641c26dec4b0b59d76c2eec9b5d6f7666cbb023ca2c'
    '8db63a6edc29d1bd1d893febc72209b81d2e0498711a35267daaf2dbc0018a5676d127a32bc858ea7672e6f9eaa054f4b2a4c0bbec54813f58373c6945c8b7b1'
    '603b3d205b97ced2fcb1f2afa2cb6774ad5879c8fec15471ea980b59c621a0947f91defd61fc3ca171479f97890d437497ec85fe2e0de749ca550eddf438f822'
    'b17e559e56ea0f4a3a3d0f86645751f9a30c23e42a6adf2031f8dd6da8c4df427daed2ac7dd71f8567a44f97212561d0a96b7747c7974713031afac8e205d7a6'
    '0eda711842c5aad8b096532fd378ad8f2bc4913b07d79009cb55ccf7ccbdcfc53bc1341d353c598d7535f7f7b7dbd69053db66200ef798b0bd51a449b43f1de2'
    '822b043c134b39b6bda800e73360e5faf17bd59b2b4c9f81b6b9b855165f7a0507e6b33ebc8bc32f37231939d1a24cba8178a399d3b053b938442bfc8f7b0ffe'
    '99cafb373e1dd4993cddd56f48c2e18323ab7f52a989c4616fae0266e97a6767adb7807fc8a8b561c91ab357736ce9d3a0fafe4370c371462ebe2e0217ca78a0')
TABLE2 = bytes.fromhex(
    '86fa1a1c07bdd864ceee5988cda91d06f73d315883a15c7edfa6509e89a812d2254975e2070feb01974a6635ab329da74ea289620f5541c552101f47b0a063a6'
    'f01c1c4c9b3cace2b34e9ff1a4912982e4760d8d4fa3344acc1cc718488efe18790887288e24b76b38f258012da8580e9c5429cfa1ae0ad23b4a10f8d819317d'
    'f3ae1b90d22f16c7e53bccefe1e12c8600dd35678d25fced321fa91a126db0f73db61fe8814d36e72530219086300eee40be6edac13aaff2ec282cf1cd449872'
    'dacdc6d9dff7ee8804e16200080ecd1637abf9f514aa2e004ef818410bd96f9bfaad2b54562e7f2c3b6a82a17c7ca68f665ee7cf83b9eafce231d410f3f422ec'
    '73144f9478798f1e29ea5f211e0837b8f69a2dc53634c197dc75b2add7e304a7c0c91c1a00e92d6fd68dbc7352c08ab6ba2ca67d7b6ff4471a72e9b2307dd4d3'
    '099c65b0d017cffcf2ff46d2a64311762be51de5c9472f4b1bdd9afd9d20b6431a64e368f3215768d4048fc3ceafa3ab69a33c34be1f84a80e74cbb7e6b1398d'
    '68003a9b9cb1091c7d521512a6b083d340479be422e36e30c4fc6f4ffe9f51141357f1eb25f7954c92b63cd03479593320beb8bfe00ad277bc435c7dfce15900'
    'de5a7d4411ac13f264844f5da2c436d723faf8d1148df9dd171d524122f51a4239fe36d50a1001d2ea12825a48d294950af7ab70f7f29889a168f9e1d6e1bd92'
    '38455f19e2ea4676c5c3f2b49f7053093fb8063af346c86acd0ae3f0aa34d972983423d1968c32323b00a39e4fedbc97d44a2615961d0e36b8ee864557046d2b'
    'c0db910a46ce7c1f3c3a81942226826d83bd132d9691536c260c44febdeedaccbd52a6113e10422060eb5f5b0d7cbb80ac2fb9f9d24aeb5480606285e51af030'
    '45b74482ef3a0ce0e594fafd2ed9eb8d5ac2ef39517192fadbef148800ffe3f6b5343440f5bbc8d3b5bdf6cfc7b1f9183da274ef40bc6b39f2c86e0064785288'
    '13f42774148fce345ef9e06d47fc386db003ed6cf66800ac2bfe732c949e3f170c33b98f3334de0518e12bb9423f5fa2b41ee945f33843bb8eb00a9439eeff9a'
    'f42d6c4b66b11e0fc21832e174ff9094f238dd56dc789196d10403092139b2d4cc2aa8abe8991ce7e4433b58c15954e8bd9c28c681fcad334f2416a047d14c4d'
    '397ac1f71d04cfe7ae1771d937dc9c0a0e9d0e04d724c1500c49e3bcca9889558673f1c38d8f9934f74be7690ab0c12f8597bfc3fdd06275b1adf304f3f37706'
    'aa775ae7eb673fb540a19c5396fd85536eed52053b6e89ef9598b66634d08a3f44ea06861339ef20ade4732c6177103db90bc20cfdf299d8b157831b24a6a0ab'
    '973ee509073f43ed12e336ce1658f2780063f767dcd95f0daa3e9aa38372feba92e9d422f0383861e2799b5e8a6227598471c0eb95280d34cbab25c63bbc52a5'
    'ca6b93ca236d358741873e48b9df0efd30b8d1b810683dbc090431945c91af6c')


def pak(path):
    """The entries of a client .pak as name -> bytes, decoded lazily."""
    data = open(path, 'rb').read()
    entries, pos = {}, 0
    while data[pos:pos + 4] == b'\xaf\xb4\xfc\xfb':  # the scrambled local header signature
        method, = struct.unpack_from('<H', data, pos + 8)
        crc, csize, usize, nlen, xlen = struct.unpack_from('<iIIHH', data, pos + 14)
        name = data[pos + 30:pos + 30 + nlen].decode('latin1')
        start = pos + 30 + nlen + xlen
        entries[name.lower()] = (method, crc, usize, data[start:start + csize])
        pos = start + csize
    return entries


def unpack(entry):
    method, crc, usize, cdata = entry
    for table, offset in ((TABLE1, (len(cdata) & 31) * 32), (TABLE2, len(cdata) & 1023)):
        b = bytearray(cdata)
        for i in range(min(32, len(b))):
            b[i] ^= table[offset + i]
        try:
            out = zlib.decompress(bytes(b), -15) if method == 8 else bytes(b)
        except zlib.error:
            continue
        if len(out) == usize and binascii.crc32(out) == crc & 0xffffffff:
            return out
    raise ValueError('undecodable pak entry')


def bxml(buf):
    """The client's binary XML (first byte 0x80) as (name, value, attributes, children)."""
    pos = [1]

    def packed():
        value = shift = 0
        while True:
            c = buf[pos[0]]
            pos[0] += 1
            if c < 0x80:
                return value | c << shift
            value |= (c & 0x7f) << shift
            shift += 7

    size = packed()
    table = buf[pos[0]:pos[0] + size]
    pos[0] += size

    def string(i):
        if i == 0:
            return ''
        end = i * 2
        while table[end:end + 2] != b'\0\0':
            end += 2
        return table[i * 2:end].decode('utf-16le')

    def node():
        name = string(packed())
        flags = buf[pos[0]]
        pos[0] += 1
        value = string(packed()) if flags & 1 else None
        attributes = {string(packed()): string(packed()) for _ in range(packed())} if flags & 2 else {}
        children = [node() for _ in range(packed())] if flags & 4 else []
        return name, value, attributes, children

    return node()


def fields(node):
    """A record whose fields are child elements, as a dict."""
    return {child[0]: child[1] or '' for child in node[3]}


# Tooltip labels for client_items fields and bonus_attr stats.
BASE_STATS = [('physical_defend', 'Physical Def'), ('magical_resist', 'Magic Resist'), ('dodge', 'Evasion'),
              ('block', 'Block'), ('parry', 'Parry'), ('damage_reduce', 'Damage Reduction'),
              ('hit_accuracy', 'Accuracy'), ('critical', 'Crit Strike'), ('magical_skill_boost', 'Magic Boost'),
              ('magical_hit_accuracy', 'Magical Accuracy'), ('attack_range', 'Attack Range')]
BONUS_NAMES = {'maxhp': 'HP', 'maxmp': 'MP', 'magicalskillboost': 'Magic Boost', 'flyspeed': 'Flight Speed',
               'speed': 'Speed', 'concentration': 'Concentration', 'physicalattack': 'Attack',
               'phyattack': 'Attack', 'magicalhitaccuracy': 'Magical Accuracy', 'hitaccuracy': 'Accuracy',
               'physicalcritical': 'Crit Strike', 'critical': 'Crit Strike', 'parry': 'Parry', 'block': 'Block',
               'dodge': 'Evasion', 'magicalresist': 'Magic Resist', 'physicaldefend': 'Physical Def',
               'attackdelay': 'Atk Speed', 'boostcastingtime': 'Casting Speed', 'maxfp': 'Flight Time',
               'boosthate': 'Enmity Boost', 'healskillboost': 'Healing Boost', 'pvpattackratio': 'PvP Attack',
               'pvpdefendratio': 'PvP Defense', 'elementaldefendfire': 'Fire Resist',
               'elementaldefendwater': 'Water Resist', 'elementaldefendair': 'Wind Resist',
               'elementaldefendearth': 'Earth Resist', 'str': 'Power', 'vit': 'Health', 'agi': 'Agility',
               'dex': 'Accuracy', 'kno': 'Knowledge', 'wil': 'Will', 'magicalcritical': 'Crit Spell'}
ARMOR_TYPES = {'robe': 'Cloth', 'leather': 'Leather', 'chain': 'Chain', 'plate': 'Plate', 'no_armor': 'Accessory'}
WEAPON_TYPES = {'1h_sword': 'Sword', '1h_dagger': 'Dagger', '1h_mace': 'Mace', '2h_sword': 'Greatsword',
                '2h_polearm': 'Polearm', '2h_staff': 'Staff', 'bow': 'Bow', '2h_book': 'Spellbook',
                '2h_orb': 'Orb'}
SLOT_TYPES = {'neck': 'Necklace', 'right_or_left_ear': 'Earrings', 'right_or_left_finger': 'Ring',
              'waist': 'Belt', 'head': 'Headgear', 'wing': 'Wings', 'sub': 'Shield',
              'right_or_left_battery': 'Power Shard'}


def item_record(item, strings):
    record = {'n': strings.get(item.get('desc', '').lower(), item.get('name', '')),
              'i': item.get('icon_name', '').strip().lower(), 'q': item.get('quality', 'common'),
              'l': int(item.get('level', '1') or 1)}
    kind = WEAPON_TYPES.get(item.get('weapon_type')) or ARMOR_TYPES.get(item.get('armor_type')) \
        or SLOT_TYPES.get(item.get('equipment_slots'))
    if kind:
        record['t'] = kind
    stats = []
    if item.get('min_damage', '0') not in ('', '0'):
        stats.append(['Attack', f"{item['min_damage']}-{item['max_damage']}"])
    for key, label in BASE_STATS:
        value = item.get(key, '0')
        if value not in ('', '0', '0.000000') and not (key == 'attack_range'):
            stats.append([label, value])
    if stats:
        record['s'] = stats
    bonuses = []
    for n in range(1, 13):
        attr = item.get(f'bonus_attr{n}')
        if attr and ' ' in attr:
            name, value = attr.split(' ', 1)
            sign = '' if value.startswith('-') else '+'
            bonuses.append([BONUS_NAMES.get(name.lower(), name), sign + value])
    if bonuses:
        record['b'] = bonuses
    description = strings.get(item.get('desc_long', '').lower())
    # Descriptions with [.Stat...] or %0 placeholders need the client's skill data to fill in.
    if description and not re.search(r'\[\.|%\d|%\w', description):
        record['d'] = description.strip()
    if item.get('max_stack_count', '1') not in ('', '1'):
        record['m'] = int(item['max_stack_count'])
    return record


ICON_SIZE = 40  # item icons fill the top-left 40x40 of a 64x64 texture; the client draws only that

GEAR_SET_TERMS = ('miragent', 'fenris', 'anuhart', 'adma', 'theobomos', 'steel beard pirate',
                  'shulack pirate', 'shulack sailor')
SLAIN_ARCHON_SETS = {
    'slain-archon-cloth': ('Slain Archon Cloth Set', [110100940, 113100848, 112100795, 111100838, 114100873]),
    'slain-archon-leather': ('Slain Archon Leather Set', [110300888, 113300865, 112300789, 111300839, 114300898]),
    'slain-archon-chain': ('Slain Archon Chain Set', [110500855, 113500831, 112500780, 111500829, 114500841]),
    'slain-archon-plate': ('Slain Archon Plate Set', [110600840, 113600806, 112600791, 111600817, 114600800]),
}


def extract_gear_sets():
    """Writes the named 1.9 armor families offered by the character grant tool."""
    sets = []
    root = ET.parse(f'{STATIC}/item_sets/item_sets.xml').getroot()
    for item_set in root.findall('itemset'):
        name = item_set.get('name', '')
        if not any(term in name.lower() for term in GEAR_SET_TERMS):
            continue
        ids = [int(part.get('itemid')) for part in item_set.findall('itempart') if part.get('itemid')]
        if ids:
            sets.append({'k': f"set-{item_set.get('id')}", 'n': name, 'i': ids})
    for key, (name, ids) in SLAIN_ARCHON_SETS.items():
        sets.append({'k': key, 'n': name, 'i': ids})
    sets.sort(key=lambda item_set: item_set['n'].casefold())
    with open(f'{OUT}/gear-sets.json', 'w') as out:
        json.dump(sets, out, separators=(',', ':'))
    return len(sets)


# skill_tree.xml keeps legacy class names, with Priest and Cleric reversed (game/data skillTreeClass).
SKILL_TREE_CLASSES = {'FIGHTER': 'GLADIATOR', 'KNIGHT': 'TEMPLAR', 'WIZARD': 'SORCERER',
                      'ELEMENTALLIST': 'SPIRIT_MASTER', 'CLERIC': 'PRIEST', 'PRIEST': 'CLERIC'}


def extract_skills():
    """Writes the skill tree by player class, for the instance presets that teach a level's skills."""
    skills = {}
    for skill in ET.parse(f'{STATIC}/skill_tree/skill_tree.xml').getroot().iter('skill'):
        if skill.get('stigma') == 'true':
            continue
        cls = SKILL_TREE_CLASSES.get(skill.get('classId'), skill.get('classId'))
        skills.setdefault(cls, []).append([int(skill.get('skillId')), int(skill.get('skillLevel')),
                                           int(skill.get('minLevel')), skill.get('race', 'ALL')])
    with open(f'{OUT}/skills.json', 'w') as out:
        json.dump(skills, out, separators=(',', ':'))
    return sum(len(entries) for entries in skills.values())


def icon_to_png(raw, path):
    Image.open(io.BytesIO(raw)).crop((0, 0, ICON_SIZE, ICON_SIZE)).save(path, optimize=True)


def rect(value):
    return tuple(int(v) for v in value.split(','))


def extract_skins(ui_pak, preload):
    """Cuts every skin out of its atlas and writes .s-<skin> and .p-<preset> CSS classes."""
    atlases, rules, skins = {}, {}, {}  # rules: skin -> CSS declarations
    os.makedirs(f'{OUT}/skins', exist_ok=True)

    def atlas(texture):
        name = texture.lower().split('/')[-1] + '.dds'
        if name not in atlases:
            atlases[name] = Image.open(io.BytesIO(unpack(ui_pak[name]))).convert('RGBA') if name in ui_pak else None
        return atlases[name]

    def walk(node, presets):
        name, _, attributes, children = node
        if name == 'Skin' and 'texture' in attributes and attributes.get('name'):
            skins[attributes['name']] = attributes
        if name == 'Preset' and attributes.get('name'):
            refs = [c[2] for c in children if c[0] == 'SkinRef']
            # The resting look: the empty slot, the frame, the button that isn't pressed.
            for ref in refs:
                if ref.get('state') in ('slot', '0', 'frame', 'back') or \
                        (ref.get('main_state') == '0' and ref.get('sub_state') == 'up'):
                    presets[attributes['name']] = ref['name']
                    break
        for child in children:
            walk(child, presets)

    presets = {}
    walk(preload, presets)
    for name, a in skins.items():
        image = atlas(a['texture'])
        if image is None:
            continue
        if 'src_image' in a:
            x, y, w, h = rect(a['src_image'])
            image.crop((x, y, x + w, y + h)).save(f'{OUT}/skins/{name}.png', optimize=True)
            rules[name] = f'background:url(skins/{name}.png) 0 0/100% 100% no-repeat'
            continue
        pieces = {key[4:]: rect(value) for key, value in a.items()
                  if re.fullmatch(r'src_(left|middle|right)_(top|middle|bottom)', key)}
        if not pieces:
            continue
        # Lay the nine pieces out in a 3x3 image whose slice lines CSS border-image can use.
        cols = [max([p[2] for k, p in pieces.items() if k.startswith(side)] or [0]) for side in ('left', 'middle', 'right')]
        rows = [max([p[3] for k, p in pieces.items() if k.endswith(side)] or [0]) for side in ('top', 'middle', 'bottom')]
        if not cols[1] or not rows[1]:
            continue
        sheet = Image.new('RGBA', (sum(cols), sum(rows)))
        for key, (x, y, w, h) in pieces.items():
            column, row = key.split('_')
            ci, ri = ('left', 'middle', 'right').index(column), ('top', 'middle', 'bottom').index(row)
            piece = image.crop((x, y, x + w, y + h)).resize((cols[ci], rows[ri]))
            sheet.paste(piece, (sum(cols[:ci]), sum(rows[:ri])))
        sheet.save(f'{OUT}/skins/{name}.png', optimize=True)
        top, right, bottom, left = rows[0], cols[2], rows[2], cols[0]
        rules[name] = (f'border:solid transparent;border-width:{top}px {right}px {bottom}px {left}px;'
                       f'border-image:url(skins/{name}.png) {top} {right} {bottom} {left} fill stretch;box-sizing:border-box')
    css = [f'.s-{name}{{{rule}}}' for name, rule in rules.items()]
    css += [f'.p-{preset}{{{rules[skin]}}}' for preset, skin in presets.items() if skin in rules]
    open(f'{OUT}/skins.css', 'w').write('\n'.join(css) + '\n')
    return len(rules), len(presets)


def main():
    os.makedirs(f'{OUT}/icons', exist_ok=True)
    strings_pak = pak(f'{CLIENT}/L10N/1_enu/data/data.pak')
    strings = {}
    for record in bxml(unpack(strings_pak['strings/client_strings.xml']))[3]:
        f = fields(record)
        strings[f.get('name', '').lower()] = f.get('body', '')

    items_pak = pak(f'{CLIENT}/Data/Items/items.pak')
    items = {}
    for record in bxml(unpack(items_pak['client_items.xml']))[3]:
        item = fields(record)
        items[item['id']] = item_record(item, strings)
    with open(f'{OUT}/items.json', 'w') as out:
        json.dump(items, out, separators=(',', ':'))

    icons = 0
    for icon in {record['i'] for record in items.values() if record['i']}:
        entry = items_pak.get(icon + '.dds')
        if entry:
            icon_to_png(unpack(entry), f'{OUT}/icons/{icon}.png')
            icons += 1

    ui_textures = pak(f'{CLIENT}/Textures/ui/ui.pak')
    preload = bxml(unpack(pak(f'{CLIENT}/Data/ui/ui.pak')['ui_preload.xml']))
    skins, presets = extract_skins(ui_textures, preload)

    fonts = pak(f'{CLIENT}/Data/Fonts/Fonts.pak')
    open(f'{OUT}/font.ttf', 'wb').write(unpack(fonts['asiafontnhh-12px.ttf']))

    exp = [int(e.text) for e in ET.parse(f'{STATIC}/player_experience_table.xml').getroot().iter('exp')]
    json.dump(exp, open(f'{OUT}/exp.json', 'w'))
    gear_sets = extract_gear_sets()
    skills = extract_skills()
    print(f'{len(items)} items, {icons} icons, {skins} skins, {presets} presets, {len(exp)} levels, {gear_sets} gear sets, {skills} skills -> {OUT}')


if __name__ == '__main__':
    main()
