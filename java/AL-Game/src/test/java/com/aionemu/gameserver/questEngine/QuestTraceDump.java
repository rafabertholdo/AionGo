package com.aionemu.gameserver.questEngine;

import java.io.File;
import java.io.PrintWriter;
import java.lang.reflect.Field;
import java.lang.reflect.Modifier;
import java.nio.channels.SelectionKey;
import java.nio.channels.SelectableChannel;
import java.nio.channels.Selector;
import java.nio.file.Files;
import java.nio.file.Path;
import java.lang.reflect.Method;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;

import gnu.trove.TIntObjectHashMap;

import org.junit.Assume;
import org.junit.Test;

import com.aionemu.gameserver.controllers.ActionitemController;
import com.aionemu.gameserver.controllers.MonsterController;
import com.aionemu.gameserver.controllers.NpcController;
import com.aionemu.gameserver.controllers.PortalController;
import com.aionemu.gameserver.controllers.PostboxController;
import com.aionemu.gameserver.model.NpcType;
import com.aionemu.gameserver.model.gameobjects.Monster;
import com.aionemu.gameserver.model.templates.NpcTemplate;
import com.aionemu.gameserver.controllers.PlayerController;
import com.aionemu.gameserver.dataholders.DataManager;
import com.aionemu.gameserver.model.Gender;
import com.aionemu.gameserver.model.PlayerClass;
import com.aionemu.gameserver.model.Race;
import com.aionemu.gameserver.model.gameobjects.Item;
import com.aionemu.gameserver.model.gameobjects.Npc;
import com.aionemu.gameserver.model.gameobjects.player.Equipment;
import com.aionemu.gameserver.model.gameobjects.player.Player;
import com.aionemu.gameserver.model.gameobjects.player.PlayerAppearance;
import com.aionemu.gameserver.model.gameobjects.player.PlayerCommonData;
import com.aionemu.gameserver.model.gameobjects.player.Storage;
import com.aionemu.gameserver.model.gameobjects.player.StorageType;
import com.aionemu.gameserver.model.templates.QuestTemplate;
import com.aionemu.gameserver.model.gameobjects.player.SkillList;
import com.aionemu.gameserver.model.gameobjects.stats.PlayerGameStats;
import com.aionemu.gameserver.model.gameobjects.stats.PlayerLifeStats;
import com.aionemu.gameserver.model.templates.quest.NpcQuestData;
import com.aionemu.gameserver.model.templates.spawn.SpawnTemplate;
import com.aionemu.gameserver.network.aion.serverpackets.SM_DIALOG_WINDOW;
import com.aionemu.gameserver.services.ItemService;
import com.aionemu.gameserver.world.KnownList;
import com.aionemu.gameserver.network.aion.AionConnection;
import com.aionemu.gameserver.network.aion.AionServerPacket;
import com.aionemu.gameserver.questEngine.handlers.QuestHandler;
import com.aionemu.gameserver.questEngine.model.QuestEnv;
import com.aionemu.gameserver.questEngine.model.QuestState;
import com.aionemu.gameserver.questEngine.model.QuestStatus;
import com.aionemu.gameserver.world.World;

import javolution.util.FastList;

/**
 * Runs every Java quest handler's onDialogEvent against a sweep of quest states, npcs and dialog ids, without a
 * database or a client, and writes what each probe sent (dialog windows, quest updates, movies, other packets), the
 * quest state after it and the inventory change. The Go test TestQuestJavaParity replays the same probes against the
 * Go handlers. Run it with go/scripts/quest-parity.sh.
 */
public class QuestTraceDump
{
	private static final sun.misc.Unsafe U = unsafe();

