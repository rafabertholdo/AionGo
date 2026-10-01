/*
 * This file is part of aion-unique <aion-unique.org>.
 *
 * aion-unique is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * aion-unique is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with aion-unique.  If not, see <http://www.gnu.org/licenses/>.
 */
package quest.brusthonin;

import com.aionemu.gameserver.model.gameobjects.Npc;
import com.aionemu.gameserver.model.gameobjects.player.Player;
import com.aionemu.gameserver.network.aion.serverpackets.SM_DIALOG_WINDOW;
import com.aionemu.gameserver.questEngine.handlers.QuestHandler;
import com.aionemu.gameserver.questEngine.model.QuestEnv;
import com.aionemu.gameserver.questEngine.model.QuestState;
import com.aionemu.gameserver.questEngine.model.QuestStatus;
import com.aionemu.gameserver.services.QuestService;
import com.aionemu.gameserver.utils.PacketSendUtility;
import com.aionemu.gameserver.world.zone.ZoneName;

/**
 * @author Dune11
 * 
 */
public class _2092GravesoftheRedSkyLegion extends QuestHandler
{
	
	private final static int	questId	= 2092;
	private final static int[]	npc_ids	= { 205150, 700394, 205188, 205190, 730163, 730162, 730161, 730160, 730159, 730158, 730156, 205208, 205209, 205210, 205211, 205212, 205213, 205214 };

	public _2092GravesoftheRedSkyLegion()
	{
		super(questId);
	}

	@Override
	public void register()
	{
		qe.setQuestEnterZone(ZoneName.Q2092).add(questId);
		qe.addQuestLvlUp(questId);	
		qe.setNpcQuestData(214402).addOnKillEvent(questId);	
		for(int npc_id : npc_ids)
			qe.setNpcQuestData(npc_id).addOnTalkEvent(questId);
		deletebleItems = new int[]{182209008, 152000857, 152010317};	
	}

	@Override
	public boolean onLvlUpEvent(QuestEnv env)
	{
		final Player player = env.getPlayer();
		final QuestState qs = player.getQuestStateList().getQuestState(questId);
		boolean lvlCheck = QuestService.checkLevelRequirement(questId, player.getCommonData().getLevel());
		if(qs == null || qs.getStatus() != QuestStatus.LOCKED || !lvlCheck)
			return false;

		QuestState qs2 = player.getQuestStateList().getQuestState(2091);
		if(qs2 == null || qs2.getStatus() != QuestStatus.COMPLETE)
			return false;
		qs.setStatus(QuestStatus.START);
		updateQuestStatus(player, qs);
		return true;
	}