	@Test
	public void dump() throws Exception
	{
		String out = System.getProperty("questTrace");
		Assume.assumeTrue("set -DquestTrace=<file> to dump the Java quest traces", out != null && !out.isEmpty());
		loadConfig();
		DataManager.getInstance();
		World.getInstance();
		QuestEngine qe = QuestEngine.getInstance();
		// QuestEngine.load compiles the scripts at runtime; Maven already compiled them, so register the classes.
		for (Class<?> c : handlerClasses())
			qe.addQuestHandler((QuestHandler) c.getDeclaredConstructor().newInstance());
		for (var d : DataManager.QUEST_SCRIPTS_DATA.getData())
			d.register(qe);
		String only = System.getProperty("questIds", "");
		Map<Integer, QuestHandler> handlers = get(qe, QuestEngine.class, "questHandlers");
		// Handlers that keep int fields share them between players; every probe starts from their initial values.
		for (QuestHandler h : handlers.values())
			for (Field f : h.getClass().getDeclaredFields())
				if (f.getType() == int.class && !Modifier.isStatic(f.getModifiers()) && !Modifier.isFinal(f.getModifiers()))
				{
					f.setAccessible(true);
					FIELDS.add(new Object[] { h, f, f.getInt(h) });
				}
		List<Integer> ids = new ArrayList<>(handlers.keySet());
		Collections.sort(ids);
		try (PrintWriter w = new PrintWriter(out, "UTF-8"))
		{
			for (int id : ids)
				if (only.isEmpty() || Arrays.asList(only.split(",")).contains(Integer.toString(id)))
					new QuestTrace(qe, id).run(w);
		}
	}

	/** Every dialog id the Go conformance sweep tries, plus the plain click (-1). */
	static final int[] SWEEP = sweep();

	/** Dialog ids NpcController.onDialogSelect serves itself when no quest handler answers (never echoed). */
	static final Set<Integer> SERVICE = new HashSet<>(Arrays.asList(2, 3, 4, 5, 6, 7, 20, 27, 29, 30, 31, 35, 36, 37,
		38, 39, 40, 41, 42, 47, 50, 52, 53, 60, 61));

	static final List<String> SEEDS = Arrays.asList("REWARD/0", "START/0", "START/1", "START/2", "START/3", "START/4",
		"START/5", "START/6", "START/7");

	static int[] sweep()
	{
		List<Integer> l = new ArrayList<>();
		l.add(-1);
		int[][] ranges = { { 1, 60 }, { 1000, 1030 }, { 1350, 1360 }, { 1690, 1700 }, { 2030, 2040 }, { 2370, 2380 },
			{ 2710, 2720 }, { 3050, 3060 }, { 3390, 3400 }, { 10000, 10005 }, { 10255, 10255 } };
		for (int[] r : ranges)
			for (int d = r[0]; d <= r[1]; d++)
				l.add(d);
		return l.stream().mapToInt(Integer::intValue).toArray();
	}

	static final class QuestTrace
	{
		final QuestEngine	qe;
		final int			id;
		final QuestTemplate	template;
		final List<Integer>	npcs	= new ArrayList<>();
		Race				race	= Race.ELYOS;
		PlayerClass			cls;
		Gender				gender	= Gender.FEMALE;
		int					level;

		QuestTrace(QuestEngine qe, int id) throws Exception
		{
			this.qe = qe;
			this.id = id;
			this.template = DataManager.QUEST_DATA.getQuestById(id);
			TIntObjectHashMap<NpcQuestData> byNpc = get(qe, QuestEngine.class, "npcQuestData");
			for (int npc : byNpc.keys())
				if ((byNpc.get(npc).getOnTalkEvent().contains(id) || byNpc.get(npc).getOnQuestStart().contains(id))
					&& DataManager.NPC_DATA.getNpcTemplate(npc) != null) // an npc missing from the data never spawns
					npcs.add(npc);
			Collections.sort(npcs);
			level = 1;
			if (template != null)
			{
				if (template.getRacePermitted() != null)
					race = template.getRacePermitted();
				if (template.getMinlevelPermitted() != null)
					level = Math.max(1, template.getMinlevelPermitted());
				if (template.getGenderPermitted() != null)
					gender = template.getGenderPermitted();
				for (PlayerClass permitted : template.getClassPermitted())
					if (cls == null || cls.isStartingClass() && level >= 10)
						cls = permitted;
			}
			// A starting class cannot pass level 9 (PlayerCommonData.setExp).
			if (cls == null)
				cls = level >= 10 ? PlayerClass.SORCERER : PlayerClass.MAGE;
		}

		void run(PrintWriter w) throws Exception
		{
			// First the states the dialogs reach from no quest at all, then seeded ones (reached in game by kills,
			// items, timers...) and whatever the dialogs reach from those.
			List<String> states = new ArrayList<>(Arrays.asList("none"));
			int reached = -1;
			StringBuilder probes = new StringBuilder();
			for (int i = 0; i <= states.size(); i++)
			{
				if (i == states.size())
				{
					if (reached >= 0)
						break;
					reached = i;
					for (String seed : SEEDS)
						if (!states.contains(seed))
							states.add(seed);
					if (i == states.size())
						break;
				}
				for (int npc : npcs)
					for (int dialog : SWEEP)
					{
						Probe pr = probe(states.get(i), npc, dialog);
						if (pr == null)
							continue;
						if (!pr.after.equals(pr.state) && !states.contains(pr.after) && states.size() < 40
							&& (pr.after.startsWith("START/") || pr.after.startsWith("REWARD/")))
							states.add(pr.after);
						if (!pr.isDefault())
							probes.append(pr.json()).append('\n');
					}
			}
			w.print("{\"quest\":" + id + ",\"race\":\"" + race + "\",\"class\":\"" + cls + "\",\"gender\":\"" + gender
				+ "\",\"level\":" + level + ",\"npcs\":" + npcs + ",\"states\":" + quoted(states) + ",\"reached\":" + reached + ",\"sweep\":" + Arrays.toString(SWEEP) + "}\n");
			w.print(probes);
			w.flush();
		}

		/** One select (or click when dialog is -1) on a fresh player in state; null for a service id nobody handled. */
		Probe probe(String state, int npcId, int dialog) throws Exception
		{
			resetFields();
			Probe pr = new Probe(id, state, npcId, dialog);
			AionConnection con = fakeConnection();
			Player p = player(state, con);
			Npc npc = npc(npcId, p);
			// The client targets the npc before talking to it (CM_SHOW_DIALOG -> setTarget).
			p.setTarget(npc);
			Map<Integer, Long> before = inventory(p);
			long exp = p.getCommonData().getExp();
			try
			{
				boolean handled;
				if (dialog == -1)
				{
					// CM_SHOW_DIALOG: the npc's own controller (menu for a plain npc, use animation for an action item).
					npc.getController().onDialogRequest(p);
				}
				else
				{
					handled = qe.onDialog(new QuestEnv(npc, p, id, dialog));
					if (!handled && SERVICE.contains(dialog))
						return null;
					if (!handled)
						con.sendPacket(new SM_DIALOG_WINDOW(NPC_OBJ, dialog, id));
				}
			}
			catch (Throwable t)
			{
				StackTraceElement at = t.getStackTrace().length > 0 ? t.getStackTrace()[0] : null;
				pr.err = t + (at == null ? "" : " @ " + at.getClassName() + ":" + at.getLineNumber());
			}
			finally
			{
				World.getInstance().removeObject(p);
			}
			for (AionServerPacket pk : queue(con))
				pr.out.add(encode(pk));
			QuestState qs = p.getQuestStateList().getQuestState(id);
			pr.after = qs == null ? "none" : qs.getStatus() + "/" + qs.getQuestVars().getQuestVars();
			Map<Integer, Long> after = inventory(p);
			for (int item : union(before.keySet(), after.keySet()))
			{
				long d = after.getOrDefault(item, 0L) - before.getOrDefault(item, 0L);
				if (d != 0)
					pr.items.put(item, d);
			}
			pr.exp = p.getCommonData().getExp() - exp;
			return pr;
		}

		/** The npc with the controller SpawnEngine.spawnObject gives its type, next to the player. */
		Npc npc(int npcId, Player p)
		{
			NpcTemplate t = DataManager.NPC_DATA.getNpcTemplate(npcId);
			SpawnTemplate spawn = new SpawnTemplate(p.getX(), p.getY(), p.getZ(), (byte) 0, 0, 0, 0);
			Npc npc;
			switch (t == null || t.getNpcType() == null ? NpcType.NON_ATTACKABLE : t.getNpcType())
			{
				case AGGRESSIVE:
				case ATTACKABLE:
					npc = new Monster(NPC_OBJ, new MonsterController(), spawn, t);
					break;
				case POSTBOX:
					npc = new Npc(NPC_OBJ, new PostboxController(), spawn, t);
					break;
				case USEITEM:
					npc = new Npc(NPC_OBJ, new ActionitemController(), spawn, t);
					break;
				case PORTAL:
					npc = new Npc(NPC_OBJ, new PortalController(), spawn, t);
					break;
				default:
					npc = new Npc(NPC_OBJ, new NpcController(), spawn, t);
			}
			npc.setKnownlist(new KnownList(npc));
			return npc;
		}