	@Override
	public boolean onDialogEvent(QuestEnv env)
	{
		final Player player = env.getPlayer();
		final QuestState qs = player.getQuestStateList().getQuestState(questId);
		final Npc npc = (Npc)env.getVisibleObject();
		
		if(qs == null)
			return false;

		int var = qs.getQuestVarById(0);
		int targetId = 0;
		if(env.getVisibleObject() instanceof Npc)
			targetId = ((Npc) env.getVisibleObject()).getNpcId();

		if(qs.getStatus() == QuestStatus.REWARD)
		{
			if(targetId == 205150)
				return defaultQuestEndDialog(env);
			return false;
		}
		else if(qs.getStatus() != QuestStatus.START)
		{
			return false;
		}
		if(targetId == 205150)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 0)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 1011);	
					return true;
				case 10000:
					if (var == 0)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}			
			}			
		}
		else if(targetId == 205188)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 2)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 1693);
				case 10002:
					if(var == 2)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}						
			}
		}
		else if(targetId == 205190)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 3)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2034);								
					else if(var==4)	
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2375);
					return true;
				case 10003:
					if (var == 3)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}							
				case 33:
					if (var==4)
					{
						if(QuestService.collectItemCheck(env, true))
						{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 10000);
						}
						else return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 10001);
					}		
			}			
		}
		else if(targetId == 730163)
		{
			switch(env.getDialogId())
			{
				case -1:
					if(var == 5)
					{
						QuestService.addNewSpawn(220050000, 1, 205214, (float) npc.getX(),
										(float) npc.getY(), (float) npc.getZ(), (byte) 0, true);
						npc.getController().onDespawn(true);
						npc.getController().scheduleRespawn();	
						return true;
					}						
			}
		}
		else if(targetId == 205214)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 5)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2717);
				case 10005:
					if(var == 5)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}						
			}
		}
		else if(targetId == 730162)
		{
			switch(env.getDialogId())
			{
				case -1:
					if(var == 5)
					{
						QuestService.addNewSpawn(220050000, 1, 205213, (float) npc.getX(),
										(float) npc.getY(), (float) npc.getZ(), (byte) 0, true);
						npc.getController().onDespawn(true);
						npc.getController().scheduleRespawn();	
						return true;
					}						
			}
		}
		else if(targetId == 205213)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 5)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2717);
				case 10005:
					if(var == 5)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}						
			}
		}
		else if(targetId == 730161)
		{
			switch(env.getDialogId())
			{
				case -1:
					if(var == 5)
					{
						QuestService.addNewSpawn(220050000, 1, 205212, (float) npc.getX(),
										(float) npc.getY(), (float) npc.getZ(), (byte) 0, true);
						npc.getController().onDespawn(true);
						npc.getController().scheduleRespawn();	
						return true;
					}						
			}
		}
		else if(targetId == 205212)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 5)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2717);
				case 10005:
					if(var == 5)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}						
			}
		}
		else if(targetId == 730160)
		{
			switch(env.getDialogId())
			{
				case -1:
					if(var == 5)
					{
						QuestService.addNewSpawn(220050000, 1, 205211, (float) npc.getX(),
										(float) npc.getY(), (float) npc.getZ(), (byte) 0, true);
						npc.getController().onDespawn(true);
						npc.getController().scheduleRespawn();	
						return true;
					}						
			}
		}
		else if(targetId == 205211)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 5)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2717);
				case 10005:
					if(var == 5)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}						
			}
		}
		else if(targetId == 730159)
		{
			switch(env.getDialogId())
			{
				case -1:
					if(var == 5)
					{
						QuestService.addNewSpawn(220050000, 1, 205210, (float) npc.getX(),
										(float) npc.getY(), (float) npc.getZ(), (byte) 0, true);
						npc.getController().onDespawn(true);
						npc.getController().scheduleRespawn();	
						return true;
					}						
			}
		}
		else if(targetId == 205210)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 5)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2717);
				case 10005:
					if(var == 5)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}						
			}
		}
		else if(targetId == 730158)
		{
			switch(env.getDialogId())
			{
				case -1:
					if(var == 5)
					{
						QuestService.addNewSpawn(220050000, 1, 205209, (float) npc.getX(),
										(float) npc.getY(), (float) npc.getZ(), (byte) 0, true);
						npc.getController().onDespawn(true);
						npc.getController().scheduleRespawn();	
						return true;
					}						
			}
		}
		else if(targetId == 205209)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 5)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2717);
				case 10005:
					if(var == 5)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}						
			}
		}
		else if(targetId == 730156)
		{
			switch(env.getDialogId())
			{
				case -1:
					if(var == 5)
					{
						QuestService.addNewSpawn(220050000, 1, 205208, (float) npc.getX(),
										(float) npc.getY(), (float) npc.getZ(), (byte) 0, true);
						npc.getController().onDespawn(true);
						npc.getController().scheduleRespawn();	
						return true;
					}						
			}
		}
		else if(targetId == 205208)
		{
			switch(env.getDialogId())
			{
				case 25:
					if(var == 5)
						return sendQuestDialog(player, env.getVisibleObject().getObjectId(), 2717);
				case 10005:
					if(var == 5)
					{
						qs.setQuestVarById(0, var + 1);
						updateQuestStatus(player, qs);
						PacketSendUtility.sendPacket(player, new SM_DIALOG_WINDOW(env.getVisibleObject().getObjectId(), 10));
						return true;
					}						
			}
		}
		return false;
	}
	
	@Override
	public boolean onEnterZoneEvent(QuestEnv env, ZoneName zoneName)
	{
		if(zoneName != ZoneName.Q2092)
			return false;
		final Player player = env.getPlayer();
		final QuestState qs = player.getQuestStateList().getQuestState(questId);
		if(qs == null || qs.getQuestVars().getQuestVars() != 1)
			return false;
		env.setQuestId(questId);
		qs.setQuestVarById(0, qs.getQuestVarById(0) + 1);
		updateQuestStatus(player, qs);
		return true;
	}


	@Override
	public boolean onKillEvent(QuestEnv env)
	{
		Player player = env.getPlayer();
		QuestState qs = player.getQuestStateList().getQuestState(questId);
		if(qs == null || qs.getStatus() != QuestStatus.START)
			return false;

		int var = qs.getQuestVarById(0);
		int targetId = 0;
		if(env.getVisibleObject() instanceof Npc)
			targetId = ((Npc) env.getVisibleObject()).getNpcId();

		if(targetId == 214402)
		{
			if(var > 5 && var < 20)
			{
				qs.setQuestVarById(0, var + 1);
				updateQuestStatus(player, qs);
				return true;
			}
			else if(var == 20 )
			{
				qs.setStatus(QuestStatus.REWARD);
				updateQuestStatus(player, qs);
				return true;
			}
		}
		return false;
	}

}