		Player player(String state, AionConnection con) throws Exception
		{
			PlayerCommonData pcd = new PlayerCommonData(PLAYER_OBJ);
			pcd.setName("Wrathchild");
			pcd.setPlayerClass(cls);
			pcd.setRace(race);
			pcd.setGender(gender);
			pcd.setPosition(World.getInstance().createPosition(210010000, 1142.48f, 1032.39f, 129.06f, (byte) 0));
			pcd.setExp(DataManager.PLAYER_EXPERIENCE_TABLE.getStartExpForLevel(level));
			Player p = new Player(new PlayerController(), pcd, new PlayerAppearance());
			p.setStorage(new Storage(StorageType.CUBE), StorageType.CUBE);
			p.setStorage(new Storage(StorageType.REGULAR_WAREHOUSE), StorageType.REGULAR_WAREHOUSE);
			p.getInventory().setLimit(27);
			Item kinah = ItemService.newItem(182400001, 0);
			p.getInventory().setKinahItem(kinah);
			p.setEquipment(new Equipment(p));
			p.setSkillList(new SkillList());
			p.setKnownlist(new KnownList(p));
			p.setPlayerStatsTemplate(DataManager.PLAYER_STATS_DATA.getTemplate(p));
			p.setGameStats(new PlayerGameStats(DataManager.PLAYER_STATS_DATA, p));
			p.setLifeStats(new PlayerLifeStats(p));
			p.setEffectController(new com.aionemu.gameserver.controllers.effect.PlayerEffectController(p));
			p.setAbyssRank(new com.aionemu.gameserver.model.gameobjects.player.AbyssRank(0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0));
			p.setClientConnection(con);
			if (template != null)
				for (int done : template.getFinishedQuestConds())
					p.getQuestStateList().addQuest(done, new QuestState(done, QuestStatus.COMPLETE, 0, 1));
			if (!state.equals("none"))
			{
				String[] sv = state.split("/");
				p.getQuestStateList().addQuest(id, new QuestState(id, QuestStatus.valueOf(sv[0]), Integer.parseInt(sv[1]), 0));
			}
			pcd.setOnline(true);
			World.getInstance().storeObject(p);
			return p;
		}
	}

	static final List<Object[]>	FIELDS		= new ArrayList<>();

	static void resetFields() throws Exception
	{
		for (Object[] x : FIELDS)
			((Field) x[1]).setInt(x[0], (Integer) x[2]);
	}

	static final int	PLAYER_OBJ	= 0x10577;
	static final int	NPC_OBJ		= 0x40000;

	static final class Probe
	{
		final int				quest, npc, dialog;
		final String			state;
		String					after, err;
		long					exp;
		final List<String>		out		= new ArrayList<>();
		final Map<Integer, Long>	items	= new TreeMap<>();

		Probe(int quest, String state, int npc, int dialog)
		{
			this.quest = quest;
			this.state = state;
			this.npc = npc;
			this.dialog = dialog;
		}

		/** The echo of a select with nothing else (clicks depend on the npc's controller and are always kept). */
		boolean isDefault()
		{
			return dialog != -1 && err == null && after.equals(state) && items.isEmpty() && exp == 0 && out.size() == 1
				&& out.get(0).equals(win(dialog, quest));
		}

		String json()
		{
			return "{\"npc\":" + npc + ",\"state\":\"" + state + "\",\"dialog\":" + dialog + ",\"after\":\"" + after
				+ "\",\"out\":" + quoted(out) + ",\"items\":" + items.toString().replace('=', ':').replaceAll("(\\d+):", "\"$1\":")
				+ ",\"exp\":" + exp + (err == null ? "" : ",\"err\":" + quote(err)) + "}";
		}
	}

	static String win(int dialog, int quest)
	{
		SM_DIALOG_WINDOW w = new SM_DIALOG_WINDOW(NPC_OBJ, dialog, quest);
		return encode(w);
	}

	/** "<opcode hex> <name> <payload hex>" with the packet's body as the 1.9 client gets it. */
	static String encode(AionServerPacket pk)
	{
		ByteBuffer buf = ByteBuffer.allocate(0x10000).order(ByteOrder.LITTLE_ENDIAN);
		String hex;
		try
		{
			Method m = AionServerPacket.class.getDeclaredMethod("writeImpl", AionConnection.class, ByteBuffer.class);
			m.setAccessible(true);
			m.invoke(pk, null, buf);
			StringBuilder sb = new StringBuilder();
			for (int i = 0; i < buf.position(); i++)
				sb.append(String.format("%02x", buf.get(i)));
			hex = sb.toString();
		}
		catch (Exception e)
		{
			hex = "!" + (e.getCause() != null ? e.getCause() : e);
		}
		return String.format("%02x %s %s", pk.getOpcode(), pk.getClass().getSimpleName(), hex);
	}

	static Map<Integer, Long> inventory(Player p)
	{
		Map<Integer, Long> m = new HashMap<>();
		for (Item it : p.getInventory().getAllItems())
			m.merge(it.getItemTemplate().getTemplateId(), it.getItemCount(), Long::sum);
		return m;
	}

	static Set<Integer> union(Set<Integer> a, Set<Integer> b)
	{
		Set<Integer> s = new TreeSet<>(a);
		s.addAll(b);
		return s;
	}

	static String quoted(List<String> l)
	{
		StringBuilder sb = new StringBuilder("[");
		for (String s : l)
			sb.append(sb.length() > 1 ? "," : "").append(quote(s));
		return sb.append(']').toString();
	}

	static String quote(String s)
	{
		return "\"" + s.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", " ") + "\"";
	}

	// ---- fixture

	/** Config.load without the network/server settings, whose placeholders are filled only in the container. */
	static void loadConfig() throws Exception
	{
		java.util.Properties[] main = com.aionemu.commons.utils.PropertiesUtils.loadAllFromDirectory("./config/main");
		for (Class<?> c : new Class<?>[] { com.aionemu.gameserver.configs.main.RateConfig.class,
			com.aionemu.gameserver.configs.main.CustomConfig.class, com.aionemu.gameserver.configs.main.OptionsConfig.class,
			com.aionemu.gameserver.configs.main.GroupConfig.class, com.aionemu.gameserver.configs.main.PricesConfig.class,
			com.aionemu.gameserver.configs.main.NpcMovementConfig.class, com.aionemu.gameserver.configs.main.ThreadConfig.class })
			com.aionemu.commons.configuration.ConfigurableProcessor.process(c, main);
		com.aionemu.gameserver.configs.main.ThreadConfig.load();
	}

	static List<Class<?>> handlerClasses() throws Exception
	{
		List<Class<?>> out = new ArrayList<>();
		Path root = new File("target/classes").toPath();
		try (var files = Files.walk(root.resolve("quest")))
		{
			for (Path p : (Iterable<Path>) files::iterator)
			{
				String name = root.relativize(p).toString();
				if (!name.endsWith(".class") || name.contains("$"))
					continue;
				Class<?> c = Class.forName(name.substring(0, name.length() - 6).replace('/', '.'));
				if (QuestHandler.class.isAssignableFrom(c) && !Modifier.isAbstract(c.getModifiers()))
					out.add(c);
			}
		}
		return out;
	}

	static FastList<AionServerPacket> queue(AionConnection con) throws Exception
	{
		return get(con, AionConnection.class, "sendMsgQueue");
	}

	/** A connection that only queues: no socket, and a selection key that is never valid. */
	static AionConnection fakeConnection() throws Exception
	{
		AionConnection con = (AionConnection) U.allocateInstance(AionConnection.class);
		Class<?> ac = AionConnection.class.getSuperclass();
		set(con, ac, "guard", new Object());
		set(con, ac, "key", new DeadKey());
		set(con, AionConnection.class, "sendMsgQueue", new FastList<AionServerPacket>());
		return con;
	}

	static final class DeadKey extends SelectionKey
	{
		public SelectableChannel channel() { return null; }
		public Selector selector() { return null; }
		public boolean isValid() { return false; }
		public void cancel() {}
		public int interestOps() { return 0; }
		public SelectionKey interestOps(int ops) { return this; }
		public int readyOps() { return 0; }
	}

	// ---- reflection

	static void set(Object o, Class<?> c, String name, Object v) throws Exception
	{
		Field f = c.getDeclaredField(name);
		U.putObject(o, U.objectFieldOffset(f), v);
	}

	@SuppressWarnings("unchecked")
	static <T> T get(Object o, Class<?> c, String name) throws Exception
	{
		Field f = c.getDeclaredField(name);
		f.setAccessible(true);
		return (T) f.get(o);
	}

	static sun.misc.Unsafe unsafe()
	{
		try
		{
			Field f = sun.misc.Unsafe.class.getDeclaredField("theUnsafe");
			f.setAccessible(true);
			return (sun.misc.Unsafe) f.get(null);
		}
		catch (Exception e)
		{
			throw new RuntimeException(e);
		}
	}
}
